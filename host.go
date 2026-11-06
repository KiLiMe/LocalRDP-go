package main

import (
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const (
	kTimerWatchdog = 1
	kTimerLaunch   = 2
	kTimerQuit     = 3
	kTimerResize   = 4
	kTimerDisplay  = 5
	kTimerClip     = 6

	wmRdpDisconnected = win.WM_APP + 1
	className         = "LocalRDP.Host"
)

type hostState struct {
	host    *axHost
	hwnd    win.HWND
	axHwnd  win.HWND
	opt     StartOptions
	jobPath string

	everConnected bool
	loggedIn      bool
	closing       bool
	failed        bool
	fullscreen    bool
	discReason    int32
	extReason     int32

	appliedW, appliedH, appliedScale int
	watchdogTicks                    int

	restorePlacement win.WINDOWPLACEMENT
	restoreStyle     uint32
	restoreExStyle   uint32
	hasRestore       bool

	launchPID   uint32
	clipStarted bool
	discDone    bool
}

var gHost *hostState

func (s *hostState) resizeAx() {
	if s.hwnd == 0 || s.host == nil {
		return
	}
	w, h := clientDesktopSize(s.hwnd)
	s.host.resize(w, h)
}

func applyDisplay(s *hostState) bool {
	if s.host == nil || s.host.disp == nil || !s.loggedIn {
		return false
	}
	w, h := clientDesktopSize(s.hwnd)
	ds, dev := resolveScale(s.hwnd, &s.opt)
	if int(w) == s.appliedW && int(h) == s.appliedH && int(ds) == s.appliedScale {
		return true
	}
	dpi := int32(windowDPI(uintptr(s.hwnd)))
	if dpi == 0 {
		dpi = 96
	}
	physW := clampI32(w*254/(10*dpi), 10, 10000)
	physH := clampI32(h*254/(10*dpi), 10, 10000)

	d := disp{s.host.disp}
	callUpdate := func(pw, ph int32) uintptr {
		args := []variant{varI4(w), varI4(h), varI4(pw), varI4(ph), varI4(0), varI4(ds), varI4(dev)}
		return d.callN("UpdateSessionDisplaySettings", args, nil)
	}
	hr := callUpdate(physW, physH)
	if hrFailed(hr) {
		hr = callUpdate(0, 0)
	}
	if hrFailed(hr) {
		logWarn("UpdateSessionDisplaySettings 失败: " + formatHr(uint32(hr)))
		var ret variant
		hr = d.callN("Reconnect", []variant{varI4(w), varI4(h)}, &ret)
		status := int32(-1)
		if ret.VT == vtI4 {
			status = int32(uint32(ret.val))
		}
		variantClear(&ret)
		if hrFailed(hr) || status != 0 {
			logWarn("Reconnect 尺寸回退失败 hr=" + formatHr(uint32(hr)) + " status=" + itoa(int(status)))
			return false
		}
		logf("Display updated via Reconnect %dx%d", w, h)
	} else {
		logf("Display updated %dx%d @%d%% dpi=%d", w, h, ds, dpi)
	}
	s.appliedW, s.appliedH, s.appliedScale = int(w), int(h), int(ds)
	return true
}

func clampI32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (s *hostState) enterFullscreen() {
	if s.hwnd == 0 || s.fullscreen {
		return
	}
	if !s.hasRestore {
		s.restoreStyle = uint32(win.GetWindowLongPtr(s.hwnd, win.GWL_STYLE))
		s.restoreExStyle = uint32(win.GetWindowLongPtr(s.hwnd, win.GWL_EXSTYLE))
		s.restorePlacement.Length = uint32(unsafe.Sizeof(s.restorePlacement))
		win.GetWindowPlacement(s.hwnd, &s.restorePlacement)
		s.hasRestore = true
	}
	mon := win.MonitorFromWindow(s.hwnd, win.MONITOR_DEFAULTTONEAREST)
	var mi win.MONITORINFO
	mi.CbSize = uint32(unsafe.Sizeof(mi))
	win.GetMonitorInfo(mon, &mi)
	s.fullscreen = true
	win.SetWindowLongPtr(s.hwnd, win.GWL_STYLE, uintptr(win.WS_POPUP|win.WS_VISIBLE))
	win.SetWindowLongPtr(s.hwnd, win.GWL_EXSTYLE, uintptr(s.restoreExStyle&^uint32(win.WS_EX_WINDOWEDGE)))
	win.SetWindowPos(s.hwnd, win.HWND_TOP, mi.RcMonitor.Left, mi.RcMonitor.Top,
		mi.RcMonitor.Right-mi.RcMonitor.Left, mi.RcMonitor.Bottom-mi.RcMonitor.Top,
		win.SWP_FRAMECHANGED|win.SWP_NOZORDER)
	s.resizeAx()
	win.PostMessage(s.hwnd, win.WM_SIZE, 0, 0)
	logInfo("Fullscreen entered")
}

func (s *hostState) leaveFullscreen() {
	if s.hwnd == 0 || !s.fullscreen {
		return
	}
	win.SetWindowLongPtr(s.hwnd, win.GWL_STYLE, uintptr(s.restoreStyle))
	win.SetWindowLongPtr(s.hwnd, win.GWL_EXSTYLE, uintptr(s.restoreExStyle))
	win.SetWindowPlacement(s.hwnd, &s.restorePlacement)
	win.SetWindowPos(s.hwnd, win.HWND_TOP, 0, 0, 0, 0, win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER)
	s.fullscreen = false
	s.host.setFullScreen(false)
	s.resizeAx()
	win.PostMessage(s.hwnd, win.WM_SIZE, 0, 0)
	logInfo("Fullscreen left")
}

func (s *hostState) requestClose() {
	if s.closing {
		return
	}
	s.closing = true
	win.PostMessage(s.hwnd, win.WM_CLOSE, 0, 0)
}

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
		printOut("Connected.")
		logInfo("Connected.")
		if s.opt.Fullscreen {
			s.host.setFullScreen(true)
		}
	case 3: // OnLogin
		s.loggedIn = true
		sid := getChildSessionID()
		printOutf("Logon complete. Child session id=%d", sid)
		logf("Logon complete. Child session id=%d", sid)
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
	case 8: // OnEnterFullScreenMode
		logInfo("OnRequestGoFullScreen")
		s.enterFullscreen()
	case 9: // OnLeaveFullScreenMode
		logInfo("OnRequestLeaveFullScreen")
		s.leaveFullscreen()
	case 10: // OnFatalError
		code := arg(0)
		logError("RDP fatal error " + itoa(int(code)))
		printErr("RDP fatal error " + itoa(int(code)))
		s.failed = true
	case 14: // OnRequestContainerMinimize
		if s.hwnd != 0 {
			win.ShowWindow(s.hwnd, win.SW_MINIMIZE)
		}
	}
}

