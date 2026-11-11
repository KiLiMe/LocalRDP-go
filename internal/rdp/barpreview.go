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
//	go run -tags barpreview ./tools/barpreview
func PreviewBar(path string, width, dpi int32, rows []barModel) error {
	m := newBarMetrics(uint32(dpi))
	height := m.h
	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)*len(rows)))
	for i, bm := range rows {
		bm.m = m
		row, err := renderDIB(width, height, func(hdc win.HDC) { bm.paint(hdc, width, height) })
		if err != nil {
			return err
		}
		blit(img, row, 0, int(height)*i)
	}
	return savePNG(path, img)
}

// PreviewOverlay renders the floating bar, one row per hovered button: 0 for
// none, then the three commands, in the order the bar lays them out.
func PreviewOverlay(path string, dpi int32, hovers []uintptr) error {
	m := newBarMetrics(uint32(dpi))
	width, height := overlaySize(m)
	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)*len(hovers)))
	for i, h := range hovers {
		s := &hostState{overlayHover: h, m: m}
		row, err := renderDIB(width, height, func(hdc win.HDC) { s.drawOverlay(hdc) })
		if err != nil {
			return err
		}
		blit(img, row, 0, int(height)*i)
	}
	return savePNG(path, img)
}

// renderDIB draws into a 32-bit section and reads the pixels back.
func renderDIB(width, height int32, draw func(hdc win.HDC)) (*image.NRGBA, error) {
	screen := win.GetDC(0)
	defer win.ReleaseDC(0, screen)
	mem := win.CreateCompatibleDC(screen)
	defer win.DeleteDC(mem)

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
		return nil, os.ErrInvalid
	}
	old := win.SelectObject(mem, win.HGDIOBJ(bm))
	draw(mem)
	win.GdiFlush()

	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	src := unsafe.Slice((*byte)(bits), int(width)*int(height)*4)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			p := (y*int(width) + x) * 4
			img.Set(x, y, color.NRGBA{R: src[p+2], G: src[p+1], B: src[p], A: 255})
		}
	}
	win.SelectObject(mem, old)
	win.DeleteObject(win.HGDIOBJ(bm))
	return img, nil
}

func blit(dst, src *image.NRGBA, x, y int) {
	for sy := 0; sy < src.Rect.Dy(); sy++ {
		for sx := 0; sx < src.Rect.Dx(); sx++ {
			dst.Set(x+sx, y+sy, src.NRGBAAt(sx, sy))
		}
	}
}

func savePNG(path string, img *image.NRGBA) error {
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
func PreviewBarFile(path string, width, dpi int32, rows []BarModelForPreview) error {
	ms := make([]barModel, len(rows))
	for i, r := range rows {
		sid := r.Sid
		if sid == 0 {
			sid = kNoSession
		}
		ms[i] = barModel{loggedIn: r.LoggedIn, everConnected: r.EverConnected, sid: sid, hover: r.Hover}
	}
	return PreviewBar(path, width, dpi, ms)
}

// PreviewOverlayFile is PreviewOverlay, for a caller outside this package.
func PreviewOverlayFile(path string, dpi int32, hovers []uintptr) error {
	return PreviewOverlay(path, dpi, hovers)
}
