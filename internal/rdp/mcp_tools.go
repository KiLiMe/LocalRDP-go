package rdp

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"localrdp/internal/mcp"
	"localrdp/internal/store"
	"localrdp/internal/win32"
)

// Windows pieces the tools need beyond what the host already binds.
var (
	procWTSQuerySessionInformationW = wtsapi32.NewProc("WTSQuerySessionInformationW")
	// CreateToolhelp32Snapshot / Process32FirstW / Process32NextW and
	// processEntry32W already live in session_launch.go.
)

const (
	wtsUserName         = 5
	th32csSnapProcess   = 0x00000002
	createNewProcessGrp = 0x00000200
	logReadLimit        = 16 << 20
	logDefaultLines     = 120
	logMaxLines         = 5000
)

var wtsStateNames = map[int32]string{
	0: "active", 1: "connected", 2: "connect-query", 3: "shadow", 4: "disconnected",
	5: "idle", 6: "listen", 7: "reset", 8: "down", 9: "init",
}

// mcpStatusJSON is the machine-readable form of `localrdp status`.
func mcpStatusJSON() (map[string]any, error) {
	r := queryEnvironment()
	var child any
	if r.ChildSessionID != kNoSession {
		child = int(r.ChildSessionID)
	}
	return map[string]any{
		"version":                versionString,
		"edition":                r.EditionID,
		"build":                  r.BuildNumber,
		"home_edition":           r.HomeEdition,
		"elevated":               r.Elevated,
		"child_sessions_enabled": r.ChildSessionsOn,
		"term_service_running":   r.TermServiceRunning,
		"rdp_listener_enabled":   r.RdpListenerEnabled,
		"rdp_control_registered": r.RdpControlOK,
		"inside_child_session":   r.InsideChildSession,
		"current_session_id":     int(r.CurrentSessionID),
		"child_session_id":       child,
		"child_session_alive":    r.ChildSessionID != kNoSession && sessionAlive(r.ChildSessionID),
		"transport":              sprintf("0x%08X", r.TransportStatus),
		"transport_state":        describeTransport(r.TransportStatus),
		"transport_pipe":         r.TransportPipe,
		"next_action":            r.NextAction,
		"ready":                  !r.blocked(),
	}, nil
}

// logLines reads the log and returns the last n matching lines.
func logLines(n int, grep, level string, since time.Duration) ([]string, error) {
	path := store.LogFile()
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	var data []byte
	if st.Size() > logReadLimit {
		if _, err := f.Seek(st.Size()-logReadLimit, 0); err != nil {
			return nil, err
		}
		data = make([]byte, logReadLimit)
		got, _ := f.Read(data)
		data = data[:got]
	} else {
		data = make([]byte, st.Size())
		got, _ := f.Read(data)
		data = data[:got]
	}
	cut := time.Now().Add(-since)
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	out := make([]string, 0, n)
	for _, line := range lines {
		if line == "" {
			continue
		}
		if level != "" && !strings.Contains(line, "["+strings.ToUpper(level)+"]") {
			continue
		}
		if grep != "" && !containsFold(line, grep) {
			continue
		}
		if since > 0 {
			if ts, err := time.ParseInLocation("2006-01-02 15:04:05.000", linePrefix(line, 23), time.Local); err == nil {
				if ts.Before(cut) {
					continue
				}
			}
		}
		out = append(out, line)
		if len(out) > n {
			out = out[1:]
		}
	}
	return out, nil
}

func linePrefix(s string, n int) string {
	if len(s) < n {
		return s
	}
	return s[:n]
}

func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// mcpSessions lists the Windows sessions (qwinsta without the console).
func mcpSessions() ([]map[string]any, error) {
	items := wtsSessions()
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		it := items[i]
		name := ""
		if it.WinStationName != nil {
			name = syscall.UTF16ToString(unsafe.Slice(it.WinStationName, 64))
		}
		out = append(out, map[string]any{
			"id":         int(it.SessionID),
			"name":       name,
			"state":      wtsStateNames[it.State],
			"user":       sessionUser(it.SessionID),
			"is_current": it.SessionID == currentSessionID(),
		})
	}
	return out, nil
}

