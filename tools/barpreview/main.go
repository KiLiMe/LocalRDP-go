//go:build barpreview

// The bar of the session window, drawn into a PNG, without connecting to
// anything: four rows, one per state.
//
//	go run -tags barpreview ./tools/barpreview
package main

import (
	"log"
	"os"

	"localrdp/internal/i18n"
	"localrdp/internal/rdp"
)

func main() {
	rows := []rdp.BarModelForPreview{
		{LoggedIn: true, Sid: 4},
		{LoggedIn: true, Sid: 12},
		{EverConnected: true},
		{},
	}
	if err := os.MkdirAll("tmp_shot", 0o755); err != nil {
		log.Fatal(err)
	}
	for _, lang := range []string{"en", "zh"} {
		i18n.Set(lang)
		if err := rdp.PreviewBarFile("tmp_shot/bar_"+lang+".png", 1000, rows); err != nil {
			log.Fatal(err)
		}
	}
}
