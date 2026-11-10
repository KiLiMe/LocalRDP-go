// icons.go is the icons the window draws: stroke-only paths, so they take
// the color of the text beside them.
package ui

import myui "github.com/egoist/mygo/ui"

// icon is one SVG of the set, made once from its paths.
type icon struct{ svg *myui.SVG }

// paths makes an icon of a 24x24 view box whose strokes take its color.
func paths(d ...string) icon {
	s := `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" ` +
		`fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">`
	for _, p := range d {
		s += `<path d="` + p + `"/>`
	}
	s += `</svg>`
	return icon{svg: myui.MustParseSVG([]byte(s))}
}

var (
	iconApp = paths(
		"M3 5.5A2.5 2.5 0 0 1 5.5 3h4L12 5.5h6.5A2.5 2.5 0 0 1 21 8v10a2.5 2.5 0 0 1-2.5 2.5h-13A2.5 2.5 0 0 1 3 18z",
		"M8 12h8", "M12 9v6",
	)
	iconDesktop   = paths("M3 5h18v11H3z", "M9 20h6", "M12 16v4")
	iconDesktopOn = paths("M3 5h18v11H3z", "M9 20h6", "M12 16v4",
		"M20 3a3.2 3.2 0 1 1-6.4 0 3.2 3.2 0 0 1 6.4 0", "M16.8 1.6v2.8M15.4 3h2.8")
	iconCopy        = paths("M9 9h10v10H9z", "M15 5H5v10")
	iconBroom       = paths("M4 20l6-6", "M9 9l6 6", "M14 4l6 6", "M8 10l6 6-3 2-5-5z")
	iconRefresh     = paths("M20 11a8 8 0 1 0-2.3 6.3", "M20 4v7h-7")
	iconTrash       = paths("M4 7h16", "M9 7V4h6v3", "M6 7l1 13h10l1-13", "M10 11v6", "M14 11v6")
	iconFolderOpen  = paths("M3 7a2 2 0 0 1 2-2h4l2 2h6a2 2 0 0 1 2 2v1", "M3 9h18l-2 9H5z")
	iconLogoff      = paths("M15 12H4", "M10 8l-4 4 4 4", "M14 4h4a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2h-4")
	iconChevronDown = paths("M6 9l6 6 6-6")
	iconChevronUp   = paths("M6 15l6-6 6 6")
	iconSliders     = paths("M4 8h9", "M17 8h3", "M4 16h4", "M13 16h7",
		"M15 8a2 2 0 1 1-4 0 2 2 0 0 1 4 0", "M12 16a2 2 0 1 1-4 0 2 2 0 0 1 4 0")
	iconSettings = paths("M12 15a3 3 0 1 0 0-6 3 3 0 0 0 0 6",
		"M19.4 15a1.7 1.7 0 0 0 .3 1.9l.1.1a2 2 0 1 1-2.8 2.8l-.1-.1a1.7 1.7 0 0 0-1.9-.3 1.7 1.7 0 0 0-1 1.5V21a2 2 0 1 1-4 0v-.1a1.7 1.7 0 0 0-1.1-1.5 1.7 1.7 0 0 0-1.9.3l-.1.1a2 2 0 1 1-2.8-2.8l.1-.1a1.7 1.7 0 0 0 .3-1.9 1.7 1.7 0 0 0-1.5-1H3a2 2 0 1 1 0-4h.1a1.7 1.7 0 0 0 1.5-1.1 1.7 1.7 0 0 0-.3-1.9l-.1-.1a2 2 0 1 1 2.8-2.8l.1.1a1.7 1.7 0 0 0 1.9.3H9a1.7 1.7 0 0 0 1-1.5V3a2 2 0 1 1 4 0v.1a1.7 1.7 0 0 0 1 1.5 1.7 1.7 0 0 0 1.9-.3l.1-.1a2 2 0 1 1 2.8 2.8l-.1.1a1.7 1.7 0 0 0-.3 1.9V9a1.7 1.7 0 0 0 1.5 1H21a2 2 0 1 1 0 4h-.1a1.7 1.7 0 0 0-1.5 1")
)

// iconButton is a small button of an icon, with its name as the tooltip.
func iconButton(c *myui.Context, ic icon, tip string, on bool, fn func()) {
	t := c.Theme()
	box := myui.Box(c).Size(26, 26).Shrink(0).Center().Radius(6).Cursor(myui.CursorPointer).
		Background(pickColor(t, on, t.Accent, myui.RGBA(0, 0, 0, 0))).OnClick(fn).Tooltip(tip)
	// Windows 11 washes a button's own surface in on hover rather than
	// outlining it, so the icon keeps its color and the box changes.
	if box.Hovered() && !on {
		box.Background(t.SurfaceHover)
	}
	box.Children(func() {
		myui.Icon(c, ic.svg).Size(14, 14).
			TextColor(pickColor(t, on, t.AccentText, t.TextMuted))
	})
}
