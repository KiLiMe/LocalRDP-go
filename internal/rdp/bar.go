package rdp

import (
	"unsafe"

	"github.com/lxn/win"
	"localrdp/internal/i18n"
	"localrdp/internal/win32"
)

// The bar the host window draws above the Remote Desktop control. It says
// what the session is doing and offers the three things worth doing with it:
// reconnect, close, and full screen.
const (
	kBarH      = 36
	barPadding = 14
	barBtnW    = 76
	barBtnH    = 24
	barBtnGap  = 8

	// wmRdpCommand asks the host window to act, from the interface or from
	// another process (see win32.PostCommand).
	wmRdpCommand  = win32.HostCommandMessage
	cmdReconnect  = win32.CmdReconnect
	cmdDisconnect = win32.CmdDisconnect
	cmdFullScreen = win32.CmdFullScreen
)

var (
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procFillRect         = user32.NewProc("FillRect")
	procDrawTextW        = user32.NewProc("DrawTextW")
)

// barHeightFor is the bar the window keeps above the control; full screen
// has none, the picture takes the whole monitor.
func barHeightFor(opt *StartOptions) int32 {
	if opt != nil && opt.Fullscreen {
		return 0
	}
	return kBarH
}

func (s *hostState) barHeight() int32 {
	if s.fullscreen {
		return 0
	}
	return kBarH
}

// barColor is a COLORREF: 0x00BBGGRR.
func barColor(r, g, b uint32) uint32 { return r | g<<8 | b<<16 }

func fillSolid(hdc win.HDC, rc win.RECT, color uint32) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(color))
	if brush == 0 {
		return
	}
	procFillRect.Call(uintptr(hdc), uintptr(unsafe.Pointer(&rc)), brush)
	win.DeleteObject(win.HGDIOBJ(brush))
}

func fillRound(hdc win.HDC, rc win.RECT, color uint32, radius int32) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(color))
	if brush == 0 {
		return
	}
	pen, _, _ := procCreatePen.Call(uintptr(win.PS_SOLID), 1, uintptr(color))
	oldBrush := win.SelectObject(hdc, win.HGDIOBJ(brush))
	oldPen := win.SelectObject(hdc, win.HGDIOBJ(pen))
	win.RoundRect(hdc, rc.Left, rc.Top, rc.Right, rc.Bottom, radius, radius)
	win.SelectObject(hdc, oldBrush)
	win.SelectObject(hdc, oldPen)
	win.DeleteObject(win.HGDIOBJ(brush))
	if pen != 0 {
		win.DeleteObject(win.HGDIOBJ(pen))
	}
}

func barFont(size, weight int32) win.HFONT {
	face := win32.UTF16Ptr("Microsoft YaHei UI")
	h, _, _ := procCreateFontW.Call(
		uintptr(int64(-size)), 0, 0, 0, uintptr(int64(weight)),
		0, 0, 0, uintptr(win.DEFAULT_CHARSET), 0, 0, 0,
		uintptr(win.CLEARTYPE_QUALITY), 0, uintptr(unsafe.Pointer(face)))
	return win.HFONT(h)
}

// barText draws s in rc and gives the font back.
func barText(hdc win.HDC, s string, rc *win.RECT, color uint32, font win.HFONT, flags uint32) {
	if font == 0 {
		return
	}
	old := win.SelectObject(hdc, win.HGDIOBJ(font))
	win.SetBkMode(hdc, win.TRANSPARENT)
	win.SetTextColor(hdc, win.COLORREF(color))
	procDrawTextW.Call(uintptr(hdc), uintptr(unsafe.Pointer(win32.UTF16Ptr(s))),
		^uintptr(0), uintptr(unsafe.Pointer(rc)), uintptr(flags))
	win.SelectObject(hdc, old)
	win.DeleteObject(win.HGDIOBJ(font))
}

type barButton struct {
	label string
	rect  win.RECT
	cmd   uintptr
}

