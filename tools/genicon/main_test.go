package main

import (
	"image/color"
	"image/png"
	"os"
	"testing"
)

// The exe's icon is the committed winres/localrdp.png, which `mygo build`
// bakes in: a drawing that changed without rerunning genicon ships an icon
// nobody drew. Pixels are compared, not bytes, so a different Go version's
// PNG encoder does not fail the test.
func TestIconMatchesTheDrawing(t *testing.T) {
	f, err := os.Open("../../winres/localrdp.png")
	if err != nil {
		t.Fatalf("open ../../winres/localrdp.png: %v (run go run ./tools/genicon)", err)
	}
	defer f.Close()
	got, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode winres/localrdp.png: %v (run go run ./tools/genicon)", err)
	}
	if b := got.Bounds(); b.Dx() != iconSize || b.Dy() != iconSize {
		t.Fatalf("winres/localrdp.png is %dx%d, want %dx%d (run go run ./tools/genicon)", b.Dx(), b.Dy(), iconSize, iconSize)
	}
	want := draw(iconSize)
	for y := 0; y < iconSize; y++ {
		for x := 0; x < iconSize; x++ {
			at := func(img interface{ At(int, int) color.Color }, x, y int) color.NRGBA {
				return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			}
			w, g := at(want, x, y), at(got, x, y)
			if !near(w, g) {
				t.Fatalf("winres/localrdp.png differs from draw(%d) at (%d,%d): has %v, wants %v (run go run ./tools/genicon)", iconSize, x, y, g, w)
			}
		}
	}
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
