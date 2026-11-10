package rdp

import (
	"os"

	"github.com/lxn/win"
	"localrdp/internal/store"
	"localrdp/internal/win32"
)

// cmdStart 对应 src/start.cpp 的 CmdStart()。
func cmdStart(opt StartOptions) int {
	opt = withDefaultCredentials(opt)

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
		if sessionAlive(sid) {
			printOutf("A child session already exists (id=%d). Run `localrdp stop` first.", sid)
			return int(ExitConflict)
		}
		// 子会话注销后 WTSGetChildSessionId 会留下旧 id，别被它卡住。
		logWarn("忽略残留的子会话 id=" + win32.Itoa(int(sid)) + "（WTS 枚举里已不存在）")
		printOutf("Ignoring stale child session id=%d: no such session in the WTS list.", sid)
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

// withDefaultCredentials 补上“当前账户”这层默认：desktop / start 都是给本机
// 这个用户开子会话，不指定账户时就该用它自己，密码取 DPAPI 缓存。
// 没有这一步，子会话会停在登录页等着从没人输入的密码。
func withDefaultCredentials(opt StartOptions) StartOptions {
	if opt.User == "" {
		opt.User = currentUserQualified()
	}
	if opt.Password == "" && opt.User != "" && store.HasSecret() {
		opt.Password = store.LoadSecret()
	}
	return opt
}

// currentUserQualified 返回 DOMAIN\user 形式的当前账户。
func currentUserQualified() string {
	user := os.Getenv("USERNAME")
	if user == "" {
		return ""
	}
	if domain := os.Getenv("USERDOMAIN"); domain != "" {
		return domain + `\` + user
	}
	return user
}

// cmdStop 对应 src/start.cpp 的 CmdStop()。
func cmdStop() int {
	sid := getChildSessionID()
	if sid == kNoSession {
		printOut("No child session is active.")
		return int(ExitOk)
	}
	if !sessionAlive(sid) {
		printOutf("Child session %d is already gone.", sid)
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
