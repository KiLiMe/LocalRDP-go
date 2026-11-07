package rdp

import (
	"unicode/utf16"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/internal/i18n"
	"localrdp/internal/win32"
)

// The bar the host window draws above the Remote Desktop control. On the left
// it says what the session is doing, on the right it offers the three things
// worth doing with it: full screen, reconnect, disconnect.
const (
	kBarH      = 40
	barPad     = 12
	barGap     = 10
	barPillH   = 22
	barPillGap = 6
	barBtnH    = 26
	barBtnGap  = 6
	barBtnPad  = 10
	barBtnIcon = 12
	barIconGap = 5

	// wmRdpCommand asks the host window to act, from the interface or from
	// another process (see win32.PostCommand).
	wmRdpCommand  = win32.HostCommandMessage
	cmdReconnect  = win32.CmdReconnect
	cmdDisconnect = win32.CmdDisconnect
	cmdFullScreen = win32.CmdFullScreen
)

// Glyphs of Segoe MDL2 Assets, the icons Windows itself uses.
const (
	glyphFullScreen = 0xE740
	glyphRefresh    = 0xE72C
	glyphPower      = 0xE7E8
	glyphPerson     = 0xE77B
	glyphFolder     = 0xE8F4 // NewFolder: the mark the window wears
)

// The bar keeps to the dark frame of the window, so the picture stays the
// only bright thing in it.
var (
	barBg      = barColor(20, 24, 31)
	barLine    = barColor(38, 43, 54)
	barTxHi    = barColor(230, 233, 238)
	barTxLo    = barColor(154, 164, 178)
	barChip    = barColor(31, 36, 46)
	barChipHi  = barColor(45, 52, 66)
	barChipBd  = barColor(44, 51, 65)
	barMark    = barColor(0, 103, 192)
	barWhite   = barColor(255, 255, 255)
	barBlue    = barColor(88, 166, 255)
	barAmber   = barColor(227, 179, 65)
	barAmberBg = barColor(45, 36, 19)
	barAmberBd = barColor(66, 52, 26)
	barGreen   = barColor(63, 185, 80)
	barGreenTx = barColor(86, 211, 100)
	barGreenBg = barColor(21, 36, 27)
	barGreenBd = barColor(32, 60, 41)
	barRedTx   = barColor(255, 123, 114)
	barRedBg   = barColor(45, 24, 26)
	barRedBd   = barColor(74, 35, 41)
	barRedHi   = barColor(58, 30, 32)
)

var (
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procFillRect         = user32.NewProc("FillRect")
	procDrawTextW        = user32.NewProc("DrawTextW")
	procTextExtent       = gdi32.NewProc("GetTextExtentPoint32W")
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
	fillRoundBorder(hdc, rc, color, color, radius)
}

