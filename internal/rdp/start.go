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
	// From here on the window lifecycle starts: make sure the log exists
	// before anything can return early, so a window that never opened still
	// leaves a line behind.
	logf("cmdStart begin: desktop=%v size=%s scale=%d w=%d h=%d", opt.Desktop, store.NormalizeSizeMode(opt.SizeMode), opt.Scale, opt.Width, opt.Height)
	// 同一个模式已经有窗口：把它叫到前面，不再开第二个（重复点是常事）。
	mode := "start"
	if opt.Desktop {
		mode = "desktop"
	}
	if rec, ok := reusableHost(mode); ok && focusHost(rec) {
		printOutf("A %s window is already open (pid=%d); bringing it to the front.", mode, rec.PID)
		logf("复用已有宿主 pid=%d", rec.PID)
		return int(ExitOk)
	}
	// 没有窗口的宿主既叫不上来也关不掉自己：先收掉再继续，否则它就一直
	// 锁着 localrdp.exe（见 hosts.go）。
	if n := reapWindowlessHosts(); n > 0 {
		printOutf("Closed %d host process(es) that had no window.", n)
	}
	if sid := getChildSessionID(); sid != kNoSession {
		if sessionInUse(sid) {
			printOutf("A child session already exists (id=%d). Run `localrdp stop` first.", sid)
			return int(ExitConflict)
		}
		if sessionAlive(sid) {
			// 刚注销的会话会在 WTS 里留一会儿（Disc）。把它当成“有人用”
			// 就会让紧接着的下一次打开被拒——那正是“关了窗口就打不开”。
			logf("清理残留子会话 id=%d（登记状态不是使用中）", sid)
			printOutf("Cleaning up the leftover child session id=%d before starting.", sid)
			wtLogoffSession(sid, false)
		} else {
			// 子会话注销后 WTSGetChildSessionId 会留下旧 id，别被它卡住。
			logWarn("忽略残留的子会话 id=" + win32.Itoa(int(sid)) + "（WTS 枚举里已不存在）")
			printOutf("Ignoring stale child session id=%d: no such session in the WTS list.", sid)
		}
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
	logf("cmdStart end: code=%d", code)
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
	code := int(ExitOk)
	// 先注销子会话：宿主收到断开事件会自己干净退出（也才有机会补 Disconnect）。
	sid := getChildSessionID()
	if sid != kNoSession && sessionAlive(sid) {
		if wtLogoffSession(sid, true) {
			printOutf("Logged off child session %d.", sid)
		} else {
			printErrf("Failed to log off child session %d.", sid)
			code = int(ExitError)
		}
		// 给宿主一点时间走完断开→退出的路。
		sleepMS(1500)
	} else {
		printOut("No child session is active.")
	}
	// 再收宿主窗口：没跟着退出的是摊位（没有窗口、不听 WM_CLOSE、锁着 exe）。
	if closed, failed := stopAllHosts(); closed > 0 || failed > 0 {
		if failed > 0 {
			printErrf("%d host process(es) could not be closed.", failed)
			code = int(ExitError)
		} else {
			printOutf("Closed %d host window(s).", closed)
		}
	}
	return code
}

func splitCommand(rest []string) string {
	if len(rest) == 0 {
		return ""
	}
	return buildCommandLine(rest)
}
