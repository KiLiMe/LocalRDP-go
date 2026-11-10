package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type rpcReply struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  json.RawMessage `json:"result"`
	Error   *RPCError       `json:"error"`
}

func serve(t *testing.T, s *Server, in string) []rpcReply {
	t.Helper()
	var out bytes.Buffer
	s.In = strings.NewReader(in)
	s.Out = &out
	if err := s.Serve(); err != nil {
		t.Fatalf("Serve: %v", err)
	}
	var replies []rpcReply
	dec := json.NewDecoder(&out)
	for dec.More() {
		var r rpcReply
		if err := dec.Decode(&r); err != nil {
			t.Fatalf("decode reply: %v", err)
		}
		replies = append(replies, r)
	}
	return replies
}

func testServer() *Server {
	return &Server{
		Name:    "localrdp",
		Version: "1.0.0",
		Tools: []Tool{{
			Name:        "echo",
			Description: "echo the text back",
			Schema: ObjectSchema(map[string]any{
				"text": Prop("string", "text to echo"),
			}, []string{"text"}, false),
			Run: func(args map[string]any) (string, error) {
				return ArgString(args, "text", ""), nil
			},
		}, {
			Name:        "fail",
			Description: "always fails",
			Schema:      ObjectSchema(map[string]any{}, nil, false),
			Run: func(map[string]any) (string, error) {
				return "", errors.New("boom")
			},
		}},
		Resources: []Resource{{
			URI:      "localrdp://status",
			Name:     "status",
			MimeType: "application/json",
			Read:     func() (string, error) { return "{\"ok\":true}", nil },
		}},
	}
}

func TestInitializeAdvertisesToolsAndResources(t *testing.T) {
	replies := serve(t, testServer(),
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`+"\n")
	if len(replies) != 1 {
		t.Fatalf("got %d replies, want 1", len(replies))
	}
	var res struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    map[string]any
		ServerInfo      struct{ Name, Version string }
	}
	if err := json.Unmarshal(replies[0].Result, &res); err != nil {
		t.Fatalf("result: %v", err)
	}
	if res.ProtocolVersion != ProtocolVersion {
		t.Errorf("protocolVersion = %q, want %q", res.ProtocolVersion, ProtocolVersion)
	}
	if _, ok := res.Capabilities["tools"]; !ok {
		t.Error("capabilities.tools missing")
	}
	if _, ok := res.Capabilities["resources"]; !ok {
		t.Error("capabilities.resources missing")
	}
	if res.ServerInfo.Name != "localrdp" {
		t.Errorf("serverInfo.name = %q", res.ServerInfo.Name)
	}
}

func TestToolsListAndCall(t *testing.T) {
	replies := serve(t, testServer(),
		`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`+"\n"+
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"echo","arguments":{"text":"hi"}}}`+"\n")
	if len(replies) != 2 {
		t.Fatalf("got %d replies, want 2", len(replies))
	}
	var listed struct {
		Tools []struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			InputSchema map[string]any `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(replies[0].Result, &listed); err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	if len(listed.Tools) != 2 {
		t.Fatalf("tools = %d, want 2", len(listed.Tools))
	}
	if listed.Tools[0].Name != "echo" || listed.Tools[0].InputSchema["type"] != "object" {
		t.Errorf("unexpected first tool: %+v", listed.Tools[0])
	}
	var called struct {
		Content []struct{ Type, Text string } `json:"content"`
		IsError bool                          `json:"isError"`
	}
	if err := json.Unmarshal(replies[1].Result, &called); err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	if called.IsError || len(called.Content) != 1 || called.Content[0].Text != "hi" {
		t.Errorf("unexpected call result: %+v", called)
	}
}

func TestToolErrorBecomesIsError(t *testing.T) {
	replies := serve(t, testServer(),
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"fail","arguments":{}}}`+"\n")
	var res struct {
		Content []struct{ Text string } `json:"content"`
		IsError bool                    `json:"isError"`
	}
	if err := json.Unmarshal(replies[0].Result, &res); err != nil {
		t.Fatalf("result: %v", err)
	}
	if !res.IsError || len(res.Content) != 1 || res.Content[0].Text != "boom" {
		t.Errorf("unexpected tool error result: %+v", res)
	}
}

func TestUnknownMethodAndUnknownTool(t *testing.T) {
	replies := serve(t, testServer(),
		`{"jsonrpc":"2.0","id":1,"method":"no/such"}`+"\n"+
			`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"nope"}}`+"\n")
	if replies[0].Error == nil || replies[0].Error.Code != CodeNoMethod {
		t.Errorf("unknown method: %+v", replies[0])
	}
	if replies[1].Error == nil || replies[1].Error.Code != CodeBadParams {
		t.Errorf("unknown tool: %+v", replies[1])
	}
}

func TestNotificationsGetNoAnswer(t *testing.T) {
	replies := serve(t, testServer(),
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`+"\n"+
			`{"jsonrpc":"2.0","id":3,"method":"ping"}`+"\n")
	if len(replies) != 1 {
		t.Fatalf("got %d replies, want 1", len(replies))
	}
	if string(replies[0].ID) != "3" {
		t.Errorf("reply id = %s, want 3", replies[0].ID)
	}
}

func TestResourcesRead(t *testing.T) {
	replies := serve(t, testServer(),
		`{"jsonrpc":"2.0","id":1,"method":"resources/list"}`+"\n"+
			`{"jsonrpc":"2.0","id":2,"method":"resources/read","params":{"uri":"localrdp://status"}}`+"\n")
	var listed struct {
		Resources []struct{ URI, MimeType string } `json:"resources"`
	}
	if err := json.Unmarshal(replies[0].Result, &listed); err != nil {
		t.Fatalf("resources/list: %v", err)
	}
	if len(listed.Resources) != 1 || listed.Resources[0].URI != "localrdp://status" {
		t.Fatalf("unexpected resources: %+v", listed.Resources)
	}
	var read struct {
		Contents []struct{ URI, Text string } `json:"contents"`
	}
	if err := json.Unmarshal(replies[1].Result, &read); err != nil {
		t.Fatalf("resources/read: %v", err)
	}
	if len(read.Contents) != 1 || read.Contents[0].Text != `{"ok":true}` {
		t.Errorf("unexpected contents: %+v", read.Contents)
	}
}

func TestArgHelpers(t *testing.T) {
	args := map[string]any{"s": "x", "n": float64(999), "b": true}
	if ArgString(args, "s", "d") != "x" || ArgString(args, "missing", "d") != "d" {
		t.Error("ArgString")
	}
	if ArgInt(args, "n", 1, 0, 50) != 50 || ArgInt(args, "missing", 7, 0, 50) != 7 {
		t.Error("ArgInt")
	}
	if !ArgBool(args, "b") || ArgBool(args, "missing") {
		t.Error("ArgBool")
	}
}
