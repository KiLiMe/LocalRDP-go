// theme.go is the palette the window draws with: Windows 11's surfaces, its
// bluer accent and its rounder corners, instead of the toolkit's slate
// greys. The cards are white on the page: the surface on the background.
package ui

import myui "github.com/egoist/mygo/ui"

// kFont is the family the window asks for first; the ones after it are what
// Windows offers when it is missing.
const kFont = "Segoe UI Variable Text, Segoe UI, Microsoft YaHei"

// theme is the palette the window is drawn with.
func theme() *myui.Theme { return winTheme(false) }

// winTheme is Windows 11 in the light, or in the dark.
func winTheme(dark bool) *myui.Theme {
	if dark {
		t := myui.DarkTheme()
		t.Background = myui.Hex("#202020")
		t.Surface = myui.Hex("#2b2b2b")
		t.SurfaceHover = myui.Hex("#333333")
		t.SurfacePressed = myui.Hex("#3a3a3a")
		t.Border = myui.Hex("#3f3f3f")
		t.Text = myui.Hex("#f5f5f5")
		t.TextMuted = myui.Hex("#a0a0a0")
		t.Accent = myui.Hex("#4cc2ff")
		t.AccentHover = myui.Hex("#6fd0ff")
		t.AccentPressed = myui.Hex("#3aa9e0")
		t.AccentText = myui.Hex("#00243d")
		t.Danger = myui.Hex("#ff6b5e")
		t.Success = myui.Hex("#3fbf6f")
		t.Selection = myui.RGBA(76, 194, 255, 0.30)
		t.Focus = myui.RGBA(76, 194, 255, 0.60)
		t.Scrollbar = myui.RGBA(255, 255, 255, 0.22)
		t.Font, t.FontSize, t.Radius = kFont, 13, 8
		return t
	}
	t := myui.LightTheme()
	t.Background = myui.Hex("#fbfbfb")
	t.Surface = myui.Hex("#ffffff")
	t.SurfaceHover = myui.Hex("#f4f5f6")
	t.SurfacePressed = myui.Hex("#eceef0")
	t.Border = myui.Hex("#e2e4e8")
	t.Text = myui.Hex("#1b1b1b")
	t.TextMuted = myui.Hex("#6b7280")
	t.Accent = myui.Hex("#0067c0")
	t.AccentHover = myui.Hex("#005a9e")
	t.AccentPressed = myui.Hex("#004e8a")
	t.AccentText = myui.Hex("#ffffff")
	t.Danger = myui.Hex("#c42b1c")
	t.Success = myui.Hex("#0f9d58")
	t.Warning = myui.Hex("#c77700")
	t.Selection = myui.RGBA(0, 103, 192, 0.25)
	t.Focus = myui.RGBA(0, 103, 192, 0.55)
	t.Scrollbar = myui.RGBA(0, 0, 0, 0.20)
	t.Font, t.FontSize, t.Radius = kFont, 13, 8
	return t
}

// tint is the accent washed into the surface, for what is chosen: the
// background of the chosen pill, the way Windows 11 tints its own.
func tint(t *myui.Theme) myui.Color {
	if t.Dark {
		return myui.RGBA(76, 194, 255, 0.16)
	}
	return myui.RGBA(0, 103, 192, 0.10)
}

// tile is the faint fill of the boxes inside a card, which reads as a level
// below the card without a second border color.
func tile(t *myui.Theme) myui.Color {
	if t.Dark {
		return myui.RGBA(255, 255, 255, 0.05)
	}
	return myui.RGBA(0, 0, 0, 0.025)
}
