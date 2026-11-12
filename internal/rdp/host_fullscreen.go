package rdp

import (
	"time"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/internal/store"
)

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
