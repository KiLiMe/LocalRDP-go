package rdp

import (
	"strings"

	"localrdp/internal/mcp"
)

// cmdMcp runs the Model Context Protocol server on stdin/stdout. Everything it
// needs to answer a question — status, log, sessions, processes, launching a
// session — is reachable through tools, so an AI client can diagnose a broken
// child session without a human copying log lines around.
//
// stdout carries protocol frames only: no printOut here, and any child process
// is started with its output dropped.
func cmdMcp(args []string) int {
	for _, a := range args {
		switch strings.ToLower(a) {
		case "-h", "--help", "/?":
			printOut("Usage: localrdp mcp")
			printOut("")
			printOut("Model Context Protocol server on stdin/stdout (JSON-RPC 2.0).")
			printOut("Tools: lrdp_status, lrdp_log, lrdp_sessions, lrdp_processes,")
			printOut("       lrdp_diag, lrdp_desktop, lrdp_start, lrdp_click, lrdp_stop.")
			printOut("Resources: localrdp://status, localrdp://log")
			return int(ExitOk)
		}
	}

	srv := &mcp.Server{
		Name:    "localrdp",
		Version: versionString,
		Instructions: "LocalRDP opens a Windows child session as a second desktop. " +
			"Start with lrdp_diag when something looks wrong: it bundles the environment " +
			"status, the session list, the running localrdp processes and the last log lines. " +
			"lrdp_desktop and lrdp_start launch sessions detached; lrdp_stop logs the child " +
			"session off. All answers come back as JSON text.",
		Tools:     mcpTools(),
		Resources: mcpResources(),
		Logf: func(format string, a ...any) {
			logf("mcp: "+format, a...)
		},
	}
	logf("mcp: server up (stdio, protocol %s)", mcp.ProtocolVersion)
	if err := srv.Serve(); err != nil {
		logError("mcp: " + err.Error())
		return int(ExitError)
	}
	logf("mcp: server down")
	return int(ExitOk)
}

// mcpResources exposes the two things a client wants to read continuously: the
// environment status and the tail of the log file.
func mcpResources() []mcp.Resource {
	return []mcp.Resource{
		{
			URI:         "localrdp://status",
			Name:        "status",
			Description: "Environment report as JSON, same data as the lrdp_status tool.",
			MimeType:    "application/json",
			Read: func() (string, error) {
				v, err := mcpStatusJSON()
				if err != nil {
					return "", err
				}
				return mcp.JSON(v)
			},
		},
		{
			URI:         "localrdp://log",
			Name:        "log",
			Description: "Last 300 lines of localrdp.log.",
			MimeType:    "text/plain",
			Read: func() (string, error) {
				lines, err := logLines(300, "", "", 0)
				if err != nil {
					return "", err
				}
				return strings.Join(lines, "\n"), nil
			},
		},
	}
}