func sessionUser(id uint32) string {
	var buf *uint16
	var n uint32
	r, _, _ := procWTSQuerySessionInformationW.Call(0, uintptr(id), wtsUserName,
		uintptr(unsafe.Pointer(&buf)), uintptr(unsafe.Pointer(&n)))
	if r == 0 || buf == nil {
		return ""
	}
	defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(buf)))
	return syscall.UTF16ToString(unsafe.Slice(buf, int(n/2)))
}

// mcpProcesses lists processes whose exe name contains filter.
func mcpProcesses(filter string) []map[string]any {
	out := []map[string]any{}
	snap, _, _ := procCreateToolhelp32Snapshot.Call(th32csSnapProcess, 0)
	if snap == ^uintptr(0) || snap == 0 {
		return out
	}
	defer syscall.CloseHandle(syscall.Handle(snap))
	var e processEntry32W
	e.Size = uint32(unsafe.Sizeof(e))
	ok, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e)))
	for ok != 0 {
		name := syscall.UTF16ToString(e.ExeFile[:])
		if filter == "" || containsFold(name, filter) {
			var sid uint32
			procProcessIdToSessionId.Call(uintptr(e.ProcessID), uintptr(unsafe.Pointer(&sid)))
			out = append(out, map[string]any{
				"pid":        int(e.ProcessID),
				"ppid":       int(e.ParentProcessID),
				"name":       name,
				"session_id": int(sid),
				"threads":    int(e.CntThreads),
			})
		}
		e.Size = uint32(unsafe.Sizeof(e))
		ok, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e)))
	}
	return out
}

// launchChild starts this exe detached, with stdout/stderr dropped so the MCP
// stream stays clean.
func launchChild(args []string, extraEnv []string) (int, error) {
	exe := win32.ExePath()
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), extraEnv...)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow | createNewProcessGrp}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	logf("mcp: launched %s %s (pid=%d)", filepath.Base(exe), strings.Join(args, " "), pid)
	return pid, nil
}

