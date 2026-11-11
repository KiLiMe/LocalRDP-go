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
//
// The numbers below are the bar as it looks at 96 dpi: newBarMetrics turns
// them into pixels for the dpi the window is on. Without that, a 150% screen
// gets a bar two thirds of the size Windows draws its own windows with.
const (
	barBaseH = 40
	// barBaseGrip is the strip of window around the picture that resizes it:
	// the window has no frame of its own, so the ring has to be ours.
	barBaseGrip      = 4
	barBasePad       = 12
	barBaseGap       = 10
	barBasePillH     = 22
	barBasePillGap   = 6
	barBasePillPad   = 9
	barBaseDot       = 6
	barBasePillGlyph = 11
	barBaseBtnH      = 26
	barBaseBtnGap    = 6
	barBaseBtnPad    = 10
	barBaseBtnIcon   = 12
	barBaseIconGap   = 5
	// barBaseCapW is the width of one of the three caption buttons at the
	// right edge, as wide as Windows draws them.
	barBaseCapW = 46

	// The mark: the accent square with the folder the window wears.
	barBaseMark   = 16
	barBaseMarkR  = 5
	barBaseRadius = 7
	// barBaseSeg is the half height of a separator hairline.
	barBaseSeg = 8

	// Font heights, in pixels at 96 dpi.
	barBaseFont     = 12
	barBaseFontName = 13
	barBaseGlyph    = 10
	barBaseBtnGlyph = 11

	// wmRdpCommand asks the host window to act, from the interface or from
	// another process (see win32.PostCommand).
	wmRdpCommand  = win32.HostCommandMessage
	cmdReconnect  = win32.CmdReconnect
	cmdDisconnect = win32.CmdDisconnect
	cmdFullScreen = win32.CmdFullScreen

	// The caption buttons are drawn here but run by Windows: the window
	// reports them as non-client, and DefWindowProc turns a click into
	// SC_MINIMIZE / SC_MAXIMIZE / SC_CLOSE.
	cmdMinimize = 100
	cmdMaximize = 101
	cmdClose    = 102
)

// Glyphs of Segoe MDL2 Assets, the icons Windows itself uses.
const (
	glyphFullScreen = 0xE740
	glyphRefresh    = 0xE72C
	glyphPower      = 0xE7E8
	glyphPerson     = 0xE77B
	glyphFolder     = 0xE8F4 // NewFolder: the mark the window wears
	glyphMinimize   = 0xE921
	glyphMaximize   = 0xE922
	glyphRestore    = 0xE923
	glyphClose      = 0xE8BB
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
	barCapHi   = barColor(38, 44, 56)
	barCapRed  = barColor(196, 43, 28)
)

var (
	procCreateSolidBrush = gdi32.NewProc("CreateSolidBrush")
	procCreatePen        = gdi32.NewProc("CreatePen")
	procCreateFontW      = gdi32.NewProc("CreateFontW")
	procFillRect         = user32.NewProc("FillRect")
	procDrawTextW        = user32.NewProc("DrawTextW")
	procTextExtent       = gdi32.NewProc("GetTextExtentPoint32W")
)

// barMetrics is the bar's numbers for one dpi, in pixels.
type barMetrics struct {
	dpi                     uint32
	h, grip                 int32
	pad, gap                int32
	pillH, pillGap, pillPad int32
	dot, pillGlyph          int32
	btnH, btnGap, btnPad    int32
	btnIcon, iconGap        int32
	capW                    int32
	mark, markR, radius     int32
	seg                     int32
	font, fontName          int32
	glyph, btnGlyph         int32
}

