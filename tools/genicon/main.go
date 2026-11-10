//go:build ignore

// genicon draws the icon of the window — the folder with the plus, in the
// accent color — at every size an .ico wants, and writes winres/localrdp.ico.
//
//	go run tools/genicon/main.go
//
// go-winres turns that file into the resource object the linker embeds:
//
//	go-winres simply --icon winres/localrdp.ico --manifest none \
//	    --product-name LocalRDP --file-version 2026.11.7.1 --product-version 2026.11.7.1
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
)

// The view box the paths are drawn in, so they match internal/ui/icons.go.
const box = 24.0

// The sizes an .ico holds: the ones Windows asks for, small to large.
var sizes = []int{16, 20, 24, 32, 40, 48, 64, 128, 256}

// The colors of the icon: the window's accent behind, white on it.
var (
	accent = color.NRGBA{R: 0x25, G: 0x63, B: 0xeb, A: 0xff}
	ink    = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

type pt struct{ x, y float64 }

func main() {
	out := flag.String("o", "winres/localrdp.ico", "the .ico to write")
	flag.Parse()

	var ico bytes.Buffer
	ico.Write(make([]byte, 6)) // the header, filled in once the images are known
	binary.LittleEndian.PutUint16(ico.Bytes()[2:], 1)
	binary.LittleEndian.PutUint16(ico.Bytes()[4:], uint16(len(sizes)))

	blobs := make([][]byte, 0, len(sizes))
	for _, n := range sizes {
		var pngs bytes.Buffer
		if err := png.Encode(&pngs, draw(n)); err != nil {
			log.Fatalf("encode %d: %v", n, err)
		}
		blobs = append(blobs, pngs.Bytes())
	}

	offset := 6 + 16*len(sizes)
	for i, n := range sizes {
		entry := make([]byte, 16)
		if n < 256 {
			entry[0], entry[1] = byte(n), byte(n)
		}
		binary.LittleEndian.PutUint16(entry[4:], 1)  // planes
		binary.LittleEndian.PutUint16(entry[6:], 32) // bits per pixel
		binary.LittleEndian.PutUint32(entry[8:], uint32(len(blobs[i])))
		binary.LittleEndian.PutUint32(entry[12:], uint32(offset))
		ico.Write(entry)
		offset += len(blobs[i])
	}
	for _, b := range blobs {
		ico.Write(b)
	}

	if err := os.MkdirAll("winres", 0o755); err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(*out, ico.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %s: %d sizes, %d bytes", *out, len(sizes), ico.Len())
}

// draw renders the icon at n pixels a side, antialiased by sampling each
// pixel 4 by 4.
func draw(n int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	scale := float64(n) / box
	glyph := glyphPath()
	half := 0.72 * scale // the stroke of the paths is 2 of the view box, scaled

	const sub = 4
	step := 1.0 / sub
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			var bg, fg float64
			for sy := 0; sy < sub; sy++ {
				for sx := 0; sx < sub; sx++ {
					px := (float64(x) + (float64(sx)+0.5)*step - 0.5) / scale
					py := (float64(y) + (float64(sy)+0.5)*step - 0.5) / scale
					p := pt{px, py}
					if !inRoundRect(p) {
						continue
					}
					bg++
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
			c := accent
			if fg > 0 {
				w := fg / bg
				c = mix(accent, ink, w)
			}
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