// barButtons lays the buttons out from the right edge of the window.
func (s *hostState) barButtons(width int32) []barButton {
	height := s.barHeight()
	if height == 0 {
		return nil
	}
	items := []struct {
		label string
		cmd   uintptr
	}{
		{i18n.T("Disconnect"), cmdDisconnect},
		{i18n.T("Reconnect"), cmdReconnect},
		{i18n.T("Full screen"), cmdFullScreen},
	}
	y := (height - barBtnH) / 2
	x := width - barPadding
	out := make([]barButton, 0, len(items))
	for _, it := range items {
		x -= barBtnW
		out = append(out, barButton{
			label: it.label,
			cmd:   it.cmd,
			rect:  win.RECT{Left: x, Top: y, Right: x + barBtnW, Bottom: y + barBtnH},
		})
		x -= barBtnGap
	}
	return out
}

// drawBar paints the bar; it runs inside WM_PAINT, where the control does not
// paint (it starts below the bar).
func (s *hostState) drawBar(hdc win.HDC) {
	height := s.barHeight()
	if height == 0 {
		return
	}
	width, _ := clientDesktopSize(s.hwnd)
	fillSolid(hdc, win.RECT{Right: width, Bottom: height}, barColor(22, 26, 33))
	fillSolid(hdc, win.RECT{Top: height - 1, Right: width, Bottom: height}, barColor(44, 51, 64))

	text := "LocalRDP"
	switch {
	case s.loggedIn:
		text += "   " + i18n.T("Connected")
		if sid := getChildSessionID(); sid != kNoSession {
			text += " · " + i18n.T("Child session") + " " + win32.Itoa(int(sid))
		}
	case s.everConnected:
		text += "   " + i18n.T("Disconnected")
	default:
		text += "   " + i18n.T("Connecting…")
	}
	rc := win.RECT{Left: barPadding, Right: width - barPadding - 3*(barBtnW+barBtnGap), Bottom: height - 1}
	barText(hdc, text, &rc, barColor(226, 232, 240), barFont(13, win.FW_SEMIBOLD),
		win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_END_ELLIPSIS)

	for _, b := range s.barButtons(width) {
		fillRound(hdc, b.rect, barColor(38, 45, 58), 8)
		label := b.rect
		barText(hdc, b.label, &label, barColor(206, 214, 228), barFont(12, win.FW_NORMAL),
			win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE)
	}
}

// barClick handles a click at x, y: it says whether the bar took it.
func (s *hostState) barClick(x, y int32) bool {
	height := s.barHeight()
	if height == 0 || y < 0 || y >= height {
		return false
	}
	width, _ := clientDesktopSize(s.hwnd)
	for _, b := range s.barButtons(width) {
		if x >= b.rect.Left && x < b.rect.Right && y >= b.rect.Top && y < b.rect.Bottom {
			s.command(b.cmd)
			return true
		}
	}
	return true
}

func (s *hostState) command(cmd uintptr) {
	switch cmd {
	case cmdReconnect:
		s.reconnect()
	case cmdDisconnect:
		logInfo("栏：断开")
		s.requestClose()
	case cmdFullScreen:
		if s.fullscreen {
			s.leaveFullscreen()
		} else {
			s.enterFullscreen()
		}
	}
}

// invalidateBar repaints the bar after the session's state changed.
func (s *hostState) invalidateBar() {
	if s.hwnd == 0 || s.barHeight() == 0 {
		return
	}
	win.InvalidateRect(s.hwnd, &win.RECT{Right: 8192, Bottom: s.barHeight()}, false)
}

// reconnect drops the current connection and connects again with the same
// options: the cure for a picture that stopped moving.
func (s *hostState) reconnect() {
	if s.retrying || s.host == nil {
		return
	}
	s.retrying = true
	logInfo("重连：先断开当前连接")
	printOut("Reconnecting…")
	if s.host.disp != nil {
		if hr := (disp{s.host.disp}).call("Disconnect"); hr != 0 {
			logf("Disconnect() hr=0x%08X", uint32(hr))
		}
	}
	win.SetTimer(s.hwnd, kTimerReconnect, 1200, 0)
}

// resetConnection forgets what the old connection knew, so the new one
// starts from the window's size again.
func (s *hostState) resetConnection() {
	s.everConnected = false
	s.loggedIn = false
	s.discSeen = false
	s.discDone = false
	s.closing = false
	s.appliedW, s.appliedH, s.appliedScale = 0, 0, 0
	s.baseW, s.baseH, s.baseScale = 0, 0, 0
	s.clipStarted = false
	s.invalidateBar()
}