// newBarMetrics scales the 96 dpi numbers for dpi. They round to the nearest
// pixel and never drop below 1: a bar that loses its padding is worse than one
// that is a pixel off.
func newBarMetrics(dpi uint32) barMetrics {
	if dpi == 0 {
		dpi = 96
	}
	px := func(v int32) int32 {
		s := int32((int64(v)*int64(dpi) + 48) / 96)
		if s < 1 {
			s = 1
		}
		return s
	}
	return barMetrics{
		dpi:  dpi,
		h:    px(barBaseH),
		grip: px(barBaseGrip),

		pad:     px(barBasePad),
		gap:     px(barBaseGap),
		pillH:   px(barBasePillH),
		pillGap: px(barBasePillGap),
		pillPad: px(barBasePillPad),
		dot:     px(barBaseDot),

		pillGlyph: px(barBasePillGlyph),
		btnH:      px(barBaseBtnH),
		btnGap:    px(barBaseBtnGap),
		btnPad:    px(barBaseBtnPad),
		btnIcon:   px(barBaseBtnIcon),
		iconGap:   px(barBaseIconGap),
		capW:      px(barBaseCapW),

		mark:   px(barBaseMark),
		markR:  px(barBaseMarkR),
		radius: px(barBaseRadius),
		seg:    px(barBaseSeg),

		font:     px(barBaseFont),
		fontName: px(barBaseFontName),
		glyph:    px(barBaseGlyph),
		btnGlyph: px(barBaseBtnGlyph),
	}
}

// px scales a one off number that is not worth a field of its own.
func (m barMetrics) px(v int32) int32 {
	s := int32((int64(v)*int64(m.dpi) + 48) / 96)
	if s < 1 {
		s = 1
	}
	return s
}

// barHeightFor is the bar the window keeps above the control; full screen
// has none, the picture takes the whole monitor.
func barHeightFor(opt *StartOptions, dpi uint32) int32 {
	if opt != nil && opt.Fullscreen {
		return 0
	}
	return newBarMetrics(dpi).h
}

func (s *hostState) barHeight() int32 {
	if s.fullscreen {
		return 0
	}
	return s.m.h
}

// setDPI takes new numbers when the window moves to a screen with another dpi,
// and lays the picture out again around the taller bar.
func (s *hostState) setDPI(dpi uint32) {
	m := newBarMetrics(dpi)
	if m == s.m {
		return
	}
	s.m = m
	if s.hwnd != 0 {
		w, h := clientDesktopSize(s.hwnd)
		s.placeView(w, h)
	}
	s.invalidateBar()
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
	m             barMetrics
	loggedIn      bool
	everConnected bool
	zoomed        bool
	sid           uint32
	hover         uintptr
}

