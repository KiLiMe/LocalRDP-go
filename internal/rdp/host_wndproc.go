package rdp

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/internal/fault"
	"localrdp/internal/store"
	"localrdp/internal/win32"
)

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
		// 顶栏的尺寸跟着 dpi 走：拖到另一块屏上时重新算一遍。
		s.setDPI(windowDPI(uintptr(hwnd)))
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
			// 不 DestroyWindow：控件在 WM_DESTROY 里拆会卡住（就是下面那个
			// 定时器分支绕开的那件事），进程会留着没有窗口、把 exe 锁死。
			win.ShowWindow(hwnd, win.SW_HIDE)
			win.PostQuitMessage(0)
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
