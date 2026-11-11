package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// The exe's icon is the committed winres/localrdp.png, which `mygo build`
// bakes in: a drawing that changed without rerunning genicon ships an icon
// nobody drew. Pixels are compared, not bytes, so a different Go version's
// PNG encoder does not fail the test.
//
// 会话窗口那张 localrdp-session.png 同样要跟着画法走，所以两张一起对。
func TestIconMatchesTheDrawing(t *testing.T) {
	for _, c := range []struct {
		path  string
		badge bool
	}{
		{"../../winres/localrdp.png", false},
		{"../../winres/localrdp-session.png", true},
	} {
		t.Run(c.path, func(t *testing.T) {
			f, err := os.Open(c.path)
			if err != nil {
				t.Fatalf("open %s: %v (run go run ./tools/genicon)", c.path, err)
			}
			defer f.Close()
			got, err := png.Decode(f)
			if err != nil {
				t.Fatalf("decode %s: %v (run go run ./tools/genicon)", c.path, err)
			}
			if b := got.Bounds(); b.Dx() != iconSize || b.Dy() != iconSize {
				t.Fatalf("%s is %dx%d, want %dx%d (run go run ./tools/genicon)", c.path, b.Dx(), b.Dy(), iconSize, iconSize)
			}
			want := render(iconSize, c.badge)
			for y := 0; y < iconSize; y++ {
				for x := 0; x < iconSize; x++ {
					w, g := at(want, x, y), at(got, x, y)
					if !near(w, g) {
						t.Fatalf("%s differs from render(%d, badge=%v) at (%d,%d): has %v, wants %v (run go run ./tools/genicon)",
							c.path, iconSize, c.badge, x, y, g, w)
					}
				}
			}
		})
	}
}

// 角标真的画上去了：会话版在点的位置是琥珀色，主图标在那里是蓝色底。
func TestSessionIconCarriesTheBadge(t *testing.T) {
	x := int(badgeAt.x * iconSize / box)
	y := int(badgeAt.y * iconSize / box)
	plain := at(draw(iconSize), x, y)
	session := at(render(iconSize, true), x, y)
	if plain == session {
		t.Fatalf("badge pixel at (%d,%d) is %v on both icons", x, y, plain)
	}
	if near(session, plain) || session.R < 0x80 || session.B > 0x80 {
		t.Fatalf("badge pixel at (%d,%d) is %v, want amber", x, y, session)
	}
}

func at(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

// near is whether two colors are the same but for the last bit of rounding.
func near(a, b color.NRGBA) bool {
	close := func(x, y uint8) bool {
		if x > y {
			x, y = y, x
		}
		return y-x <= 1
	}
	return close(a.R, b.R) && close(a.G, b.G) && close(a.B, b.B) && close(a.A, b.A)
}
