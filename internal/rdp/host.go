package rdp

import (
	"os"
	"runtime"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
	"localrdp/internal/fault"
	"localrdp/internal/i18n"
	"localrdp/internal/store"
	"localrdp/internal/win32"
)

const (
	kTimerWatchdog  = 1
	kTimerLaunch    = 2
	kTimerQuit      = 3
	kTimerResize    = 4
	kTimerDisplay   = 5
	kTimerClip      = 6
	kTimerReconnect = 7
	// kTimerOverlay watches the pointer in full screen: the floating bar
	// only shows while it is at the top edge.
	kTimerOverlay = 8

	wmRdpDisconnected = win.WM_APP + 1
	// wmRdpLaunched reports a launch that had to happen off the UI thread:
	// wParam says which one, lParam is the pid (0 when it failed).
	wmRdpLaunched = win.WM_APP + 3
	className     = "LocalRDP.Host"

	// hotkeyFullScreen is the id of the Ctrl+Alt+F hot key this window claims.
	hotkeyFullScreen = 1

	// kMinDesktopW, kMinDesktopH are the smallest desktop the session takes.
	// The window cannot be dragged smaller than this, and anything below it
	// (a minimized window reports 0x0) is not a size to negotiate.
	kMinDesktopW = 480
	kMinDesktopH = 320
)

type hostState struct {
	host   *axHost
	hwnd   win.HWND
	axHwnd win.HWND
	// view is the child window the control lives in: the bar above it belongs
	// to this process, so the control gets the rest of the window.
	view win.HWND
	// overlay is the bar's buttons floated over the picture in full screen.
	overlay win.HWND
	opt     StartOptions
	jobPath string

	everConnected bool
	loggedIn      bool
	closing       bool
	failed        bool
	fullscreen    bool
	discReason    int32
	extReason     int32
	// code and detail are the fault the window ends with, when it is one the
	// program knows; faults.go says which failures get one.
	code   fault.Code
	detail string

	overlayHover                     uintptr
	overlayTracked                   bool
	overlayShown                     bool
	appliedW, appliedH, appliedScale int
	// retrying is set while a reconnect is between Disconnect and Connect.
	retrying   bool
	lastToggle time.Time
	// sizeMode is store.SizeFit, SizeFixed or SizeScale: what the window
	// does with its size while the session runs.
	sizeMode string
	// baseW, baseH are the client size the session connected with; fixed
	// and scale keep the session at it.
	baseW, baseH int32
	// baseScale is the desktop scale factor the session was started with; scale
	// mode keeps the same visual size when the resolution grows.
	baseScale int32
	// modeBeforeFullscreen is the size mode full screen turned off, and what
	// leaving full screen turns back on; empty when there is none.
	modeBeforeFullscreen string
	// fixedW, fixedH are the outer size a fixed window is locked to.
	fixedW, fixedH int32
	watchdogTicks  int

	restorePlacement win.WINDOWPLACEMENT
	restoreStyle     uint32
	restoreExStyle   uint32
	hasRestore       bool

	clipStarted bool
	discDone    bool
	discSeen    bool
	// resizable is whether the window got WS_THICKFRAME: fixed size windows
	// have no grip to drag.
	resizable bool
	// hoverTracked is whether a leave message is on its way for the bar.
	hoverTracked bool
	// barHoverCmd is the bar button under the pointer, 0 for none: the bar
	// paints it highlighted.
	barHoverCmd uintptr
}

var gHost *hostState

// viewSize is the desktop the session should use now: the control's window,
// which leaves the bar out (and the bar is hidden in full screen). It is the
// same size the session connected with, from the same function, so dragging
// the window does not quietly change the meaning of "the window's pixels".
func (s *hostState) viewSize() (int32, int32) {
	hwnd := s.view
	if hwnd == 0 {
		hwnd = s.hwnd
	}
	return viewSize(hwnd, &s.opt)
}

