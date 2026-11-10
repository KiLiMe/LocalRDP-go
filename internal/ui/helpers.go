// helpers.go is the small pieces the views share: the picks and the space
// the debug switch prints.
package ui

import (
	"strings"

	myui "github.com/egoist/mygo/ui"
)

// trimSpace is s without the space around it.
func trimSpace(s string) string { return strings.TrimSpace(s) }

// max32 is the larger of a and b.
func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// firstNonEmpty is s, or the stand-in when it is blank.
func firstNonEmpty(s, stand string) string {
	if s = trimSpace(s); s == "" {
		return stand
	}
	return s
}

// pickText picks one of two strings.
func pickText(on bool, yes, no string) string {
	if on {
		return yes
	}
	return no
}

// pickColor picks one of two colors.
func pickColor(t *myui.Theme, on bool, yes, no myui.Color) myui.Color {
	if on {
		return yes
	}
	return no
}

// dumpMain prints the main row's size, when DebugSizes asks for it.
func dumpMain(c *myui.Context, e myui.Element) {
	if !DebugSizes {
		return
	}
	b := e.Bounds()
	println("main row", int(b.X), int(b.Y), int(b.W), int(b.H))
}