// fillRoundBorder paints a rounded rectangle with its own edge, the shape of
// the pills and the buttons of the bar.
func fillRoundBorder(hdc win.HDC, rc win.RECT, fill, border uint32, radius int32) {
	brush, _, _ := procCreateSolidBrush.Call(uintptr(fill))
	if brush == 0 {
		return
	}
	pen, _, _ := procCreatePen.Call(uintptr(win.PS_SOLID), 1, uintptr(border))
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

func makeFont(face string, size, weight int32) win.HFONT {
	wface := win32.UTF16Ptr(face)
	h, _, _ := procCreateFontW.Call(
		uintptr(int64(-size)), 0, 0, 0, uintptr(int64(weight)),
		0, 0, 0, uintptr(win.DEFAULT_CHARSET), 0, 0,
		uintptr(win.CLEARTYPE_QUALITY), 0, uintptr(unsafe.Pointer(wface)))
	return win.HFONT(h)
}

func barFont(size, weight int32) win.HFONT {
	return makeFont("Microsoft YaHei UI", size, weight)
}

func barIconFont(size int32) win.HFONT {
	return makeFont("Segoe MDL2 Assets", size, win.FW_NORMAL)
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

// barGlyph draws one icon of the symbol font, centered in rc.
func barGlyph(hdc win.HDC, code uint32, rc win.RECT, color uint32, size int32) {
	barText(hdc, string(rune(code)), &rc, color, barIconFont(size),
		win.DT_CENTER|win.DT_VCENTER|win.DT_SINGLELINE|win.DT_NOPREFIX)
}

// barWidth is how wide s is in the given font; the font is given back.
func barWidth(hdc win.HDC, s string, font win.HFONT) int32 {
	if font == 0 {
		return 0
	}
	var size win.SIZE
	old := win.SelectObject(hdc, win.HGDIOBJ(font))
	procTextExtent.Call(uintptr(hdc), uintptr(unsafe.Pointer(win32.UTF16Ptr(s))),
		uintptr(len(utf16.Encode([]rune(s)))), uintptr(unsafe.Pointer(&size)))
	win.SelectObject(hdc, old)
	win.DeleteObject(win.HGDIOBJ(font))
	return size.CX
}

type barButton struct {
	label string
	rect  win.RECT
	cmd   uintptr
	glyph uint32
	fg    uint32
	bg    uint32
	bd    uint32
	hi    uint32
}

// barModel is what the bar says, apart from the window it is drawn in.
type barModel struct {
	loggedIn      bool
	everConnected bool
	sid           uint32
	hover         uintptr
}

// barModel costs nothing: the session id is only asked for when painting.
func (s *hostState) barModel() barModel {
	return barModel{
		loggedIn:      s.loggedIn,
		everConnected: s.everConnected,
		sid:           kNoSession,
		hover:         s.barHoverCmd,
	}
}

// status is the pill on the left: what the session is doing, in its color.
func (m barModel) status() (label string, dot, fg, bg, bd uint32) {
	switch {
	case m.loggedIn:
		return i18n.T("Connected"), barGreen, barGreenTx, barGreenBg, barGreenBd
	case m.everConnected:
		return i18n.T("Disconnected"), barTxLo, barTxLo, barChip, barChipBd
	default:
		return i18n.T("Connecting…"), barAmber, barAmber, barAmberBg, barAmberBd
	}
}

// buttons lays the three buttons out from the right edge of the window.
func (m barModel) buttons(hdc win.HDC, width int32) []barButton {
	defs := []barButton{
		{label: i18n.T("Disconnect"), cmd: cmdDisconnect, glyph: glyphPower,
			fg: barRedTx, bg: barRedBg, bd: barRedBd, hi: barRedHi},
		{label: i18n.T("Reconnect"), cmd: cmdReconnect, glyph: glyphRefresh,
			fg: barAmber, bg: barChip, bd: barChipBd, hi: barChipHi},
		{label: i18n.T("Full screen"), cmd: cmdFullScreen, glyph: glyphFullScreen,
			fg: barBlue, bg: barChip, bd: barChipBd, hi: barChipHi},
	}
	y := int32((kBarH - barBtnH) / 2)
	x := width - barPad
	out := make([]barButton, 0, len(defs))
	for _, d := range defs {
		w := 2*barBtnPad + barBtnIcon + barIconGap + barWidth(hdc, d.label, barFont(12, win.FW_NORMAL))
		x -= w
		d.rect = win.RECT{Left: x, Top: y, Right: x + w, Bottom: y + barBtnH}
		out = append(out, d)
		x -= barBtnGap
	}
	return out
}

// pill draws a rounded label and gives back the x the next one starts at.
func pill(hdc win.HDC, x, mid int32, label string, dot, fg, bg, bd uint32, glyph uint32) int32 {
	const pad, dotW, glyphW = int32(9), int32(6), int32(11)
	w := 2*pad + barWidth(hdc, label, barFont(12, win.FW_NORMAL))
	if dot != 0 {
		w += dotW + 6
	}
	if glyph != 0 {
		w += glyphW + barIconGap
	}
	rc := win.RECT{Left: x, Top: mid - barPillH/2, Right: x + w, Bottom: mid + (barPillH+1)/2}
	fillRoundBorder(hdc, rc, bg, bd, barPillH/2)
	at := x + pad
	if dot != 0 {
		fillRoundBorder(hdc, win.RECT{Left: at, Top: mid - dotW/2, Right: at + dotW, Bottom: mid + dotW/2},
			dot, dot, dotW/2)
		at += dotW + 6
	}
	if glyph != 0 {
		barGlyph(hdc, glyph, win.RECT{Left: at - 1, Top: rc.Top, Right: at + glyphW, Bottom: rc.Bottom}, fg, 10)
		at += glyphW + barIconGap
	}
	barText(hdc, label, &win.RECT{Left: at, Top: rc.Top, Right: rc.Right - pad + 2, Bottom: rc.Bottom},
		fg, barFont(12, win.FW_NORMAL), win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE)
	return rc.Right
}

// paint draws the whole bar and gives back the buttons, for hit testing.
func (m barModel) paint(hdc win.HDC, width int32) []barButton {
	height := int32(kBarH)
	fillSolid(hdc, win.RECT{Right: width, Bottom: height}, barBg)
	fillSolid(hdc, win.RECT{Top: height - 1, Right: width, Bottom: height}, barLine)

	mid := height / 2
	x := int32(barPad)

	// The mark: the accent square with the folder the window wears.
	fillRound(hdc, win.RECT{Left: x, Top: mid - 8, Right: x + 16, Bottom: mid + 8}, barMark, 5)
	barGlyph(hdc, glyphFolder, win.RECT{Left: x, Top: mid - 8, Right: x + 16, Bottom: mid + 9}, barWhite, 10)
	x += 16 + 8

	// The name.
	nameW := barWidth(hdc, "LocalRDP", barFont(13, win.FW_SEMIBOLD))
	barText(hdc, "LocalRDP", &win.RECT{Left: x, Top: 0, Right: x + nameW + 4, Bottom: height - 1},
		barTxHi, barFont(13, win.FW_SEMIBOLD), win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE)
	x += nameW + barGap

	// A hairline, then what the session is doing.
	fillSolid(hdc, win.RECT{Left: x, Top: mid - 8, Right: x + 1, Bottom: mid + 8}, barLine)
	x += 1 + barGap

	label, dot, fg, bg, bd := m.status()
	x = pill(hdc, x, mid, label, dot, fg, bg, bd, 0)
	if m.loggedIn && m.sid != kNoSession {
		pill(hdc, x+barPillGap, mid, i18n.T("Child session")+" "+win32.Itoa(int(m.sid)),
			0, barTxLo, barChip, barChipBd, glyphPerson)
	}

	btns := m.buttons(hdc, width)
	for _, b := range btns {
		bg := b.bg
		if b.cmd == m.hover {
			bg = b.hi
		}
		fillRoundBorder(hdc, b.rect, bg, b.bd, 7)
		at := b.rect.Left + barBtnPad
		barGlyph(hdc, b.glyph, win.RECT{Left: at, Top: b.rect.Top, Right: at + barBtnIcon, Bottom: b.rect.Bottom + 1},
			b.fg, 11)
		barText(hdc, b.label, &win.RECT{Left: at + barBtnIcon + barIconGap, Top: b.rect.Top,
			Right: b.rect.Right - barBtnPad + 2, Bottom: b.rect.Bottom},
			b.fg, barFont(12, win.FW_NORMAL), win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE)
	}
	return btns
}

// drawBar paints the bar; it runs inside WM_PAINT, where the control does not
// paint (it starts below the bar).
func (s *hostState) drawBar(hdc win.HDC) {
	if s.barHeight() == 0 {
		return
	}
	width, _ := clientDesktopSize(s.hwnd)
	m := s.barModel()
	if m.loggedIn {
		m.sid = getChildSessionID()
	}
	m.paint(hdc, width)
}

// barHit is the command of the button at x, y, or 0 for the bar itself.
func (s *hostState) barHit(x, y int32) uintptr {
	width, _ := clientDesktopSize(s.hwnd)
	hdc := win.GetDC(s.hwnd)
	if hdc == 0 {
		return 0
	}
	btns := s.barModel().buttons(hdc, width)
	win.ReleaseDC(s.hwnd, hdc)
	for _, b := range btns {
		if x >= b.rect.Left && x < b.rect.Right && y >= b.rect.Top && y < b.rect.Bottom {
			return b.cmd
		}
	}
	return 0
}

// barHover is the button under the pointer, for the highlight; it repaints
// the bar when that changed.
func (s *hostState) barHover(x, y int32) {
	if s.barHeight() == 0 {
		return
	}
	cmd := uintptr(0)
	if y >= 0 && y < kBarH {
		cmd = s.barHit(x, y)
	}
	if cmd == s.barHoverCmd {
		return
	}
	s.barHoverCmd = cmd
	s.invalidateBar()
}

// barClick handles a click at x, y: it says whether the bar took it.
func (s *hostState) barClick(x, y int32) bool {
	height := s.barHeight()
	if height == 0 || y < 0 || y >= height {
		return false
	}
	if cmd := s.barHit(x, y); cmd != 0 {
		s.command(cmd)
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
