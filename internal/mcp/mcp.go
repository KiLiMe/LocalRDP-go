// Package mcp is a small Model Context Protocol server: JSON-RPC 2.0 over a
// stream (stdin/stdout by default), with the tools and resources capabilities.
// It depends on nothing but the standard library, so the exe stays a single
// file.
package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// ProtocolVersion is the revision this server speaks. Clients ask for their
// own in initialize; we answer with ours, as the spec allows.
const ProtocolVersion = "2025-06-18"

// JSON-RPC error codes.
const (
	CodeParse          = -32700
	CodeInvalidRequest = -32600
	CodeNoMethod       = -32601
	CodeBadParams      = -32602
	CodeInternal       = -32603
)

// RPCError is the error object of a failed request.
type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return e.Message }

// Tool is one callable tool. Schema is the inputSchema object: a JSON Schema
// with type "object". Run returns the text the client shows; an error comes
// back as a tool error, not a protocol error, as MCP asks.
type Tool struct {
	Name        string
	Title       string
	Description string
	Schema      map[string]any
	Run         func(args map[string]any) (string, error)
}

// Resource is one readable resource, addressed by URI.
type Resource struct {
	URI         string
	Name        string
	Description string
	MimeType    string
	Read        func() (string, error)
}

// Server answers requests from in and writes responses to out.
type Server struct {
	Name         string
	Version      string
	Instructions string
	Tools        []Tool
	Resources    []Resource
	In           io.Reader
	Out          io.Writer
	// Logf receives protocol-level notes (bad input and the like). It must
	// never write to Out: stdout carries the protocol alone.
	Logf func(format string, a ...any)
}

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

// Serve reads messages until the stream ends.
func (s *Server) Serve() error {
	if s.In == nil {
		s.In = os.Stdin
	}
	if s.Out == nil {
		s.Out = os.Stdout
	}
	dec := json.NewDecoder(bufio.NewReaderSize(s.In, 1<<20))
	enc := json.NewEncoder(s.Out)
	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			if err == io.EOF {
				return nil
			}
			// One bad message must not end the session.
			s.logf("bad message: %v", err)
			continue
		}
		// Notifications (no id) get no answer.
		if len(req.ID) == 0 {
			continue
		}
		if err := enc.Encode(s.handle(&req)); err != nil {
			return err
		}
	}
}

func (s *Server) logf(format string, a ...any) {
	if s.Logf != nil {
		s.Logf(format, a...)
	}
}

func (s *Server) handle(req *request) *response {
	res := &response{JSONRPC: "2.0", ID: req.ID}
	result, rerr := s.dispatch(req)
	if rerr != nil {
		res.Error = rerr
		return res
	}
	res.Result = result
	return res
}

func (s *Server) dispatch(req *request) (any, *RPCError) {
	switch req.Method {
	case "initialize":
		return s.initialize(req.Params), nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return s.listTools(), nil
	case "tools/call":
		return s.callTool(req.Params)
	case "resources/list":
		return s.listResources(), nil
	case "resources/read":
		return s.readResource(req.Params)
	case "shutdown":
		return map[string]any{}, nil
	case "notifications/initialized", "notifications/cancelled":
		return map[string]any{}, nil
	}
	return nil, &RPCError{Code: CodeNoMethod, Message: "unknown method: " + req.Method}
}

func (s *Server) initialize(params json.RawMessage) any {
	var in struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &in)
	caps := map[string]any{
		"tools": map[string]any{"listChanged": false},
	}
	if len(s.Resources) > 0 {
		caps["resources"] = map[string]any{"subscribe": false, "listChanged": false}
	}
	out := map[string]any{
		"protocolVersion": ProtocolVersion,
		"capabilities":    caps,
		"serverInfo": map[string]any{
			"name":    s.Name,
			"version": s.Version,
		},
	}
	if s.Instructions != "" {
		out["instructions"] = s.Instructions
	}
	return out
}

func (s *Server) listTools() any {
	tools := make([]map[string]any, 0, len(s.Tools))
	for _, t := range s.Tools {
		m := map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"inputSchema": t.Schema,
		}
		if t.Title != "" {
			m["title"] = t.Title
		}
		tools = append(tools, m)
	}
	sort.Slice(tools, func(i, j int) bool {
		return tools[i]["name"].(string) < tools[j]["name"].(string)
	})
	return map[string]any{"tools": tools}
}

func (s *Server) callTool(params json.RawMessage) (any, *RPCError) {
	var in struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &in); err != nil {
		return nil, &RPCError{Code: CodeBadParams, Message: "tools/call: " + err.Error()}
	}
	for _, t := range s.Tools {
		if t.Name != in.Name {
			continue
		}
		text, err := t.Run(in.Arguments)
		if err != nil {
			return map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}, nil
		}
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
		}, nil
	}
	return nil, &RPCError{Code: CodeBadParams, Message: "unknown tool: " + in.Name}
}

func (s *Server) listResources() any {
	out := make([]map[string]any, 0, len(s.Resources))
	for _, r := range s.Resources {
		out = append(out, map[string]any{
			"uri":         r.URI,
			"name":        r.Name,
			"description": r.Description,
			"mimeType":    r.MimeType,
		})
	}
	return map[string]any{"resources": out}
}

func (s *Server) readResource(params json.RawMessage) (any, *RPCError) {
	var in struct {
		URI string `json:"uri"`
	}
	if err := json.Unmarshal(params, &in); err != nil {
		return nil, &RPCError{Code: CodeBadParams, Message: "resources/read: " + err.Error()}
	}
	for _, r := range s.Resources {
		if r.URI != in.URI {
			continue
		}
		text, err := r.Read()
		if err != nil {
			return nil, &RPCError{Code: CodeInternal, Message: err.Error()}
		}
		return map[string]any{
			"contents": []map[string]any{{
				"uri":      r.URI,
				"mimeType": r.MimeType,
				"text":     text,
			}},
		}, nil
	}
	return nil, &RPCError{Code: CodeBadParams, Message: "unknown resource: " + in.URI}
}

// ArgString reads a string argument, with a default.
func ArgString(args map[string]any, key, def string) string {
	if v, ok := args[key].(string); ok && v != "" {
		return v
	}
	return def
}

// ArgInt reads a number argument, with a default and a clamp to [min, max].
func ArgInt(args map[string]any, key string, def, min, max int) int {
	v, ok := args[key].(float64)
	if !ok {
		return def
	}
	n := int(v)
	if n < min {
		n = min
	}
	if n > max {
		n = max
	}
	return n
}

// ArgBool reads a boolean argument.
func ArgBool(args map[string]any, key string) bool {
	v, _ := args[key].(bool)
	return v
}

// JSON renders v as indented JSON, the form the tools answer in.
func JSON(v any) (string, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// StringMap is a small helper for building tool schemas.
func StringMap(kv ...string) map[string]any {
	m := make(map[string]any, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		m[kv[i]] = kv[i+1]
	}
	return m
}

// ObjectSchema builds an object schema from properties, required names and
// whether unknown properties are allowed.
func ObjectSchema(props map[string]any, required []string, allowExtra bool) map[string]any {
	s := map[string]any{
		"type":       "object",
		"properties": props,
	}
	if len(required) > 0 {
		s["required"] = required
	}
	s["additionalProperties"] = allowExtra
	return s
}

// Prop is one property of a tool schema.
func Prop(typ, desc string) map[string]any {
	return map[string]any{"type": typ, "description": desc}
}

// ensure fmt is used even when logging is disabled
var _ = fmt.Sprintf
