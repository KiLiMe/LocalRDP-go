// genicon draws the icon of the window — the folder with the plus, in the
// accent color — and writes winres/localrdp.png, the picture `mygo build`
// bakes into the exe: mygo scales that PNG down to every size a Windows icon
// holds (cmd/mygo/icon.go in github.com/egoist/mygo).
//
//	go run ./tools/genicon
//
// The PNG is committed, and main_test.go fails when it no longer matches what
// draw renders: change the drawing, rerun the command above. No build tag
// hides the file, so that test can draw through the same code.
package main

import (
	"bytes"
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
)

// The view box the paths are drawn in, so they match internal/ui/icons.go.
const box = 24.0

// iconSize is the edge of the PNG, in pixels: 256 is the biggest size of a
// Windows icon, so mygo passes it through untouched and averages down from it
// for the smaller ones.
const iconSize = 256

// The colors of the icon: the window's accent behind, white on it.
var (
	accent = color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff}
	ink    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
	// badgeColor 是会话窗口右下角那个点：琥珀色，在蓝色底上、在深色与浅色
	// 任务栏上都认得出来。
	badgeColor = color.NRGBA{R: 0xff, G: 0xc2, B: 0x4b, A: 0xff}
)

// 那个点在 view box 里的位置与大小：中心靠右下角，半径 4.2，外面留一条
// 1.4 宽的蓝环把它与白色描边隔开。
var badgeAt = pt{17.6, 17.6}

const (
	badgeR    = 4.2
	badgeRing = 1.4
)

type pt struct{ x, y float64 }

func main() {
	out := flag.String("o", "winres/localrdp.png", "the PNG to write")
	flag.Parse()

	write(*out, render(iconSize, false))
	// 会话窗口的图标：同一张，右下角多一个点，好让任务栏上两个窗口分得开。
	write(sessionPath(*out), render(iconSize, true))
}

func sessionPath(main string) string {
	return strings.TrimSuffix(main, ".png") + "-session.png"
}

func write(path string, img image.Image) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		log.Fatalf("encode %dx%d: %v", iconSize, iconSize, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s: %dx%d, %d bytes", path, iconSize, iconSize, buf.Len())
}

// draw renders the icon at n pixels a side, antialiased by sampling each
// pixel 4 by 4.
func draw(n int) image.Image { return render(n, false) }

// render draws the icon; badge adds the amber dot in the bottom right corner,
// which is what tells the child session window apart in the taskbar.
func render(n int, badge bool) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	scale := float64(n) / box
	glyph := glyphPath()
	half := 0.72 * scale // the stroke of the paths is 2 of the view box, scaled

	const sub = 4
	step := 1.0 / sub
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var bg, fg, dot float64
			for sy := 0; sy < sub; sy++ {
				for sx := 0; sx < sub; sx++ {
					px := (float64(x) + (float64(sx)+0.5)*step - 0.5) / scale
					py := (float64(y) + (float64(sy)+0.5)*step - 0.5) / scale
					p := pt{px, py}
					if !inRoundRect(p) {
						continue
					}
					bg++
					if badge {
						switch d := math.Hypot(p.x-badgeAt.x, p.y-badgeAt.y); {
						case d <= badgeR:
							dot++
							continue
						case d <= badgeR+badgeRing:
							// 蓝环：不记成描边，直接算底色。
							continue
						}
					}
					if nearPath(p, glyph, half/scale) {
						fg++
					}
				}
			}
			total := float64(sub * sub)
			if bg == 0 {
				continue
			}
			a := bg / total
			// 三类子样本各自占的比重：底色（含下面那条蓝环，所以两者合并）、
			// 白色描边、橙色的点。fg 是 bg 的子集，别再算一遍。
			inkW := fg / bg
			dotW := dot / bg
			c := mix(accent, ink, math.Min(1, inkW*(1-dotW)))
			c = mix(c, badgeColor, math.Min(1, dotW))
			c.A = uint8(math.Round(a * 255))
			img.SetNRGBA(x, y, c)
		}
	}
	return img
}

// mix blends a toward b by w.
func mix(a, b color.NRGBA, w float64) color.NRGBA {
	blend := func(x, y uint8) uint8 {
		return uint8(math.Round(float64(x)*(1-w) + float64(y)*w))
	}
	return color.NRGBA{R: blend(a.R, b.R), G: blend(a.G, b.G), B: blend(a.B, b.B), A: 0xff}
}

// inRoundRect is whether p is in the rounded square the icon sits on.
func inRoundRect(p pt) bool {
	const inset, radius = 0.6, 5.2
	lo, hi := inset+radius, box-inset-radius
	cx := math.Min(math.Max(p.x, lo), hi)
	cy := math.Min(math.Max(p.y, lo), hi)
	return math.Hypot(p.x-cx, p.y-cy) <= radius
}

// nearPath is whether p is within half of the stroked path.
func nearPath(p pt, path []pt, half float64) bool {
	for i := 1; i < len(path); i++ {
		if segDist(p, path[i-1], path[i]) <= half {
			return true
		}
	}
	return false
}

// segDist is the distance from p to the segment ab.
func segDist(p, a, b pt) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	l := dx*dx + dy*dy
	t := 0.0
	if l > 0 {
		t = math.Max(0, math.Min(1, ((p.x-a.x)*dx+(p.y-a.y)*dy)/l))
	}
	return math.Hypot(p.x-(a.x+t*dx), p.y-(a.y+t*dy))
}

// glyphPath is the folder with the plus of internal/ui/icons.go, its corners
// flattened, scaled down to sit inside the rounded square.
func glyphPath() []pt {
	raw := []pt{}
	raw = append(raw, arc(pt{5.5, 5.5}, 2.5, math.Pi, 1.5*math.Pi)...)
	raw = append(raw, pt{9.5, 3}, pt{12, 5.5}, pt{18.5, 5.5})
	raw = append(raw, arc(pt{18.5, 8}, 2.5, 1.5*math.Pi, 2*math.Pi)...)
	raw = append(raw, pt{21, 18})
	raw = append(raw, arc(pt{18.5, 18}, 2.5, 0, 0.5*math.Pi)...)
	raw = append(raw, pt{5.5, 20.5})
	raw = append(raw, arc(pt{5.5, 18}, 2.5, 0.5*math.Pi, math.Pi)...)
	raw = append(raw, pt{3, 5.5})
	raw = append(raw, pt{8, 12}, pt{16, 12}, pt{12, 12}, pt{12, 9}, pt{12, 12}, pt{12, 15})

	// Scale about the middle, so the strokes keep a little air inside the
	// rounded square.
	const k = 0.72
	out := make([]pt, len(raw))
	for i, p := range raw {
		out[i] = pt{box/2 + (p.x-box/2)*k, box/2 + (p.y-box/2)*k}
	}
	return out
}

// arc is the points of a circle from angle a to b, its ends included.
func arc(c pt, r, a, b float64) []pt {
	const steps = 10
	out := make([]pt, 0, steps+1)
	for i := 0; i <= steps; i++ {
		t := a + (b-a)*float64(i)/steps
		out = append(out, pt{c.x + r*math.Cos(t), c.y + r*math.Sin(t)})
	}
	return out
}
