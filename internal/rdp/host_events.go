package rdp

import (
	"github.com/lxn/win"

	"localrdp/internal/fault"
	"localrdp/internal/win32"
)

// onEvent 处理 RDP 控件事件（对应 src/rdp_events.cpp + 各 On* 回调）。
func (s *hostState) onEvent(id int32, args []variant) {
	logf("事件 %s", describeEvent(id, args))
	arg := func(i int) int32 {
		if i < len(args) {
			v := args[i]
			if v.VT == vtI4 {
				return int32(uint32(v.val))
			}
			if v.VT == vtI2 {
				return int32(int16(uint32(v.val)))
			}
		}
		return 0
	}
	switch id {
	case 2: // OnConnected
		s.everConnected = true
		s.tlsRetries = 0
		s.refreshSelf()
		printOut("Connected.")
		logInfo("Connected.")
		s.invalidateBar()
		if s.baseW == 0 {
			s.baseW, s.baseH = s.viewSize()
			// 连接时的缩放就是基准：scale 档以后按窗口/基准的比例往上走。
			s.baseScale = s.layoutScale()
			logf("会话尺寸基准 %dx%d（size=%s）", s.baseW, s.baseH, s.sizeMode)
		}
		if s.opt.Fullscreen {
			s.host.setFullScreen(true)
		}
	case 3: // OnLogin
		s.loggedIn = true
		s.invalidateBar()
		sid := getChildSessionID()
		// 标题挂上会话号：任务栏上两个按钮一个叫 LocalRDP，一个叫
		// “LocalRDP · 子会话 48”，不给号就分不出哪个是哪个。
		setWindowText(uintptr(s.hwnd), sessionTitle(s.opt.Title, sid))
		printOutf("Logon complete. Child session id=%d", sid)
		logf("Logon complete. Child session id=%d", sid)
		// 会话号到这一步才知道，登记表（hosts.go）跟着写一次：`localrdp
		// hosts`/`status` 靠它区分活会话与残留。
		s.refreshSelf()
		if s.opt.Clipboard {
			win.SetTimer(s.hwnd, kTimerClip, 800, 0)
		}
		s.appliedW, s.appliedH, s.appliedScale = 0, 0, 0
		win.SetTimer(s.hwnd, kTimerDisplay, 800, 0)
		if s.jobPath != "" {
			win.SetTimer(s.hwnd, kTimerLaunch, 1500, 0)
		}
	case 4: // OnDisconnected
		s.discReason = arg(0)
		if s.host.disp != nil {
			if v, hr := (disp{s.host.disp}).get("ExtendedDisconnectReason"); hr == hrOK {
				if v.VT == vtI4 {
					s.extReason = int32(uint32(v.val))
				}
				variantClear(&v)
			}
		}
		logInfo("OnDisconnected " + describeDisconnect(s.discReason, s.extReason))
		s.discSeen = true
		s.loggedIn = false
		s.invalidateBar()
		// 断开之后那个号就不属于这个窗口了，留着只会让人去 stop 一个不存在的会话。
		s.forgetSessionInBook()
		if s.retrying {
			// 重连时断开是它自己要求的第一步。
			return
		}
		if code, detail, ok := faultForDisconnect(s.discReason, s.extReason, s.everConnected); ok {
			// 握手被本机掐掉是半开状态，会自己消退：先重试，别急着报 E202。
			if code == fault.TLSHandshake && s.retryTLSHandshake() {
				return
			}
			s.fail(code, detail)
		}
		// 断开之后控件只剩一块白板：把窗口收掉，别留一个既没画面也不
		// 退出的空窗口干等超时——没连上就断开的那些失败也走这条路，
		// 于是白屏在断开的那一刻就结束，而不是 90 秒之后。
		win.PostMessage(s.hwnd, wmRdpDisconnected, 0, 0)
	case 8: // OnEnterFullScreenMode
		logInfo("OnRequestGoFullScreen")
		if s.toggleAllowed() {
			s.enterFullscreen()
		}
	case 9: // OnLeaveFullScreenMode
		logInfo("OnRequestLeaveFullScreen")
		if s.toggleAllowed() {
			s.leaveFullscreen()
		}
	case 10: // OnFatalError
		code := arg(0)
		logError("RDP fatal error " + win32.Itoa(int(code)))
		s.fail(fault.ControlBroken, "错误码 "+win32.Itoa(int(code)))
	case 14: // OnRequestContainerMinimize
		if s.hwnd != 0 {
			win.ShowWindow(s.hwnd, win.SW_MINIMIZE)
		}
	}
}

// Which launch wmRdpLaunched is reporting on.
const (
	launchClipBridge   = 1
	launchSessionEntry = 2
)

// launchAsync runs a launch off the UI thread. launchInSession retries for
// seconds while the child session is not ready, and the window has to keep
// following the mouse meanwhile; the outcome comes back as wmRdpLaunched.
func (s *hostState) launchAsync(kind uintptr, sid uint32, cmd string) {
	hwnd := s.hwnd
	go func() {
		var pid uint32
		ok := launchInSession(sid, cmd, "", true, &pid)
		var lp uintptr
		if ok {
			lp = uintptr(pid)
		}
		win.PostMessage(hwnd, wmRdpLaunched, kind, lp)
	}()
}

// onLaunchResult runs on the UI thread, so everything it touches is ours.
func (s *hostState) onLaunchResult(kind, pid uintptr) uintptr {
	switch kind {
	case launchClipBridge:
		if pid == 0 {
			s.warn(fault.ClipBridgeFailed, "")
			return 0
		}
		s.clipStarted = true
		logf("Clipboard bridge started in child session pid=%d", pid)
	case launchSessionEntry:
		if pid == 0 {
			logError("Failed to start the session entry process in the child session.")
			if !s.opt.Desktop {
				s.fail(fault.LaunchFailed, "")
				s.requestClose()
			}
			return 0
		}
		printOutf("Started program in child session pid=%d", pid)
		logf("Started program in child session pid=%d", pid)
	}
	return 0
}

func (s *hostState) startClipBridge() {
	sid := getChildSessionID()
	if sid == kNoSession {
		return
	}
	cmd := buildCommandLine([]string{win32.ExePath(), "--clip-bridge",
		"--expect-session", win32.Itoa(int(sid)), "--parent-pid", win32.Itoa(int(currentProcessID()))})
	s.launchAsync(launchClipBridge, sid, cmd)
}

func (s *hostState) startSessionEntry() {
	sid := getChildSessionID()
	if sid == kNoSession {
		logWarn("启动程序失败：找不到子会话 id")
		if !s.opt.Desktop {
			s.fail(fault.LaunchFailed, "找不到子会话 id")
			s.requestClose()
		}
		return
	}
	cmd := buildCommandLine([]string{win32.ExePath(), "--session-entry",
		"--job", s.jobPath, "--expect-session", win32.Itoa(int(sid))})
	s.launchAsync(launchSessionEntry, sid, cmd)
}
