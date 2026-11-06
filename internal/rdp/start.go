package rdp

import (
	"github.com/lxn/win"
)

// cmdStart 对应 src/start.cpp 的 CmdStart()。
func cmdStart(opt StartOptions) int {
	if insideChildSession() {
		printErr("Refusing to start a nested child session: this process already runs inside one.")
		return int(ExitError)
	}
	if code := ensureChildSessionsForStart(); code != 0 {
		return code
	}
	env := queryEnvironment()
	if env.blocked() {
		printStatus(env)
		return int(ExitNotReady)
	}
	if win.GetSystemMetrics(win.SM_REMOTESESSION) != 0 {
		printOut("Note: running inside a remote session; the child desktop opens on this session's display.")
	}
	if sid := getChildSessionID(); sid != kNoSession {
		printOutf("A child session already exists (id=%d). Run `localrdp stop` first.", sid)
		return int(ExitConflict)
	}
	if opt.Command == "" && !opt.Desktop {
		printOut("No program specified; opening the child desktop only.")
	}

	job := ""
	if opt.Command != "" {
		job = writeJobFile(opt)
		if job == "" {
			printErr("Failed to write the job file.")
			return int(ExitError)
		}
		logf("Job file: %s", job)
	}
	code := RunRdpSession(opt, job)
	if job != "" {
		deleteJobFile(job)
	}
	return code
}

// cmdStop 对应 src/start.cpp 的 CmdStop()。
func cmdStop() int {
	sid := getChildSessionID()
	if sid == kNoSession {
		printOut("No child session is active.")
		return int(ExitOk)
	}
	if wtLogoffSession(sid, true) {
		printOutf("Logged off child session %d.", sid)
		return int(ExitOk)
	}
	printErrf("Failed to log off child session %d.", sid)
	return int(ExitError)
}

func splitCommand(rest []string) string {
	if len(rest) == 0 {
		return ""
	}
	return buildCommandLine(rest)
}
