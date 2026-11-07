// window.go is the window's frame: its own title bar, and the row under it.
package ui

import (
	myui "github.com/egoist/mygo/ui"
)

// view builds a frame of the window.
func (st *State) view(c *myui.Context) {
	c.SetTheme(theme())
	st.formNumbers()
	st.Dark = c.Theme().Dark
	st.WindowW, st.WindowH = c.Size()
	myui.Column(c).Fill().Children(func() {
		st.titleBar(c)
		main := myui.Row(c).Fill().AlignItems(myui.Stretch).Justify(myui.Start)
		dumpMain(c, main)
		main.Children(func() {
			if st.Active >= 0 && st.Active < len(st.Sessions) {
				st.sessionView(c)
			} else {
				st.console(c)
			}
		})
	})
	if st.SettingsOpen {
		st.settingsDialog(c)
	} else if st.settingsShown {
		// Closed without saving: the language tried on goes back.
		setLang(st.Settings.Lang)
	}
	st.settingsShown = st.SettingsOpen
	if st.CloseOpen {
		st.closeDialog(c)
	}
	// What was just done is said once, on the frame after it was done, so
	// the toast is raised with the picture the state now makes.
	if st.ToastSeq != st.toastSeen {
		st.toastSeen = st.ToastSeq
		c.Toast(st.ToastMsg)
	}
}

// titleBar is the strip above the window: the name, the button back to the
// console, and the settings gear. The window itself moves the window by it.
func (st *State) titleBar(c *myui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	myui.Row(c).Height(max32(bar.Height, 40)).FillWidth().Shrink(0).AlignItems(myui.Center).
		PaddingStart(12).PaddingEnd(max32(bar.Right, 100)).Gap(8).
		Background(t.Surface).
		BorderWidth(0, 0, 0, 1).BorderColor(t.Border).DragWindow().Children(func() {
		myui.Icon(c, iconApp.svg).Size(18, 18).TextColor(t.Accent)
		myui.Text(c, "LocalRDP").FontSize(13).Bold().TextColor(t.Text).DragWindow()
		myui.Box(c).Grow(1)
		if st.hasSession() {
			iconButton(c, iconPlus, T("New session"), false, func() { st.Active = -1 })
		}
		iconButton(c, iconSettings, T("Settings"), false, st.openSettings)
	})
}

// hasSession reports whether a session's view is in front.
func (st *State) hasSession() bool {
	return st.Active >= 0 && st.Active < len(st.Sessions)
}

// openSettings opens the settings dialog on a copy of what is saved, so
// Cancel can put the language back.
func (st *State) openSettings() {
	st.SettingsDraft = st.Settings
	st.SettingsOpen = true
}