// barModel costs nothing: the session id is only asked for when painting.
func (s *hostState) barModel() barModel {
	return barModel{
		m:             s.m,
		loggedIn:      s.loggedIn,
		everConnected: s.everConnected,
		zoomed:        s.hwnd != 0 && win.IsZoomed(s.hwnd),
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
	y := (m.m.h - m.m.btnH) / 2
	x := width - m.m.pad
	out := make([]barButton, 0, len(defs))
	for _, d := range defs {
		w := 2*m.m.btnPad + m.m.btnIcon + m.m.iconGap + barWidth(hdc, d.label, barFont(m.m.font, win.FW_NORMAL))
		x -= w
		d.rect = win.RECT{Left: x, Top: y, Right: x + w, Bottom: y + m.m.btnH}
		out = append(out, d)
		x -= m.m.btnGap
	}
	return out
}

// pill draws a rounded label and gives back the x the next one starts at.
func pill(hdc win.HDC, m barMetrics, x, mid int32, label string, dot, fg, bg, bd uint32, glyph uint32) int32 {
	pad, dotW, glyphW := m.pillPad, m.dot, m.pillGlyph
	font := barFont(m.font, win.FW_NORMAL)
	w := 2*pad + barWidth(hdc, label, barFont(m.font, win.FW_NORMAL))
	if dot != 0 {
		w += dotW + m.px(6)
	}
	if glyph != 0 {
		w += glyphW + m.iconGap
	}
	rc := win.RECT{Left: x, Top: mid - m.pillH/2, Right: x + w, Bottom: mid + (m.pillH+1)/2}
	fillRoundBorder(hdc, rc, bg, bd, m.pillH/2)
	at := x + pad
	if dot != 0 {
		fillRoundBorder(hdc, win.RECT{Left: at, Top: mid - dotW/2, Right: at + dotW, Bottom: mid + dotW/2},
			dot, dot, dotW/2)
		at += dotW + m.px(6)
	}
	if glyph != 0 {
		barGlyph(hdc, glyph, win.RECT{Left: at - 1, Top: rc.Top, Right: at + glyphW, Bottom: rc.Bottom}, fg, m.glyph)
		at += glyphW + m.iconGap
	}
	barText(hdc, label, &win.RECT{Left: at, Top: rc.Top, Right: rc.Right - pad + 2, Bottom: rc.Bottom},
		fg, font, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE)
	return rc.Right
}

// captionButtons lays the three window buttons out at the right edge, on the
// same line as the bar: they are non-client, so Windows runs them.
func captionButtons(m barMetrics, hover uintptr, zoomed bool, width int32) []barButton {
	maxGlyph := uint32(glyphMaximize)
	if zoomed {
		maxGlyph = glyphRestore
	}
	defs := []barButton{
		{cmd: cmdClose, glyph: glyphClose, fg: barTxHi, hi: barCapRed},
		{cmd: cmdMaximize, glyph: maxGlyph, fg: barTxHi, hi: barCapHi},
		{cmd: cmdMinimize, glyph: glyphMinimize, fg: barTxHi, hi: barCapHi},
	}
	x := width
	out := make([]barButton, 0, len(defs))
	for _, d := range defs {
		x -= m.capW
		d.rect = win.RECT{Left: x, Top: 0, Right: x + m.capW, Bottom: m.h}
		out = append(out, d)
	}
	return out
}

// paint draws the whole bar and gives back the action buttons, for hit
// testing. The area below the bar is painted too: the window has no frame,
// and the ring the mouse resizes it with is this color.
func (m barModel) paint(hdc win.HDC, width, height int32) []barButton {
	fillSolid(hdc, win.RECT{Right: width, Bottom: height}, barBg)
	fillSolid(hdc, win.RECT{Top: m.m.h - 1, Right: width, Bottom: m.m.h}, barLine)

	mid := m.m.h / 2
	x := m.m.pad

	// The mark: the accent square with the folder the window wears.
	half := m.m.mark / 2
	fillRound(hdc, win.RECT{Left: x, Top: mid - half, Right: x + m.m.mark, Bottom: mid + half}, barMark, m.m.markR)
	barGlyph(hdc, glyphFolder, win.RECT{Left: x, Top: mid - half, Right: x + m.m.mark, Bottom: mid + half + 1}, barWhite, m.m.glyph)
	x += m.m.mark + m.m.px(8)

	// The name.
	nameFont := barFont(m.m.fontName, win.FW_SEMIBOLD)
	nameW := barWidth(hdc, "LocalRDP", nameFont)
	barText(hdc, "LocalRDP", &win.RECT{Left: x, Top: 0, Right: x + nameW + 4, Bottom: m.m.h},
		barTxHi, nameFont, win.DT_LEFT|win.DT_VCENTER|win.DT_SINGLELINE)
	x += nameW + m.m.gap

	// A hairline, then what the session is doing.
	fillSolid(hdc, win.RECT{Left: x, Top: mid - m.m.seg, Right: x + 1, Bottom: mid + m.m.seg}, barLine)
	x += 1 + m.m.gap

	label, dot, fg, bg, bd := m.status()
	x = pill(hdc, m.m, x, mid, label, dot, fg, bg, bd, 0)
	if m.loggedIn && m.sid != kNoSession {
		pill(hdc, m.m, x+m.m.pillGap, mid, i18n.T("Child session")+" "+win32.Itoa(int(m.sid)),
			0, barTxLo, barChip, barChipBd, glyphPerson)
	}

	btns := m.buttons(hdc, width-3*m.m.capW-m.m.gap)
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

	// A hairline, then the three buttons Windows would have drawn itself.
	sepX := width - 3*m.m.capW - 1
	fillSolid(hdc, win.RECT{Left: sepX, Top: mid - m.m.seg, Right: sepX + 1, Bottom: mid + m.m.seg}, barLine)
	for _, b := range captionButtons(m.m, m.hover, m.zoomed, width) {
		fg := b.fg
		if b.cmd == m.hover {
			fillSolid(hdc, b.rect, b.hi)
			if b.cmd == cmdClose {
				fg = barWhite
			}
		}
		barGlyph(hdc, b.glyph, b.rect, fg, m.m.glyph)
	}
	return btns
}

// drawBar paints the bar; it runs inside WM_PAINT, where the control does not
// paint (it starts below the bar).
func (s *hostState) drawBar(hdc win.HDC) {
	if s.barHeight() == 0 {
		return
	}
	width, height := clientDesktopSize(s.hwnd)
	m := s.barModel()
	if m.loggedIn {
		m.sid = getChildSessionID()
	}
	m.paint(hdc, width, height)
}

// barHit is the command of the action button at x, y, or 0 for the bar itself.
func (s *hostState) barHit(x, y int32) uintptr {
	width, _ := clientDesktopSize(s.hwnd)
	hdc := win.GetDC(s.hwnd)
	if hdc == 0 {
		return 0
	}
	btns := s.barModel().buttons(hdc, width-3*s.m.capW-s.m.gap)
	win.ReleaseDC(s.hwnd, hdc)
	for _, b := range btns {
		if x >= b.rect.Left && x < b.rect.Right && y >= b.rect.Top && y < b.rect.Bottom {
			return b.cmd
		}
	}
	return 0
}

// captionHit is the caption button at x, y, or 0 for the rest of the bar.
func (s *hostState) captionHit(x, y int32) uintptr {
	width, _ := clientDesktopSize(s.hwnd)
	zoomed := s.hwnd != 0 && win.IsZoomed(s.hwnd)
	for _, b := range captionButtons(s.m, 0, zoomed, width) {
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
	if y >= 0 && y < s.m.h {
		cmd = s.captionHit(x, y)
		if cmd == 0 {
			cmd = s.barHit(x, y)
		}
	}
	if cmd == s.barHoverCmd {
		return
	}
	s.barHoverCmd = cmd
	s.invalidateBar()
}

// barHoverAtPointer repeats the highlight after a leave message, which comes
// with no coordinates of its own.
func (s *hostState) barHoverAtPointer() {
	var pt win.POINT
	if !win.GetCursorPos(&pt) || !win.ScreenToClient(s.hwnd, &pt) {
		return
	}
	s.barHover(pt.X, pt.Y)
}

// windowHit is what the window has at x, y of its client area: the edges
// resize it, the bar drags it, the buttons act.
func (s *hostState) windowHit(x, y int32) uintptr {
	w, h := clientDesktopSize(s.hwnd)
	if grip := s.grip(); grip > 0 && !win.IsZoomed(s.hwnd) {
		if code := gripHit(x, y, w, h, grip); code != 0 {
			return code
		}
	}
	if y < s.barHeight() {
		switch s.captionHit(x, y) {
		case cmdMinimize:
			return win.HTMINBUTTON
		case cmdMaximize:
			return win.HTMAXBUTTON
		case cmdClose:
			return win.HTCLOSE
		}
		if s.barHit(x, y) != 0 {
			return win.HTCLIENT
		}
		// The rest of the bar drags the window, double-clicks to maximize,
		// and opens the system menu on a right click: all of that is what
		// HTCAPTION means to Windows.
		return win.HTCAPTION
	}
	return win.HTCLIENT
}

// gripHit is the resize code for a point in the ring around the picture, or
// 0 when the point is not on it.
func gripHit(x, y, w, h, grip int32) uintptr {
	side := uintptr(0)
	switch {
	case x < grip:
		side = win.HTLEFT
	case x >= w-grip:
		side = win.HTRIGHT
	}
	switch {
	case y < grip:
		switch side {
		case win.HTLEFT:
			return win.HTTOPLEFT
		case win.HTRIGHT:
			return win.HTTOPRIGHT
		}
		return win.HTTOP
	case y >= h-grip:
		switch side {
		case win.HTLEFT:
			return win.HTBOTTOMLEFT
		case win.HTRIGHT:
			return win.HTBOTTOMRIGHT
		}
		return win.HTBOTTOM
	}
	return side
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
