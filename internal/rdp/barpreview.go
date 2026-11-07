//go:build barpreview

package rdp

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"unsafe"

	"github.com/lxn/win"
)

// PreviewBar renders the bar into a PNG, one row per model: a way to look at
// the bar without connecting to anything.
//
//	go run -tags barpreview ./tmp_bar
func PreviewBar(path string, width int32, rows []barModel) error {
	const height = int32(kBarH)
	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)*len(rows)))

	screen := win.GetDC(0)
	defer win.ReleaseDC(0, screen)

	for i, m := range rows {
		mem := win.CreateCompatibleDC(screen)
		var bits unsafe.Pointer
		bi := win.BITMAPINFOHEADER{
			BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
			BiWidth:       width,
			BiHeight:      -height,
			BiPlanes:      1,
			BiBitCount:    32,
			BiCompression: win.BI_RGB,
		}
		bm := win.CreateDIBSection(mem, &bi, win.DIB_RGB_COLORS, &bits, 0, 0)
		if bm == 0 || bits == nil {
			return os.ErrInvalid
		}
		old := win.SelectObject(mem, win.HGDIOBJ(bm))
		m.paint(mem, width)
		win.GdiFlush()

		src := unsafe.Slice((*byte)(bits), int(width)*int(height)*4)
		for y := 0; y < int(height); y++ {
			for x := 0; x < int(width); x++ {
				p := (y*int(width) + x) * 4
				img.Set(x, y+int(height)*i, color.NRGBA{R: src[p+2], G: src[p+1], B: src[p], A: 255})
			}
		}
		win.SelectObject(mem, old)
		win.DeleteObject(win.HGDIOBJ(bm))
		win.DeleteDC(mem)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// BarModelForPreview is one row of the preview: the bar's state, spelled out
// for a program outside this package.
type BarModelForPreview struct {
	LoggedIn      bool
	EverConnected bool
	Sid           uint32
	Hover         uintptr
}

// PreviewBarFile is PreviewBar for the model a caller outside this package
// can name.
func PreviewBarFile(path string, width int32, rows []BarModelForPreview) error {
	ms := make([]barModel, len(rows))
	for i, r := range rows {
		sid := r.Sid
		if sid == 0 {
			sid = kNoSession
		}
		ms[i] = barModel{loggedIn: r.LoggedIn, everConnected: r.EverConnected, sid: sid, hover: r.Hover}
	}
	return PreviewBar(path, width, ms)
}