// mcpTools is the tool set the MCP server exposes.
func mcpTools() []mcp.Tool {
	return []mcp.Tool{
		{
			Name:        "lrdp_status",
			Title:       "LocalRDP environment status",
			Description: "Windows edition, child-session support, RDP control registration, transport state, current and child session ids, and the recommended next action. Same data as `localrdp status`, as JSON.",
			Schema:      mcp.ObjectSchema(map[string]any{}, nil, false),
			Run: func(map[string]any) (string, error) {
				v, err := mcpStatusJSON()
				if err != nil {
					return "", err
				}
				return mcp.JSON(v)
			},
		},
		{
			Name:        "lrdp_log",
			Title:       "Read the LocalRDP log",
			Description: "Tail of %LOCALAPPDATA%\\LocalRDP\\localrdp.log. Filter by level (INFO/WARN/ERROR), by substring, and by age. Most host problems show up here first.",
			Schema: mcp.ObjectSchema(map[string]any{
				"lines":         mcp.Prop("integer", "how many matching lines to return (default 120, max 5000)"),
				"level":         mcp.Prop("string", "only lines of this level: INFO, WARN or ERROR"),
				"grep":          mcp.Prop("string", "only lines containing this text (case-insensitive)"),
				"since_minutes": mcp.Prop("integer", "only lines newer than this many minutes"),
			}, nil, false),
			Run: func(args map[string]any) (string, error) {
				n := mcp.ArgInt(args, "lines", logDefaultLines, 1, logMaxLines)
				level := strings.TrimSpace(mcp.ArgString(args, "level", ""))
				grep := mcp.ArgString(args, "grep", "")
				since := time.Duration(mcp.ArgInt(args, "since_minutes", 0, 0, 60*24*30)) * time.Minute
				lines, err := logLines(n, grep, level, since)
				if err != nil {
					return "", err
				}
				return mcp.JSON(map[string]any{
					"path":  store.LogFile(),
					"count": len(lines),
					"lines": lines,
				})
			},
		},
		{
			Name:        "lrdp_sessions",
			Title:       "List Windows sessions",
			Description: "All logon sessions with id, station name, user and state. Use it to see whether a child session exists and whether it is active or disconnected.",
			Schema:      mcp.ObjectSchema(map[string]any{}, nil, false),
			Run: func(map[string]any) (string, error) {
				s, err := mcpSessions()
				if err != nil {
					return "", err
				}
				return mcp.JSON(map[string]any{"sessions": s})
			},
		},
		{
			Name:        "lrdp_processes",
			Title:       "List LocalRDP processes",
			Description: "Processes whose exe name matches the filter (default localrdp), with pid, parent pid and session id.",
			Schema: mcp.ObjectSchema(map[string]any{
				"filter": mcp.Prop("string", "exe name filter (default localrdp)"),
			}, nil, false),
			Run: func(args map[string]any) (string, error) {
				filter := mcp.ArgString(args, "filter", "localrdp")
				list := mcpProcesses(filter)
				sort.Slice(list, func(i, j int) bool {
					return list[i]["pid"].(int) < list[j]["pid"].(int)
				})
				return mcp.JSON(map[string]any{"filter": filter, "processes": list})
			},
		},
		{
			Name:        "lrdp_diag",
			Title:       "One-shot diagnosis",
			Description: "Environment status + sessions + localrdp processes + the last warnings and errors from the log, in one answer. Start here when a session does not behave.",
			Schema: mcp.ObjectSchema(map[string]any{
				"log_lines": mcp.Prop("integer", "how many recent warn/error lines to include (default 40)"),
			}, nil, false),
			Run: func(args map[string]any) (string, error) {
				n := mcp.ArgInt(args, "log_lines", 40, 1, 500)
				status, _ := mcpStatusJSON()
				sessions, serr := mcpSessions()
				warn, _ := logLines(n, "", "WARN", 0)
				errs, _ := logLines(n, "", "ERROR", 0)
				all, _ := logLines(n, "", "", 0)
				out := map[string]any{
					"status":     status,
					"processes":  mcpProcesses("localrdp"),
					"log_path":   store.LogFile(),
					"last_lines": all,
					"warnings":   warn,
					"errors":     errs,
				}
				if serr == nil {
					out["sessions"] = sessions
				} else {
					out["sessions_error"] = serr.Error()
				}
				return mcp.JSON(out)
			},
		},
		{
			Name:        "lrdp_desktop",
			Title:       "Open the child desktop",
			Description: "Start `localrdp desktop` detached: a host window showing the child session of this account. Returns the new pid. Use lrdp_log and lrdp_diag afterwards to watch the connection.",
			Schema: mcp.ObjectSchema(map[string]any{
				"user":         mcp.Prop("string", "account name for the child session (default: current user)"),
				"password":     mcp.Prop("string", "password; passed to the child through an environment variable, never argv"),
				"password_env": mcp.Prop("string", "name of an environment variable holding the password"),
				"width":        mcp.Prop("integer", "window client width"),
				"height":       mcp.Prop("integer", "window client height"),
				"fullscreen":   mcp.Prop("boolean", "open full screen"),
				"title":        mcp.Prop("string", "host window title"),
				"timeout":      mcp.Prop("integer", "connect timeout in seconds"),
			}, nil, false),
			Run: func(args map[string]any) (string, error) {
				return mcpLaunch(args, false)
			},
		},
		{
			Name:        "lrdp_start",
			Title:       "Run a program in the child session",
			Description: "Start `localrdp start -- <program>` detached. Needs the two privileges granted by install-privileges.ps1 and a logon re-login.",
			Schema: mcp.ObjectSchema(map[string]any{
				"program":      mcp.Prop("string", "program to run in the child session"),
				"args":         mcp.Prop("string", "arguments for the program, one command line"),
				"user":         mcp.Prop("string", "account name (default: current user)"),
				"password":     mcp.Prop("string", "password; passed through an environment variable"),
				"password_env": mcp.Prop("string", "name of an environment variable holding the password"),
				"workdir":      mcp.Prop("string", "working directory for the program"),
			}, []string{"program"}, false),
			Run: func(args map[string]any) (string, error) {
				return mcpLaunch(args, true)
			},
		},
		{
			Name:        "lrdp_click",
			Title:       "Press a button",
			Description: "Press a button of the window or of a session window without a mouse, as `localrdp click` does: fullscreen / reconnect / disconnect for the session window's bar, and the window's own buttons for the rest.",
			Schema: mcp.ObjectSchema(map[string]any{
				"button": mcp.Prop("string", "button name: fullscreen, reconnect, disconnect, open, status, stop, privilege, forget-password, data-folder, show, close-session, refresh, clear-log, copy-log"),
				"pid":    mcp.Prop("integer", "session window process id (default: the one that is open)"),
			}, []string{"button"}, false),
			Run: func(args map[string]any) (string, error) {
				return mcpClick(args)
			},
		},
		{
			Name:        "lrdp_stop",
			Title:       "Log off the child session",
			Description: "Log off the child session, as `localrdp stop` does.",
			Schema:      mcp.ObjectSchema(map[string]any{}, nil, false),
			Run: func(map[string]any) (string, error) {
				sid := getChildSessionID()
				if sid == kNoSession {
					return mcp.JSON(map[string]any{"stopped": false, "reason": "no child session is active"})
				}
				ok := wtLogoffSession(sid, true)
				return mcp.JSON(map[string]any{"stopped": ok, "session_id": int(sid)})
			},
		},
	}
}