func (s *hostState) resizeAx() {
	if s.hwnd == 0 || s.host == nil {
		return
	}
	w, h := clientDesktopSize(s.hwnd)
	s.placeView(w, h)
}

// placeView puts the control's window under the bar, inside the ring the
// window resizes with, and says the same thing to the control itself.
func (s *hostState) placeView(w, h int32) {
	bar := s.barHeight()
	grip := s.grip()
	vw := maxI32(w-2*grip, 1)
	vh := maxI32(h-bar-grip, 1)
	if s.view != 0 {
		win.SetWindowPos(s.view, 0, grip, bar, vw, vh, win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	}
	if s.host != nil {
		s.host.setRect(0, 0, vw, vh)
	}
}

// grip is the width of the strip around the picture that resizes the window:
// the window draws no frame, so the strip has to be inside it.
func (s *hostState) grip() int32 {
	if !s.resizable || s.fullscreen || s.sizeMode == store.SizeFixed {
		return 0
	}
	return kGrip
}

// maxI32 keeps a size at least at the floor.
func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

// applyDisplay says the size the session desktop should have now. Fit is the
// only mode that changes it: fixed and scale keep the resolution the session
// connected with, and the control fits that picture into the window (scale;
// SmartSizing shrinks but never magnifies), or the window never moves away
// from it (fixed).
func applyDisplay(s *hostState) bool {
	if s.host == nil || s.host.disp == nil || !s.loggedIn {
		return false
	}
	w, h := s.viewSize()
	ds := s.explicitScale()
	if s.sizeMode != store.SizeFit {
		if s.baseW <= 0 || s.baseH <= 0 {
			// 还没连上，基准尺寸未知，先不动。
			return true
		}
		// 固定与缩放都不换分辨率：会话一直是连接时那个桌面，
		// 只有窗口在动（缩放档由 SmartSizing 把画面缩到放得下，放大不了）。
		w, h, ds = s.baseW, s.baseH, s.baseScale
	}
	dev := deviceScaleFor(ds)
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
			logWarn("Reconnect 尺寸回退失败 hr=" + formatHr(uint32(hr)) + " status=" + win32.Itoa(int(status)))
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

// explicitScale is the desktop scale the user asked for with --scale; 0 asks
// for no scaling at all. Nothing follows the screen's DPI any more: the
// window is DPI aware already, so its pixels are the desktop's pixels.
func (s *hostState) explicitScale() int32 {
	// Only the scale mode asks for a desktop scale: fit follows the window's
	// pixels and fixed keeps the size it connected with, so a --scale on
	// either of them would magnify on top of that.
	if s.sizeMode != store.SizeScale {
		return 100
	}
	ds, _ := resolveScale(s.hwnd, &s.opt)
	return ds
}

// toggleFullscreen flips full screen. Ctrl+Alt+F reaches both this window and
// the control, which would otherwise toggle twice.
func (s *hostState) toggleFullscreen() {
	if !s.toggleAllowed() {
		return
	}
	if s.fullscreen {
		s.leaveFullscreen()
		return
	}
	s.enterFullscreen()
}

// toggleAllowed says whether a toggle may happen now, remembering this one.
func (s *hostState) toggleAllowed() bool {
	now := time.Now()
	if !s.lastToggle.IsZero() && now.Sub(s.lastToggle) < 400*time.Millisecond {
		return false
	}
	s.lastToggle = now
	return true
}

// suspendSizeModeForFullscreen hands the window to the screen while it is
// full screen: fixed and scale would keep the session at a desktop that is
// not the one filling the screen. leaveFullscreen puts the mode back.
func (s *hostState) suspendSizeModeForFullscreen() {
	if s.sizeMode == store.SizeFit {
		return
	}
	logWarn("全屏模式忽略 size=" + s.sizeMode + "，按跟随屏幕处理")
	s.modeBeforeFullscreen = s.sizeMode
	s.sizeMode = store.SizeFit
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
	// 全屏就是让画面填满屏幕：先把尺寸档交给屏幕，退出全屏再拿回来。
	s.suspendSizeModeForFullscreen()
	win.SetWindowLongPtr(s.hwnd, win.GWL_STYLE, uintptr(win.WS_POPUP|win.WS_VISIBLE))
	win.SetWindowLongPtr(s.hwnd, win.GWL_EXSTYLE, uintptr(s.restoreExStyle&^uint32(win.WS_EX_WINDOWEDGE)))
	win.SetWindowPos(s.hwnd, win.HWND_TOP, mi.RcMonitor.Left, mi.RcMonitor.Top,
		mi.RcMonitor.Right-mi.RcMonitor.Left, mi.RcMonitor.Bottom-mi.RcMonitor.Top,
		win.SWP_FRAMECHANGED|win.SWP_NOZORDER)
	s.resizeAx()
	win.PostMessage(s.hwnd, win.WM_SIZE, 0, 0)
	win.InvalidateRect(s.hwnd, nil, true)
	s.layoutOverlay()
	win.SetTimer(s.hwnd, kTimerOverlay, 120, 0)
	logInfo("Fullscreen entered（Ctrl+Alt+F 退出）")
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
	if s.modeBeforeFullscreen != "" {
		// 全屏期间停用的尺寸模式回来了。
		s.sizeMode = s.modeBeforeFullscreen
		s.modeBeforeFullscreen = ""
		logf("恢复窗口尺寸模式 %s", s.sizeMode)
	}
	// 全屏把会话改成了屏幕大小，回到窗口后要把分辨率改回来。固定与缩放
	// 都在这里拿回连接时的基准，不等 WM_SIZE（固定和缩放档的 WM_SIZE 不重设）。
	applyDisplay(s)
	win.PostMessage(s.hwnd, win.WM_SIZE, 0, 0)
	win.InvalidateRect(s.hwnd, nil, true)
	s.layoutOverlay()
	win.KillTimer(s.hwnd, kTimerOverlay)
	logInfo("Fullscreen left")
}

// requestClose asks for the window to close. It may arrive twice (the user
// clicked X, and the control then reported the disconnect); the WM_CLOSE
// handler does the actual work only once.
func (s *hostState) requestClose() {
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
		s.invalidateBar()
		if s.baseW == 0 {
			s.baseW, s.baseH = s.viewSize()
			// 连接时的缩放就是基准：scale 档以后按窗口/基准的比例往上走。
			s.baseScale = s.explicitScale()
			logf("会话尺寸基准 %dx%d（size=%s）", s.baseW, s.baseH, s.sizeMode)
		}
		if s.opt.Fullscreen {
			s.host.setFullScreen(true)
		}
	case 3: // OnLogin
		s.loggedIn = true
		s.invalidateBar()
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
		s.discSeen = true
		s.loggedIn = false
		s.invalidateBar()
		if s.retrying {
			// 重连时断开是它自己要求的第一步。
			return
		}
		if code, detail, ok := faultForDisconnect(s.discReason, s.extReason, s.everConnected); ok {
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

func wndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	s := gHost
	if s == nil {
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	}
	switch msg {
	case win.WM_NCCALCSIZE:
		// 无边框窗口：客户区就是整个窗口，本进程在里面自己画标题栏。
		if wParam == 0 {
			break
		}
		if p := ptrAt[nccalcParams](lParam); p != nil && !s.fullscreen && win.IsZoomed(hwnd) {
			// 最大化时系统给的位置比工作区大出一圈边框，要收回来，
			// 否则客户区会有半边在屏幕外。
			b := int32(win.GetSystemMetrics(win.SM_CXSIZEFRAME)) + int32(win.GetSystemMetrics(smCXPaddedBorder))
			p.Rgv[0].Left += b
			p.Rgv[0].Top += b
			p.Rgv[0].Right -= b
			p.Rgv[0].Bottom -= b
		}
		return 0
	case win.WM_NCHITTEST:
		if s.fullscreen || s.barHeight() == 0 {
			break
		}
		if pt := screenPoint(lParam); win.ScreenToClient(hwnd, &pt) {
			return s.windowHit(pt.X, pt.Y)
		}
	case win.WM_NCLBUTTONDOWN:
		// 没有真正的标题栏，DefWindowProc 不会替我们按这几个按钮。
		switch wParam {
		case win.HTMINBUTTON:
			win.ShowWindow(hwnd, win.SW_MINIMIZE)
			return 0
		case win.HTMAXBUTTON:
			if win.IsZoomed(hwnd) {
				win.ShowWindow(hwnd, win.SW_RESTORE)
			} else {
				win.ShowWindow(hwnd, win.SW_MAXIMIZE)
			}
			return 0
		case win.HTCLOSE:
			s.requestClose()
			return 0
		}
	case win.WM_NCMOUSEMOVE:
		// 顶栏成了非客户区（HTCAPTION），悬停高亮要从这条消息里跟。
		if pt := screenPoint(lParam); win.ScreenToClient(hwnd, &pt) {
			s.barHover(pt.X, pt.Y)
		}
		if !s.hoverTracked {
			s.hoverTracked = true
			var tme win.TRACKMOUSEEVENT
			tme.CbSize = uint32(unsafe.Sizeof(tme))
			tme.DwFlags = win.TME_LEAVE | win.TME_NONCLIENT
			tme.HwndTrack = hwnd
			win.TrackMouseEvent(&tme)
		}
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	case win.WM_NCMOUSELEAVE, win.WM_MOUSELEAVE:
		s.hoverTracked = false
		s.barHoverAtPointer()
		return 0
	case win.WM_SIZE:
		if win.IsIconic(hwnd) {
			// 最小化时客户区是 0，别拿它去改会话分辨率。
			return 0
		}
		// 最大化/还原会换掉最右那个按钮的字形。
		s.invalidateBar()
		s.layoutOverlay()
		s.resizeAx()
		if s.sizeMode == store.SizeFixed {
			// 固定大小：窗口拖不动，但进出全屏后会话要回到连接时的大小，
			// 所以照样对齐一次（没变的时候 applyDisplay 自己就返回了）。
			applyDisplay(s)
			return 0
		}
		if s.sizeMode == store.SizeScale {
			// 缩放：会话分辨率不变，画面由控件的 SmartSizing 缩到放得下
			// （只缩不放），不用去重设显示设置（退出全屏时 leaveFullscreen 会再对齐一次）。
			return 0
		}
		win.SetTimer(hwnd, kTimerResize, 400, 0)
		return 0
	case win.WM_DPICHANGED:
		if s.sizeMode == store.SizeFixed || win.IsIconic(hwnd) {
			return 0
		}
		if r := ptrAt[win.RECT](lParam); r != nil {
			win.SetWindowPos(hwnd, 0, r.Left, r.Top, r.Right-r.Left, r.Bottom-r.Top,
				win.SWP_NOZORDER|win.SWP_NOACTIVATE)
		}
		s.resizeAx()
		if s.sizeMode == store.SizeScale {
			// 缩放档的分辨率不跟窗口走，画面只是被缩到放得下，DPI 变了也不用重设。
			return 0
		}
		win.SetTimer(hwnd, kTimerResize, 400, 0)
		return 0
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_GETMINMAXINFO:
		mmi := ptrAt[win.MINMAXINFO](lParam)
		if s.sizeMode == store.SizeFixed && !s.fullscreen && s.fixedW > 0 && s.fixedH > 0 {
			// 固定大小：最小 = 最大，拖不动，也最大化不了。
			mmi.PtMinTrackSize.X = s.fixedW
			mmi.PtMinTrackSize.Y = s.fixedH
			mmi.PtMaxTrackSize.X = s.fixedW
			mmi.PtMaxTrackSize.Y = s.fixedH
			return 0
		}
		mmi.PtMinTrackSize.X = kMinDesktopW
		mmi.PtMinTrackSize.Y = kMinDesktopH
		return 0
	case wmRdpDisconnected:
		s.requestClose()
		return 0
	case wmRdpLaunched:
		return s.onLaunchResult(wParam, lParam)
	case win.WM_HOTKEY:
		if int(wParam) == hotkeyFullScreen {
			s.toggleFullscreen()
		}
		return 0
	case win.WM_KEYDOWN, win.WM_SYSKEYDOWN:
		if wParam == win.VK_ESCAPE && s.fullscreen {
			s.leaveFullscreen()
			return 0
		}
	case win.WM_SYSCOMMAND:
		// 全屏时禁掉系统菜单（Alt+空格）与最大化，只留还原。
		if s.fullscreen && (wParam&0xFFF0) == win.SC_MAXIMIZE {
			return 0
		}
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
				s.fail(fault.ConnectTimeout, sprintf("%d 秒内没有连上", s.opt.TimeoutSec))
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
		case kTimerReconnect:
			win.KillTimer(hwnd, kTimerReconnect)
			s.retrying = false
			s.resetConnection()
			s.resizeAx()
			if s.host == nil || !s.host.configure(&s.opt) {
				logError("重连失败：控件配置出错")
				printErr("Reconnect failed: the control could not be configured.")
				return 0
			}
			s.host.connect()
			logInfo("重连：已发出 Connect()")
			return 0
		case kTimerQuit:
			win.KillTimer(hwnd, kTimerQuit)
			logf("kTimerQuit 兜底退出")
			// 不调用 DestroyWindow：控件在 WM_DESTROY 里会卡住。直接让消息循环退出。
			win.PostQuitMessage(0)
			return 0
		case kTimerOverlay:
			s.trackOverlay()
			return 0
		}
		return 0
	case win.WM_PAINT:
		var ps win.PAINTSTRUCT
		hdc := win.BeginPaint(hwnd, &ps)
		if hdc != 0 {
			s.drawBar(hdc)
			win.EndPaint(hwnd, &ps)
		}
		return 0
	case win.WM_MOUSEMOVE:
		s.barHover(int32(int16(lParam&0xFFFF)), int32(int16((lParam>>16)&0xFFFF)))
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	case win.WM_LBUTTONDOWN:
		bx := int32(int16(lParam & 0xFFFF))
		by := int32(int16((lParam >> 16) & 0xFFFF))
		if s.barClick(bx, by) {
			return 0
		}
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	case wmRdpCommand:
		s.command(wParam)
		return 0
	case win.WM_CLOSE:
		logf("WM_CLOSE ever=%v done=%v seen=%v", s.everConnected, s.discDone, s.discSeen)
		if !s.everConnected {
			win.DestroyWindow(hwnd)
			return 0
		}
		if s.discSeen || s.discDone {
			// 断开已经发生（或者是第二次关闭请求）：控件只剩白板，退出。
			logf("会话已断开，退出窗口")
			win.PostQuitMessage(0)
			return 0
		}
		s.discDone = true
		if s.host.disp != nil {
			hr := (disp{s.host.disp}).call("Disconnect")
			logf("Disconnect() hr=%s", formatHr(uint32(hr)))
		}
		// 正常情况等 OnDisconnected 事件收尾；定时器和下面的 goroutine 只是
		// 兜底——控件偶尔不上报断开，而窗口定时器在断开后不一定还会触发。
		win.SetTimer(hwnd, kTimerQuit, 3000, 0)
		go func(h win.HWND) {
			sleepMS(2500)
			win.PostMessage(h, wmRdpDisconnected, 0, 0)
		}(hwnd)
		return 0
	case win.WM_DESTROY:
		win32.UnregisterHotKey(uintptr(hwnd), hotkeyFullScreen)
		// 只发退出信号；COM 对象在消息循环结束后再释放，
		// 避免在控件自己的回调栈里 Release 导致卡死。
		for _, id := range []uintptr{kTimerWatchdog, kTimerLaunch, kTimerQuit, kTimerResize, kTimerDisplay, kTimerClip, kTimerReconnect, kTimerOverlay} {
			win.KillTimer(hwnd, id)
		}
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

var wndProcCb = syscall.NewCallback(wndProc)

// smCXPaddedBorder is SM_CXPADDEDBORDER, which lxn/win does not name: the
// extra ring Windows adds around a resizable frame.
const smCXPaddedBorder = 92

// nccalcParams is NCCALCSIZE_PARAMS: the rectangles Windows proposes for the
// client area, the window itself, and the client area before the change.
type nccalcParams struct {
	Rgv    [3]win.RECT
	LpPos  unsafe.Pointer
	LpRect unsafe.Pointer
}

// screenPoint unpacks the pointer position mouse messages carry in lParam.
func screenPoint(lParam uintptr) win.POINT {
	return win.POINT{
		X: int32(int16(lParam & 0xFFFF)),
		Y: int32(int16((lParam >> 16) & 0xFFFF)),
	}
}

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
		ch = opt.Height + int(barHeightFor(opt))
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
// pickLanguage speaks the language the window is set to: the host runs as a
// separate process, so it cannot inherit it.
func pickLanguage() {
	lang := store.LoadSettings().Lang
	if lang == "" {
		lang = i18n.Detect()
	}
	i18n.Set(lang)
}

func RunRdpSession(opt StartOptions, jobPath string) int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pickLanguage()

	icc := win.INITCOMMONCONTROLSEX{DwSize: uint32(unsafe.Sizeof(win.INITCOMMONCONTROLSEX{})), DwICC: win.ICC_WIN95_CLASSES}
	win.InitCommonControlsEx(&icc)

	hr := win.OleInitialize()
	if hrFailed(uintptr(uint32(hr))) && hr != 1 && uint32(hr) != 0x80010106 {
		printErr("OleInitialize failed: " + formatHr(uint32(hr)))
		return int(ExitError)
	}
	defer win.OleUninitialize()

	s := &hostState{opt: opt, jobPath: jobPath, host: newAxHost()}
	s.sizeMode = store.NormalizeSizeMode(opt.SizeMode)
	if opt.Fullscreen {
		// 全屏就是让画面填满屏幕，和尺寸档冲突：全屏优先。
		// 只停用全屏期间的模式，退出全屏后照旧生效。
		s.suspendSizeModeForFullscreen()
	}
	logf("窗口尺寸模式 %s", s.sizeMode)
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
	if s.sizeMode == store.SizeFixed {
		// 固定大小：没有可拖的边框，也没有最大化按钮。
		style &^= win.WS_THICKFRAME | win.WS_MAXIMIZEBOX
	}

	// 无边框：窗口矩形就是客户区，不再往外加一圈看不见的边框。
	rc := win.RECT{Left: x, Top: y, Right: x + cw, Bottom: y + ch}
	s.fixedW, s.fixedH = cw, ch
	s.resizable = style&win.WS_THICKFRAME != 0

	inst := win.GetModuleHandle(nil)
	appIcon := loadAppIcon(inst, win.GetSystemMetrics(win.SM_CXICON))
	appIconSm := loadAppIcon(inst, win.GetSystemMetrics(win.SM_CXSMICON))
	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		Style:         win.CS_HREDRAW | win.CS_VREDRAW,
		LpfnWndProc:   wndProcCb,
		HInstance:     inst,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(uintptr(win.IDC_ARROW))),
		HbrBackground: win.HBRUSH(win.GetStockObject(win.BLACK_BRUSH)),
		HIcon:         appIcon,
		HIconSm:       appIconSm,
		LpszClassName: win32.UTF16Ptr(className),
	}
	if win.RegisterClassEx(&wc) == 0 {
		printErr("RegisterClassEx failed: " + formatWin32(uint32(win.GetLastError())))
		return int(ExitError)
	}
	s.hwnd = win.CreateWindowEx(exStyle, win32.UTF16Ptr(className), win32.UTF16Ptr(opt.Title), style,
		rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, 0, 0, inst, nil)
	if s.hwnd == 0 {
		printErr("CreateWindowEx failed: " + formatWin32(uint32(win.GetLastError())))
		return int(ExitError)
	}
	// 窗口自己再挂一次图标：只写在窗口类里时，任务栏缩略图和 Alt+Tab 有时会画成空壳。
	win.SendMessage(s.hwnd, win.WM_SETICON, iconBig, uintptr(appIcon))
	win.SendMessage(s.hwnd, win.WM_SETICON, iconSmall, uintptr(appIconSm))
	if opt.Fullscreen {
		// 走同一条路，才能记住原样式/位置，Ctrl+Alt+F 才退得回来。
		s.enterFullscreen()
	} else {
		// 建窗口时系统只问了不带 wParam 的那一次，客户区还是按有标题栏算的；
		// 这一句让它按新的算法重算一次，标题栏才真的消失。
		win.SetWindowPos(s.hwnd, 0, 0, 0, 0, 0,
			win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	}
	win.ShowWindow(s.hwnd, show)
	if show != int32(win.SW_HIDE) && !win.IsWindowVisible(s.hwnd) {
		// A process started with STARTF_USESHOWWINDOW/SW_HIDE (see
		// win32.HideWindow) has its very first ShowWindow ignored, which left
		// the window built but never seen. A second call does show it.
		win.ShowWindow(s.hwnd, int32(win.SW_SHOW))
		win.SetForegroundWindow(s.hwnd)
	}
	win.UpdateWindow(s.hwnd)
	if !win32.RegisterHotKey(uintptr(s.hwnd), hotkeyFullScreen, win32.ModControl|win32.ModAlt|win32.ModNoRepeat, 'F') {
		logWarn("Ctrl+Alt+F 热键注册失败，只能用顶栏按钮切全屏")
	}

	// 抽干一次消息队列，避免尺寸消息早于控件创建到达。
	var msg win.MSG
	for win.PeekMessage(&msg, 0, 0, 0, win.PM_REMOVE) {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}

	var created bool
	// 控件住在一个子窗口里：上面的栏是本进程画的，控件只管栏下面那块，
	// 四周再留一圈给鼠标拖拽改大小。
	bar := barHeightFor(&opt)
	grip := int32(kGrip)
	if !s.resizable || s.sizeMode == store.SizeFixed {
		grip = 0
	}
	s.view = win.CreateWindowEx(0, win32.UTF16Ptr("STATIC"), nil,
		win.WS_CHILD|win.WS_VISIBLE|win.WS_CLIPCHILDREN|win.WS_CLIPSIBLINGS,
		grip, bar, cw-2*grip, ch-bar-grip, s.hwnd, 0, inst, nil)
	if s.view == 0 {
		logWarn("创建控件容器失败: " + formatWin32(uint32(win.GetLastError())))
	}
	s.createOverlay(inst)
	for i := range clsidRdpClients {
		if err := s.host.create(&clsidRdpClients[i], s.view); err != nil {
			logWarn("创建 RDP 控件失败: " + err.Error())
			continue
		}
		created = true
		logf("RDP 控件已创建 clsid[%d]", i)
		break
	}
	if !created {
		s.fail(fault.ControlBroken, "创建控件失败")
		return int(ExitError)
	}
	if opt.ViewOnly {
		if w := s.host.controlWindow(); w != 0 {
			win.EnableWindow(w, false)
		}
	}
	s.host.onEvent = s.onEvent

	if !s.host.configure(&opt) {
		s.fail(fault.NoChildSupport, "")
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
		// 具体原因已经由 fail() 说过一次了。
		return int(ExitConnectFailed)
	}
	if s.everConnected {
		printOut(describeDisconnect(s.discReason, s.extReason))
	}
	return int(ExitOk)
}