func (s *hostState) startClipBridge() {
	sid := getChildSessionID()
	if sid == kNoSession {
		return
	}
	cmd := buildCommandLine([]string{exePath(), "--clip-bridge",
		"--expect-session", itoa(int(sid)), "--parent-pid", itoa(int(currentProcessID()))})
	var pid uint32
	if !launchInSession(sid, cmd, "", &pid) {
		logWarn("子会话内剪贴板桥启动失败")
		return
	}
	s.clipStarted = true
	logf("Clipboard bridge started in child session pid=%d", pid)
}

func (s *hostState) startSessionEntry() {
	sid := getChildSessionID()
	if sid == kNoSession {
		logWarn("启动程序失败：找不到子会话 id")
		if !s.opt.Desktop {
			s.failed = true
			s.requestClose()
		}
		return
	}
	cmd := buildCommandLine([]string{exePath(), "--session-entry",
		"--job", s.jobPath, "--expect-session", itoa(int(sid))})
	var pid uint32
	if !launchInSession(sid, cmd, "", &pid) {
		logError("Failed to start the session entry process in the child session.")
		if !s.opt.Desktop {
			s.failed = true
			s.requestClose()
		}
		return
	}
	s.launchPID = pid
	printOutf("Started program in child session pid=%d", pid)
	logf("Started program in child session pid=%d", pid)
}

func wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	s := gHost
	if s == nil {
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	}
	switch msg {
	case win.WM_SIZE:
		s.resizeAx()
		win.SetTimer(hwnd, kTimerResize, 400, 0)
		return 0
	case win.WM_DPICHANGED:
		if r := (*win.RECT)(unsafe.Pointer(lParam)); r != nil {
			win.SetWindowPos(hwnd, 0, r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top,
				win.SWP_NOZORDER|win.SWP_NOACTIVATE)
		}
		s.resizeAx()
		win.SetTimer(hwnd, kTimerResize, 400, 0)
		return 0
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_GETMINMAXINFO:
		mmi := (*win.MINMAXINFO)(unsafe.Pointer(lParam))
		mmi.PtMinTrackSize.X = 480
		mmi.PtMinTrackSize.Y = 320
		return 0
	case wmRdpDisconnected:
		s.requestClose()
		return 0
	case win.WM_TIMER:
		switch wParam {
		case kTimerWatchdog:
			s.watchdogTicks++
			if s.everConnected {
				win.KillTimer(hwnd, kTimerWatchdog)
				return 0
			}
			if s.watchdogTicks%5 == 0 && s.host.disp != nil {
				if v, hr := (disp{s.host.disp}).get("Connected"); hr == hrOK {
					logf("仍然等待中 Connected=%s", variantString(&v))
					variantClear(&v)
				}
			}
			if s.watchdogTicks >= s.opt.TimeoutSec {
				printErr("Timed out waiting for the child session.")
				logError("Timed out waiting for the child session.")
				if !s.opt.Desktop {
					s.failed = true
				}
				win.KillTimer(hwnd, kTimerWatchdog)
				win.PostQuitMessage(1)
			}
			return 0
		case kTimerLaunch:
			win.KillTimer(hwnd, kTimerLaunch)
			s.startSessionEntry()
			return 0
		case kTimerClip:
			win.KillTimer(hwnd, kTimerClip)
			s.startClipBridge()
			return 0
		case kTimerDisplay:
			win.KillTimer(hwnd, kTimerDisplay)
			applyDisplay(s)
			return 0
		case kTimerResize:
			win.KillTimer(hwnd, kTimerResize)
			applyDisplay(s)
			return 0
		case kTimerQuit:
			win.KillTimer(hwnd, kTimerQuit)
			// 不调用 DestroyWindow：控件在 WM_DESTROY 里会卡住。直接让消息循环退出。
			win.PostQuitMessage(0)
			return 0
		}
		return 0
	case win.WM_CLOSE:
		logf("WM_CLOSE ever=%v done=%v", s.everConnected, s.discDone)
		if s.everConnected && !s.discDone {
			s.discDone = true
			if s.host.disp != nil {
				hr := (disp{s.host.disp}).call("Disconnect")
				logf("Disconnect() hr=%s", formatHr(uint32(hr)))
			}
			win.SetTimer(hwnd, kTimerQuit, 2000, 0)
			return 0
		}
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_DESTROY:
		// 只发退出信号；COM 对象在消息循环结束后再释放，
		// 避免在控件自己的回调栈里 Release 导致卡死。
		for _, id := range []uintptr{kTimerWatchdog, kTimerLaunch, kTimerQuit, kTimerResize, kTimerDisplay, kTimerClip} {
			win.KillTimer(hwnd, id)
		}
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

var wndProcCb = syscall.NewCallback(wndProc)

// defaultWindowRect 按 C++ 的规则算客户区：工作区 5/6，宽取偶，居中。
func defaultWindowRect(opt *StartOptions) (int32, int32, int32, int32) {
	wa := primaryWorkArea()
	w := int(wa.Right - wa.Left)
	h := int(wa.Bottom - wa.Top)
	cw := 800
	ch := 600
	if x := w * 5 / 6; x > cw {
		cw = x
	}
	if y := h * 5 / 6; y > ch {
		ch = y
	}
	cw &^= 1
	x := int(wa.Left) + (w-cw)/2
	y := int(wa.Top) + (h-ch)/2
	if opt.Width > 0 {
		cw = opt.Width &^ 1
		x = int(wa.Left) + (w-cw)/2
	}
	if opt.Height > 0 {
		ch = opt.Height
		y = int(wa.Top) + (h-ch)/2
	}
	if opt.X != cwUseDefault {
		x = opt.X
	}
	if opt.Y != cwUseDefault {
		y = opt.Y
	}
	return int32(x), int32(y), int32(cw), int32(ch)
}

// RunRdpSession 对应 src/rdp_host.cpp 的 RunRdpSession()。
func RunRdpSession(opt StartOptions, jobPath string) int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	icc := win.INITCOMMONCONTROLSEX{DwSize: uint32(unsafe.Sizeof(win.INITCOMMONCONTROLSEX{})), DwICC: win.ICC_WIN95_CLASSES}
	win.InitCommonControlsEx(&icc)

	hr := win.OleInitialize()
	if hrFailed(uintptr(uint32(hr))) && hr != 1 && uint32(hr) != 0x80010106 {
		printErr("OleInitialize failed: " + formatHr(uint32(hr)))
		return int(ExitError)
	}
	defer win.OleUninitialize()

	s := &hostState{opt: opt, jobPath: jobPath, host: newAxHost()}
	gHost = s

	x, y, cw, ch := defaultWindowRect(&opt)
	style := uint32(win.WS_OVERLAPPEDWINDOW | win.WS_CLIPCHILDREN | win.WS_CLIPSIBLINGS)
	exStyle := uint32(win.WS_EX_APPWINDOW)
	show := int32(win.SW_SHOW)
	if opt.Hidden {
		style = win.WS_POPUP
		exStyle = win.WS_EX_TOOLWINDOW | win.WS_EX_NOACTIVATE
		show = int32(win.SW_HIDE)
		x, y = 0, 0
	} else if opt.Minimized {
		show = int32(win.SW_SHOWMINNOACTIVE)
	}
	if opt.Fullscreen {
		style = win.WS_POPUP | win.WS_VISIBLE
		show = int32(win.SW_SHOW)
	}

	rc := win.RECT{Left: x, Top: y, Right: x + cw, Bottom: y + ch}
	adjustWindowRectEx((*win_Rect)(unsafe.Pointer(&rc)), style, false, exStyle)

	inst := win.GetModuleHandle(nil)
	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		Style:         win.CS_HREDRAW | win.CS_VREDRAW,
		LpfnWndProc:   wndProcCb,
		HInstance:     inst,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(uintptr(win.IDC_ARROW))),
		HbrBackground: win.HBRUSH(win.GetStockObject(win.BLACK_BRUSH)),
		LpszClassName: utf16Ptr(className),
	}
	if win.RegisterClassEx(&wc) == 0 {
		printErr("RegisterClassEx failed: " + formatWin32(uint32(win.GetLastError())))
		return int(ExitError)
	}
	s.hwnd = win.CreateWindowEx(exStyle, utf16Ptr(className), utf16Ptr(opt.Title), style,
		rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, 0, 0, inst, nil)
	if s.hwnd == 0 {
		printErr("CreateWindowEx failed: " + formatWin32(uint32(win.GetLastError())))
		return int(ExitError)
	}
	if opt.Fullscreen {
		mon := win.MonitorFromWindow(s.hwnd, win.MONITOR_DEFAULTTONEAREST)
		var mi win.MONITORINFO
		mi.CbSize = uint32(unsafe.Sizeof(mi))
		win.GetMonitorInfo(mon, &mi)
		win.SetWindowPos(s.hwnd, 0, mi.RcMonitor.Left, mi.RcMonitor.Top,
			mi.RcMonitor.Right-mi.RcMonitor.Left, mi.RcMonitor.Bottom-mi.RcMonitor.Top,
			win.SWP_NOZORDER)
		s.fullscreen = true
	}
	win.ShowWindow(s.hwnd, show)
	win.UpdateWindow(s.hwnd)

	// 抽干一次消息队列，避免尺寸消息早于控件创建到达。
	var msg win.MSG
	for win.PeekMessage(&msg, 0, 0, 0, win.PM_REMOVE) {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}

	var created bool
	for i := range clsidRdpClients {
		if err := s.host.create(&clsidRdpClients[i], s.hwnd); err != nil {
			logWarn("创建 RDP 控件失败: " + err.Error())
			continue
		}
		created = true
		logf("RDP 控件已创建 clsid[%d]", i)
		break
	}
	if !created {
		printErr("Failed to create the Remote Desktop control.")
		return int(ExitError)
	}
	if opt.ViewOnly {
		if w := s.host.controlWindow(); w != 0 {
			win.EnableWindow(w, false)
		}
	}
	s.host.onEvent = s.onEvent

	if !s.host.configure(&opt) {
		return int(ExitError)
	}
	if opt.Clipboard {
		startClipHost()
	}
	s.host.connect()

	if opt.TimeoutSec > 0 {
		win.SetTimer(s.hwnd, kTimerWatchdog, 1000, 0)
	}
	for {
		r := win.GetMessage(&msg, 0, 0, 0)
		if r == 0 || r == -1 {
			break
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	go func() {
		sleepMS(6000)
		logError("清理超时，强制退出")
		os.Exit(int(ExitOk))
	}()
	logf("消息循环已退出，开始清理")
	stopClipHost()
	logf("剪辑板宿主已停止")
	if s.host != nil {
		s.host.release()
	}
	logf("控件已释放")
	gHost = nil

	sid := getChildSessionID()
	if sid != kNoSession && !opt.KeepSess {
		wtLogoffSession(sid, false)
		logf("Logged off child session %d.", sid)
	}
	if s.failed && !s.everConnected {
		printErr(describeDisconnect(s.discReason, s.extReason))
		return int(ExitConnectFailed)
	}
	if s.everConnected {
		printOut(describeDisconnect(s.discReason, s.extReason))
	}
	return int(ExitOk)
}
