package rdp

import "testing"

// 放得下的请求原样返回：宽取偶，高加上顶栏，水平居中。
func TestDefaultWindowRectHonoursAFittingSize(t *testing.T) {
	wa := primaryWorkArea()
	opt := &StartOptions{Width: 1201, Height: 800, X: cwUseDefault, Y: cwUseDefault}
	x, y, cw, ch, clamped := defaultWindowRect(opt, 96)
	if clamped {
		t.Fatalf("clamped = true for a 1201x800 window in the work area")
	}
	if cw != 1200 {
		t.Fatalf("width = %d, want 1200 (even)", cw)
	}
	if ch != 800+newBarMetrics(96).h {
		t.Fatalf("height = %d, want %d (800 plus the bar)", ch, 800+newBarMetrics(96).h)
	}
	wantX := int32(int(wa.Left) + (int(wa.Right-wa.Left)-1200)/2)
	if x != wantX || y != int32(int(wa.Top)+(int(wa.Bottom-wa.Top)-int(ch))/2) {
		t.Fatalf("position = (%d,%d), want it centered", x, y)
	}
}

// 比工作区还大的请求要截断并说出来：不截断的话窗口居中到负坐标，顶栏跟着跑出屏幕。
func TestDefaultWindowRectFitsTheWorkArea(t *testing.T) {
	wa := primaryWorkArea()
	areaW := int(wa.Right - wa.Left)
	areaH := int(wa.Bottom - wa.Top)
	opt := &StartOptions{Width: areaW * 2, Height: areaH * 2, X: cwUseDefault, Y: cwUseDefault}
	x, y, cw, ch, clamped := defaultWindowRect(opt, 96)
	if !clamped {
		t.Fatalf("clamped = false for a %dx%d request in a %dx%d work area", areaW*2, areaH*2, areaW, areaH)
	}
	if cw > int32(areaW) || ch > int32(areaH) {
		t.Fatalf("window = %dx%d, does not fit the %dx%d work area", cw, ch, areaW, areaH)
	}
	if cw%2 != 0 {
		t.Fatalf("width = %d, want an even one", cw)
	}
	if x < wa.Left || y < wa.Top || x+cw > wa.Right || y+ch > wa.Bottom {
		t.Fatalf("window at (%d,%d) %dx%d leaves the work area (%d,%d)-(%d,%d)", x, y, cw, ch, wa.Left, wa.Top, wa.Right, wa.Bottom)
	}
}

// 默认尺寸（不填宽高）本来就落在工作区里，不该被当成截断。
func TestDefaultWindowRectOfTheDefaultSizeIsNotClamped(t *testing.T) {
	if _, _, _, _, clamped := defaultWindowRect(&StartOptions{X: cwUseDefault, Y: cwUseDefault}, 96); clamped {
		t.Fatalf("clamped = true for the default window size")
	}
}

// 150% 的屏上，用户填的尺寸要换成 1.5 倍的像素：不换的话子会话桌面、顶栏、
// 关闭按钮都只有主机的三分之二大，看着像被缩过。
func TestDefaultWindowRectScalesForTheDisplayDPI(t *testing.T) {
	opt := &StartOptions{Width: 64, Height: 48, X: cwUseDefault, Y: cwUseDefault}
	_, _, cw, ch, clamped := defaultWindowRect(opt, 144)
	if clamped {
		t.Skipf("the work area is too small for a %dx%d window", 96, 72)
	}
	if cw != 96 {
		t.Fatalf("width = %d, want 96 (64 pixels at 150%%)", cw)
	}
	if want := int32(72) + newBarMetrics(144).h; ch != want {
		t.Fatalf("height = %d, want %d (48 at 150%% plus the bar)", ch, want)
	}
}

// 顶栏自己也得跟着 dpi 长：40 像素照搬到 150% 的屏上，关闭按钮就只有三分之二。
func TestBarMetricsFollowTheDPI(t *testing.T) {
	for _, c := range []struct {
		dpi     uint32
		h, capW int32
	}{{96, 40, 46}, {120, 50, 58}, {144, 60, 69}, {192, 80, 92}} {
		m := newBarMetrics(c.dpi)
		if m.h != c.h {
			t.Fatalf("dpi %d: bar height = %d, want %d", c.dpi, m.h, c.h)
		}
		if m.capW != c.capW {
			t.Fatalf("dpi %d: caption button width = %d, want %d", c.dpi, m.capW, c.capW)
		}
		if m.grip < 1 || m.gap < 1 {
			t.Fatalf("dpi %d: metrics lost their padding: %+v", c.dpi, m)
		}
	}
}

// 用户点明了位置就用他的，不重新居中。
func TestDefaultWindowRectKeepsAnExplicitPosition(t *testing.T) {
	x, y, _, _, _ := defaultWindowRect(&StartOptions{Width: 800, Height: 600, X: 10, Y: 20}, 96)
	if x != 10 || y != 20 {
		t.Fatalf("position = (%d,%d), want (10,20)", x, y)
	}
}