// mcpClick presses a button, the way `localrdp click` does.
func mcpClick(args map[string]any) (string, error) {
	name := strings.ToLower(strings.TrimSpace(mcp.ArgString(args, "button", "")))
	if name == "" {
		return "", errString("button is required")
	}
	pid := int(mcp.ArgInt(args, "pid", 0, 0, 100000))
	if cmd, ok := hostClicks[name]; ok {
		if pid == 0 {
			pid = win32.HostProcess()
		}
		if pid == 0 {
			return mcp.JSON(map[string]any{"clicked": false, "reason": "no session window is open"})
		}
		ok := win32.PostCommand(pid, cmd)
		return mcp.JSON(map[string]any{"clicked": ok, "button": name, "pid": pid})
	}
	if !win32.GuiRunning() {
		return mcp.JSON(map[string]any{"clicked": false, "button": name, "reason": "the window is not open"})
	}
	if err := store.WriteGuiCommand(name); err != nil {
		return mcp.JSON(map[string]any{"clicked": false, "button": name, "reason": err.Error()})
	}
	return mcp.JSON(map[string]any{"clicked": true, "button": name})
}

// mcpLaunch builds the command line for desktop/start and starts it detached.
// A program rides on the desktop: the session outlives it.
func mcpLaunch(args map[string]any, start bool) (string, error) {
	cmd := []string{"desktop"}
	if start {
		if mcp.ArgString(args, "program", "") == "" {
			return "", errString("program is required")
		}
	}
	if user := mcp.ArgString(args, "user", ""); user != "" {
		cmd = append(cmd, "--user", user)
	}
	env := []string{}
	pw := mcp.ArgString(args, "password", "")
	pwEnv := mcp.ArgString(args, "password_env", "")
	switch {
	case pwEnv != "":
		cmd = append(cmd, "--password-env", pwEnv)
	case pw != "":
		env = append(env, "LRDP_PW="+pw)
		cmd = append(cmd, "--password-env", "LRDP_PW")
	}
	if v := mcp.ArgInt(args, "width", 0, 0, 8192); v > 0 {
		cmd = append(cmd, "--width", win32.Itoa(v))
	}
	if v := mcp.ArgInt(args, "height", 0, 0, 8192); v > 0 {
		cmd = append(cmd, "--height", win32.Itoa(v))
	}
	if v := mcp.ArgInt(args, "timeout", 0, 5, 3600); v > 0 {
		cmd = append(cmd, "--timeout", win32.Itoa(v))
	}
	if t := mcp.ArgString(args, "title", ""); t != "" {
		cmd = append(cmd, "--title", t)
	}
	if start {
		if wd := mcp.ArgString(args, "workdir", ""); wd != "" {
			cmd = append(cmd, "--workdir", wd)
		}
		cmd = append(cmd, "--", mcp.ArgString(args, "program", ""))
		if rest := mcp.ArgString(args, "args", ""); rest != "" {
			cmd = append(cmd, strings.Fields(rest)...)
		}
	} else if mcp.ArgBool(args, "fullscreen") {
		cmd = append(cmd, "--fullscreen")
	}
	pid, err := launchChild(cmd, env)
	if err != nil {
		return "", err
	}
	return mcp.JSON(map[string]any{
		"pid":     pid,
		"command": cmd,
		"note":    "detached; read lrdp_log without since_minutes to follow the connection",
	})
}

// errString is a tiny error type so the tools stay allocation-light.
type errString string

func (e errString) Error() string { return string(e) }
