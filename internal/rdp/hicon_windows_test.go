package rdp

import (
	"image"
	"image/color"
	"testing"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/internal/i18n"
)

// sessionTitle 少了会话号，任务栏上两个按钮就都叫 LocalRDP。
func TestSessionTitleCarriesTheSessionID(t *testing.T) {
	got := sessionTitle("LocalRDP", 48)
	if want := "LocalRDP · " + i18n.T("Child session") + " 48"; got != want {
		t.Fatalf("sessionTitle = %q, want %q", got, want)
	}
	if got := sessionTitle("  ", 48); got != "LocalRDP · "+i18n.T("Child session")+" 48" {
		t.Fatalf("sessionTitle of a blank title = %q", got)
	}
	if got := sessionTitle("LocalRDP", 0); got != "LocalRDP" {
		t.Fatalf("sessionTitle without a session = %q, want the plain title", got)
	}
}

// 会话图标画回去还是那张图：右下角是琥珀色的点，四角是透明的。这条守的是
// PNG→HICON 那步（预乘 alpha、行序、掩码），错了任务栏上就是个灰方块。
func TestSessionIconRoundTrips(t *testing.T) {
	const size = 32
	h := sessionIcon(size)
	if h == 0 {
		t.Fatal("sessionIcon(32) = 0: 会话窗口会退回 exe 里那张，两个窗口又长一样")
	}
	bmp := drawIcon(h, size)
	if c := bmp.NRGBAAt(size*3/4, size*3/4); !amberish(c) {
		t.Fatalf("右下角是 %v，该是琥珀色：PNG→HICON 这步画歪了", c)
	}
	if c := bmp.NRGBAAt(0, 0); c.A > 8 {
		t.Fatalf("左上角 alpha = %d，圆角外面该是透明的", c.A)
	}
	if c := bmp.NRGBAAt(size/2, size/3); c.R < 0x80 || c.B < 0x80 {
		t.Fatalf("中间是 %v，该是蓝色底或白色文件夹", c)
	}
}

// amberish 是"偏琥珀"的判据：红高、蓝低。
func amberish(c color.NRGBA) bool { return c.R >= 0x80 && c.B <= 0x80 }

// drawIcon 把图标画进一张 32 位 DIB 再读回来。像素是预乘 alpha 的 BGRA，
// alpha 满的地方两套表示一样，测试只比这些。
func drawIcon(h win.HICON, size int32) *image.NRGBA {
	bi := win.BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
		BiWidth:       size,
		BiHeight:      -size,
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: win.BI_RGB,
	}
	var bits unsafe.Pointer
	bmp := win.CreateDIBSection(0, &bi, win.DIB_RGB_COLORS, &bits, 0, 0)
	if bmp == 0 || bits == nil {
		return image.NewNRGBA(image.Rect(0, 0, int(size), int(size)))
	}
	defer win.DeleteObject(win.HGDIOBJ(bmp))
	dc := win.CreateCompatibleDC(0)
	defer win.DeleteDC(dc)
	win.SelectObject(dc, win.HGDIOBJ(bmp))
	win.DrawIconEx(dc, 0, 0, h, size, size, 0, 0, win.DI_NORMAL)

	src := unsafe.Slice((*byte)(bits), int(size*size*4))
	out := image.NewNRGBA(image.Rect(0, 0, int(size), int(size)))
	for i := 0; i < int(size*size); i++ {
		out.SetNRGBA(i%int(size), i/int(size),
			color.NRGBA{R: src[i*4+2], G: src[i*4+1], B: src[i*4], A: src[i*4+3]})
	}
	return out
}
