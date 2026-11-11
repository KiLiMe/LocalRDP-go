package rdp

// 会话窗口的图标：exe 里那张（mygo 从 winres\localrdp.png 烤进去的图标组），
// 右下角多一个琥珀色点。一个 exe 只能带一组图标，所以这一张以字节窜的形式
// 随程序走，运行时缩成窗口要的边长再做成 HICON。

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/winres"
)

var (
	sessionIconOnce sync.Once
	sessionIconImg  image.Image
	sessionIconMu   sync.Mutex
	sessionIcons    = map[int32]win.HICON{}
)

// sessionIcon 返回边长 size 的会话窗口图标；取不到返回 0，调用方退回 exe 资源
// 里的那张（两个窗口图标又会一样，但窗口不能没有图标）。
//
// 同一个 size 只做一次，HICON 是进程内的共享句柄，窗口销毁时不要删它。
func sessionIcon(size int32) win.HICON {
	if size <= 0 {
		size = 32
	}
	sessionIconOnce.Do(func() {
		img, err := png.Decode(bytes.NewReader(winres.SessionIcon))
		if err != nil {
			logf("会话版图标解不开（%v），改用 exe 里的那张", err)
			return
		}
		sessionIconImg = img
	})
	if sessionIconImg == nil {
		return 0
	}
	sessionIconMu.Lock()
	defer sessionIconMu.Unlock()
	if h, ok := sessionIcons[size]; ok {
		return h
	}
	h := iconFromImage(boxScale(sessionIconImg, int(size)))
	sessionIcons[size] = h
	return h
}

// iconFromImage 把一张正方形图片做成 32 位 HICON。像素按预乘 alpha 填：图标的
// 颜色位图就是这么存的，填错的话半透明的边缘会发白。
func iconFromImage(img *image.NRGBA) win.HICON {
	n := int32(img.Bounds().Dx())
	if n <= 0 || img.Bounds().Dy() != int(n) {
		return 0
	}
	bi := win.BITMAPINFOHEADER{
		BiSize:        uint32(unsafe.Sizeof(win.BITMAPINFOHEADER{})),
		BiWidth:       n,
		BiHeight:      -n, // 负数 = 自上而下，和 image 的行序一致
		BiPlanes:      1,
		BiBitCount:    32,
		BiCompression: win.BI_RGB,
	}
	var bits unsafe.Pointer
	bmp := win.CreateDIBSection(0, &bi, win.DIB_RGB_COLORS, &bits, 0, 0)
	if bmp == 0 || bits == nil {
		return 0
	}
	defer win.DeleteObject(win.HGDIOBJ(bmp))

	px := unsafe.Slice((*byte)(bits), int(n*n*4))
	for y := int32(0); y < n; y++ {
		for x := int32(0); x < n; x++ {
			c := img.NRGBAAt(int(x), int(y))
			a := uint32(c.A)
			i := (y*n + x) * 4
			px[i+0] = uint8(uint32(c.B) * a / 255)
			px[i+1] = uint8(uint32(c.G) * a / 255)
			px[i+2] = uint8(uint32(c.R) * a / 255)
			px[i+3] = uint8(a)
		}
	}
	// 32 位图标不看掩码，但 CreateIconIndirect 要求它有一张。
	mask := win.CreateBitmap(n, n, 1, 1, nil)
	if mask == 0 {
		return 0
	}
	defer win.DeleteObject(win.HGDIOBJ(mask))
	return win.CreateIconIndirect(&win.ICONINFO{FIcon: 1, HbmMask: mask, HbmColor: bmp})
}

// boxScale 把 src 按面积平均缩到 n×n。任务栏要的是 16/24/32，直接从 256 让系统
// 缩会糊，自己平均一遍边界才干净。
func boxScale(src image.Image, n int) *image.NRGBA {
	if n <= 0 {
		return nil
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	dst := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		y0, y1 := b.Min.Y+y*h/n, b.Min.Y+(y+1)*h/n
		for x := 0; x < n; x++ {
			x0, x1 := b.Min.X+x*w/n, b.Min.X+(x+1)*w/n
			var r, g, bl, a, cnt float64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, ca := src.At(sx, sy).RGBA()
					r += float64(cr >> 8)
					g += float64(cg >> 8)
					bl += float64(cb >> 8)
					a += float64(ca >> 8)
					cnt++
				}
			}
			if cnt == 0 {
				continue
			}
			dst.SetNRGBA(x, y, color.NRGBA{R: uint8(r / cnt), G: uint8(g / cnt), B: uint8(bl / cnt), A: uint8(a / cnt)})
		}
	}
	return dst
}
