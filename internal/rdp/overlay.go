package rdp

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/internal/win32"
)

// The overlay is the bar's three buttons, floated over the picture while the
// window is full screen: the bar is hidden there, and with it the only way to
// reconnect or to come back out without typing a hot key.
const (
	overlayClass = "LocalRDP.Overlay"
	// kOverlayTop is the gap between the top edge and the floating bar.
	kOverlayTop = 10
	// kOverlayReveal is how close to the top edge the pointer has to be for
	// the bar to come out: it is a strip of the remote desktop otherwise.
	kOverlayReveal = 4
	// kOverlaySlack is how far below the bar the pointer may wander without
	// the bar going away again, so it can be reached from the edge.
	kOverlaySlack = 8
	// maNoActivate is MA_NOACTIVATE, which lxn/win does not name.
	maNoActivate = 3
)

var (
	procCreateRoundRectRgn = gdi32.NewProc("CreateRoundRectRgn")
	procSetWindowRgn       = user32.NewProc("SetWindowRgn")
)

// overlaySize is the floating bar's size: the three buttons with a padding on
// both sides, as tall as the bar itself.
func overlaySize(m barMetrics) (int32, int32) {
	hdc := win.GetDC(0)
	if hdc == 0 {
		return 0, 0
	}
	btns := barModel{m: m}.buttons(hdc, 1<<14)
	win.ReleaseDC(0, hdc)
	if len(btns) == 0 {
		return 0, 0
	}
	total := (1 << 14) - m.pad - btns[len(btns)-1].rect.Left
	return total + 2*m.pad, m.h
}

// createOverlay makes the floating bar. It stays hidden until the window goes
// full screen.
func (s *hostState) createOverlay(inst win.HINSTANCE) {
	if s.overlay != 0 || s.hwnd == 0 {
		return
	}
	w, h := overlaySize(s.m)
	if w == 0 || h == 0 {
		return
	}
	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   overlayProcCb,
		HInstance:     inst,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(uintptr(win.IDC_ARROW))),
		HbrBackground: win.HBRUSH(win.GetStockObject(win.BLACK_BRUSH)),
		LpszClassName: win32.UTF16Ptr(overlayClass),
	}
	win.RegisterClassEx(&wc)
	s.overlay = win.CreateWindowEx(win.WS_EX_NOACTIVATE, win32.UTF16Ptr(overlayClass), nil,
		win.WS_CHILD|win.WS_CLIPCHILDREN, 0, 0, w, h, s.hwnd, 0, inst, nil)
	if s.overlay == 0 {
		logWarn("创建全屏悬浮条失败: " + formatWin32(uint32(win.GetLastError())))
		return
	}
	// 圆角靠窗口区域裁：一块矩形窗口的四个角会被切掉，不用透明通道。
	if rgn, _, _ := procCreateRoundRectRgn.Call(0, 0, uintptr(w+1), uintptr(h+1), 12, 12); rgn != 0 {
		procSetWindowRgn.Call(uintptr(s.overlay), rgn, 1)
	}
	s.layoutOverlay()
}

// layoutOverlay puts the floating bar in the middle of the top edge while
// the window is full screen, and keeps it out of the way the rest of the time.
func (s *hostState) layoutOverlay() {
	if s.overlay == 0 {
		return
	}
	if !s.fullscreen || !s.overlayShown {
		s.overlayShown = false
		s.overlayHover = 0
		win.ShowWindow(s.overlay, win.SW_HIDE)
		return
	}
	w, h := overlaySize(s.m)
	cw, _ := clientDesktopSize(s.hwnd)
	x := (cw - w) / 2
	if x < 0 {
		x = 0
	}
	win.SetWindowPos(s.overlay, win.HWND_TOP, x, kOverlayTop, w, h,
		win.SWP_NOACTIVATE|win.SWP_SHOWWINDOW)
}

// trackOverlay watches the pointer on the timer full screen runs: touching
// the top edge brings the bar out, and it goes away again once the pointer is
// below it. Nothing else on the picture moves.
func (s *hostState) trackOverlay() {
	if s.overlay == 0 || !s.fullscreen {
		return
	}
	var pt win.POINT
	if !win.GetCursorPos(&pt) {
		return
	}
	if !win.ScreenToClient(s.hwnd, &pt) {
		return
	}
	_, h := overlaySize(s.m)
	show := pt.Y >= 0 && pt.Y <= kOverlayReveal
	if !show && s.overlayShown {
		// The pointer is on its way down from the edge to the buttons.
		show = pt.Y >= 0 && pt.Y <= kOverlayTop+h+kOverlaySlack
	}
	if show == s.overlayShown {
		return
	}
	s.overlayShown = show
	s.layoutOverlay()
}

