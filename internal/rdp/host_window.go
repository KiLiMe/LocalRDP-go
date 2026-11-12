package rdp

import "github.com/lxn/win"

// requestClose asks for the window to close. It may arrive twice (the user
// clicked X, and the control then reported the disconnect); the WM_CLOSE
// handler does the actual work only once.
func (s *hostState) requestClose() {
	s.closing = true
	win.PostMessage(s.hwnd, win.WM_CLOSE, 0, 0)
}

// defaultWindowRect 按 C++ 的规则算客户区：工作区 5/6，宽取偶，居中。
//
// 用户填的宽高是 100% 下的尺寸：窗口在 150% 的屏上就开 1.5 倍的像素，
// 子会话桌面才和主机的窗口一样大——否则画面和顶栏都只有三分之二。
//
// 要的尺寸比工作区还大时按工作区截断：比屏幕还大的窗口有一半落在屏幕外，
// 顶栏（拖动、关闭都靠它）跟着看不见——高分辨率配小屏幕时就是这样。
func defaultWindowRect(opt *StartOptions, dpi uint32) (x, y, cw, ch int32, clamped bool) {
	wa := primaryWorkArea()
	w := int(wa.Right - wa.Left)
	h := int(wa.Bottom - wa.Top)
	sizeW := 800
	sizeH := 600
	if v := w * 5 / 6; v > sizeW {
		sizeW = v
	}
	if v := h * 5 / 6; v > sizeH {
		sizeH = v
	}
	if opt.Width > 0 {
		sizeW = dpiPx(opt.Width, dpi) &^ 1
	}
	if opt.Height > 0 {
		sizeH = dpiPx(opt.Height, dpi) + int(barHeightFor(opt, dpi))
	}
	if sizeW > w {
		sizeW, clamped = w&^1, true
	}
	if sizeH > h {
		sizeH, clamped = h, true
	}
	x = int32(int(wa.Left) + (w-sizeW)/2)
	y = int32(int(wa.Top) + (h-sizeH)/2)
	if opt.X != cwUseDefault {
		x = int32(opt.X)
	}
	if opt.Y != cwUseDefault {
		y = int32(opt.Y)
	}
	return x, y, int32(sizeW), int32(sizeH), clamped
}
