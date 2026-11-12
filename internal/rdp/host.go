// Package rdp 的会话窗口按职责分文件，读改时只看需要的那一个：
//
//	host.go            窗口状态、定时器/消息常量、控件布局（viewSize/placeView）
//	host_display.go    会话分辨率与缩放协商（applyDisplay、size=fit|fixed|scale）
//	host_fullscreen.go 全屏进出（Ctrl+Alt+F）与尺寸档挂起/恢复
//	host_events.go     控件事件回调（On*）与子会话内进程启动（异步 + 回投）
//	host_wndproc.go    窗口过程：消息分发、命中测试、非客户区、定时器
//	host_window.go     初始窗口矩形（工作区 5/6、DPI 换算）与关闭请求
//	host_run.go        RunRdpSession：类注册 → 控件创建 → 消息循环 → 清理
package rdp

import (
	"time"

	"github.com/lxn/win"

	"localrdp/internal/fault"
	"localrdp/internal/store"
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
	// m is the bar's numbers for the dpi of the screen the window is on.
	m barMetrics
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
	return s.m.grip
}

// maxI32 keeps a size at least at the floor.
func maxI32(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}
