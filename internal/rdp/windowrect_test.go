package rdp

import (
	"testing"

	"localrdp/internal/store"
)

// 放得下的请求原样返回：宽取偶，高加上顶栏，水平居中。
//
// 请求取 641×480：CI 的 1024×768 工作区放得下，屏幕更小时直接跳过——
// 把“屏幕小”当成失败，只会让自己在 CI 上红着。
func TestDefaultWindowRectHonoursAFittingSize(t *testing.T) {
	wa := primaryWorkArea()
	areaW, areaH := int(wa.Right-wa.Left), int(wa.Bottom-wa.Top)
	if areaW < 700 || areaH < 600 {
		t.Skipf("the work area is %dx%d, too small for a 641x480 window", areaW, areaH)
	}
	opt := &StartOptions{Width: 641, Height: 480, X: cwUseDefault, Y: cwUseDefault}
	x, y, cw, ch, clamped := defaultWindowRect(opt, 96)
	if clamped {
		t.Fatalf("clamped = true for a 641x480 window in a %dx%d work area", areaW, areaH)
	}
	if cw != 640 {
		t.Fatalf("width = %d, want 640 (even)", cw)
	}
	if ch != 480+newBarMetrics(96).h {
		t.Fatalf("height = %d, want %d (480 plus the bar)", ch, 480+newBarMetrics(96).h)
	}
	wantX := int32(int(wa.Left) + (areaW-640)/2)
	if x != wantX || y != int32(int(wa.Top)+(areaH-int(ch))/2) {
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

// 子会话的桌面缩放默认跟着窗口所在的屏：150% 的屏上是 150% 的会话（桌面像素就是窗口
// 像素，没有第二次拉伸）。拉伸档只认 --scale——它的画面本来就要被 SmartSizing 缩一次；
// 勾了「禁用缩放自动继承」且没写 --scale 时钉成 100。
func TestDesktopScaleFollowsTheScreenUnlessAsked(t *testing.T) {
	cases := []struct {
		name string
		opt  StartOptions
		mode string
		dpi  uint32
		want int32
	}{
		{"the screen's dpi by default", StartOptions{}, store.SizeFit, 144, 150},
		{"no dpi at all means 100%", StartOptions{}, store.SizeFit, 0, 100},
		{"a 120 dpi screen is 125%", StartOptions{}, store.SizeFit, 120, 125},
		{"fixed follows the screen too", StartOptions{}, store.SizeFixed, 192, 200},
		{"no inherited scaling keeps 100%", StartOptions{NoScaleInherit: true}, store.SizeFit, 144, 100},
		{"stretch takes --scale", StartOptions{Scale: 175}, store.SizeScale, 144, 175},
		{"stretch without a scale is 100%", StartOptions{}, store.SizeScale, 144, 100},
		{"an explicit scale wins over the option", StartOptions{NoScaleInherit: true, Scale: 150}, store.SizeScale, 144, 150},
	}
	for _, c := range cases {
		if got := desktopScale(c.opt, c.mode, c.dpi); got != c.want {
			t.Errorf("%s: desktopScale = %d, want %d", c.name, got, c.want)
		}
	}
}

// 物理尺寸与实际缩放因子必须说同一件事：150% 就是 144 dpi，不然客人按一个数排版、
// 按另一个数画。
func TestLayoutDPIMatchesTheScale(t *testing.T) {
	for _, c := range []struct{ ds, want int32 }{{100, 96}, {125, 120}, {150, 144}, {200, 192}} {
		if got := layoutDPI(c.ds); got != c.want {
			t.Errorf("layoutDPI(%d) = %d, want %d", c.ds, got, c.want)
		}
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