// overlayModel is what the floating bar draws with: the same three buttons as
// the bar, with the one under the pointer lit.
func (s *hostState) overlayModel() barModel {
	return barModel{m: s.m, hover: s.overlayHover}
}

func (s *hostState) drawOverlay(hdc win.HDC) {
	w, h := overlaySize(s.m)
	fillSolid(hdc, win.RECT{Right: w, Bottom: h}, barBg)
	win.SetBkMode(hdc, win.TRANSPARENT)
	m := s.overlayModel()
	btns := m.buttons(hdc, w)
	for _, b := range btns {
		bg := b.bg
		if b.cmd == m.hover {
			bg = b.hi
		}
		fillRoundBorder(hdc, b.rect, bg, b.bd, m.m.radius)
		at := b.rect.Left + m.m.btnPad
		barGlyph(hdc, b.glyph, win.RECT{Left: at, Top: b.rect.Top, Right: at + m.m.btnIcon, Bottom: b.rect.Bottom + 1},
			b.fg, m.m.btnGlyph)
		barText(hdc, b.label, &win.RECT{Left: at + m.m.btnIcon + m.m.iconGap, Top: b.rect.Top,
			Right: b.rect.Right - m.m.btnPad + 2, Bottom: b.rect.Bottom},
			b.fg, barFont(m.m.font, win.FW_NORMAL), win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE)
	}
	// 一条上边线，和顶栏一样，浮在画面上时不至于糊在一起。
	fillSolid(hdc, win.RECT{Right: w, Bottom: 1}, barLine)
}

// overlayAt is the command of the button at x, y of the floating bar.
func overlayAt(m barMetrics, x, y int32) uintptr {
	w, _ := overlaySize(m)
	hdc := win.GetDC(0)
	if hdc == 0 {
		return 0
	}
	btns := barModel{m: m}.buttons(hdc, w)
	win.ReleaseDC(0, hdc)
	for _, b := range btns {
		if x >= b.rect.Left && x < b.rect.Right && y >= b.rect.Top && y < b.rect.Bottom {
			return b.cmd
		}
	}
	return 0
}

func overlayProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	s := gHost
	if s == nil {
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	}
	switch msg {
	case win.WM_MOUSEACTIVATE:
		// 点它不该把键盘焦点从画面上抢走。
		return maNoActivate
	case win.WM_ERASEBKGND:
		return 1
	case win.WM_PAINT:
		var ps win.PAINTSTRUCT
		hdc := win.BeginPaint(hwnd, &ps)
		if hdc != 0 {
			s.drawOverlay(hdc)
			win.EndPaint(hwnd, &ps)
		}
		return 0
	case win.WM_MOUSEMOVE:
		x := int32(int16(lParam & 0xFFFF))
		y := int32(int16((lParam >> 16) & 0xFFFF))
		cmd := overlayAt(s.m, x, y)
		if cmd != s.overlayHover {
			s.overlayHover = cmd
			win.InvalidateRect(hwnd, nil, true)
		}
		if !s.overlayTracked {
			s.overlayTracked = true
			var tme win.TRACKMOUSEEVENT
			tme.CbSize = uint32(unsafe.Sizeof(tme))
			tme.DwFlags = win.TME_LEAVE
			tme.HwndTrack = hwnd
			win.TrackMouseEvent(&tme)
		}
		return 0
	case win.WM_MOUSELEAVE:
		s.overlayTracked = false
		if s.overlayHover != 0 {
			s.overlayHover = 0
			win.InvalidateRect(hwnd, nil, true)
		}
		return 0
	case win.WM_LBUTTONDOWN:
		x := int32(int16(lParam & 0xFFFF))
		y := int32(int16((lParam >> 16) & 0xFFFF))
		if cmd := overlayAt(s.m, x, y); cmd != 0 {
			s.command(cmd)
		}
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

var overlayProcCb = syscall.NewCallback(overlayProc)
