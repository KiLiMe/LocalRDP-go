// chrome.go is the window's frame: its own title bar, the menus at the
// left of it, the window buttons, and the rail of sessions under it.
package ui

import (
	myui "github.com/egoist/mygo/ui"
)

// titleBar is the strip above the window: the name, the menus, and the
// window buttons, which the window itself moves the window by.
func (st *State) titleBar(c *myui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	myui.Row(c).Height(max32(bar.Height, 40)).FillWidth().Shrink(0).AlignItems(myui.Center).
		PaddingStart(12).PaddingEnd(max32(bar.Right, 100)).Gap(8).
		Background(t.Surface).
		BorderWidth(0, 0, 0, 1).BorderColor(t.Border).DragWindow().Children(func() {
		myui.Icon(c, iconApp.svg).Size(18, 18).TextColor(t.Accent)
		myui.Text(c, "LocalRDP").FontSize(13).Bold().TextColor(t.Text).DragWindow()
		myui.Box(c).Width(6)
		st.menus(c)
	})
}

// max32 is the larger of a and b.
func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// menus is File, Session, and Help; the items are drawn by the window,
// so a frame of them is what they act on.
func (st *State) menus(c *myui.Context) {
	t := c.Theme()
	menus := []struct {
		label string
		items func(m *myui.Menu)
	}{
		{T("File"), func(m *myui.Menu) {
			m.Item(T("Settings…")).Chosen()
			m.Separator()
			m.Item(T("Open the data folder")).Chosen()
			m.Separator()
			m.Item(T("Quit")).Chosen()
		}},
		{T("View"), func(m *myui.Menu) {
			m.Item(T("Refresh")).Chosen()
			m.Item(T("Clear the log")).Chosen()
		}},
		{T("Session"), func(m *myui.Menu) {
			m.Item(T("New session…")).Chosen()
			m.Separator()
			m.Item(T("Open child session")).Disabled(st.Busy).Chosen()
			m.Item(T("Status")).Disabled(st.Busy).Chosen()
			m.Item(T("Log off the child session")).Disabled(st.Busy || !st.Open).Chosen()
			m.Separator()
			m.Item(T("Grant privileges")).Disabled(st.Busy).Chosen()
			m.Item(T("Forget password")).Disabled(st.Busy).Chosen()
			m.Item(T("Shortcut")).Disabled(st.Busy).Chosen()
		}},
		{T("Help"), func(m *myui.Menu) {
			m.Item(T("About LocalRDP")).Chosen()
		}},
	}
	for _, menu := range menus {
		label, items := menu.label, menu.items
		myui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Padding(6, 6).
			Radius(4).Cursor(myui.CursorPointer).Menu(func(m *myui.Menu) {
			items(m)
			switch {
			case m.Item(T("Settings…")).Chosen():
				st.openSettings()
			case m.Item(T("Open the data folder")).Chosen():
				st.Act.Do(st.Act.OpenDataFolder)
			case m.Item(T("Quit")).Chosen():
				st.Act.Do(st.Act.Quit)
			case m.Item(T("Clear the log")).Chosen():
				st.Act.Do(st.Act.ClearLog)
			}
		})
	}
}

// askClose closes the window, after asking when a session is open; the
// menus and the shortcut use it.

// rail is the column of sessions on the left, and the button that opens
// a new one.
func (st *State) rail(c *myui.Context) {
	t := c.Theme()
	rail := myui.Column(c).Key("rail").Width(148).FillHeight().Shrink(0).
		Background(t.Surface).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
		myui.Column(c).Padding(10).Shrink(0).Children(func() {
			if myui.Button(c, "+ "+T("New session")).FillWidth().Clicked() {
				st.Active = -1
			}
		})
		myui.Row(c).Padding(12, 12, 4, 6).Shrink(0).Children(func() {
			myui.Text(c, T("Session list")).FontSize(10).Bold().TextColor(t.TextMuted)
		})
		myui.Scroll(c).Grow(1).Children(func() {
			myui.Column(c).Padding(8, 0, 8, 6).Children(func() {
				if len(st.Sessions) == 0 {
					myui.Text(c, T("No sessions yet.")).FontSize(11).TextColor(t.TextMuted).Wrap()
				}
				for i := range st.Sessions {
					st.railItem(c, i)
				}
			})
		})
		myui.Column(c).Padding(12, 8).Shrink(0).BorderWidth(1, 0, 0, 0).
			BorderColor(t.Border).Children(func() {
			myui.Text(c, "v"+Version).FontSize(10).TextColor(t.TextMuted)
		})
	})
	dumpRail(c, rail)
}

// railItem is one session of the rail.
func (st *State) railItem(c *myui.Context, i int) {
	t := c.Theme()
	tab, on := &st.Sessions[i], i == st.Active
	myui.Column(c).Key(struct{ Rail int }{i}).FillWidth().Padding(9, 8).Radius(6).Gap(6).
		Background(pickColor(t, on, t.Selection, myui.RGBA(0, 0, 0, 0))).
		Cursor(myui.CursorPointer).OnClick(func() { st.Active = i }).
		ContextMenu(func(m *myui.Menu) {
			if m.Item(T("Close session")).Chosen() {
				id := tab.Session.ID()
				st.Act.Do(func() { st.Act.CloseSession(id) })
			}
			if m.Item(T("Show session")).Chosen() {
				id := tab.Session.ID()
				st.Act.Do(func() { st.Act.ShowSession(id) })
			}
		}).Children(func() {
		myui.Row(c).Gap(8).AlignItems(myui.Center).Children(func() {
			myui.Icon(c, pickIcon(tab.Open, iconDesktopOn, iconDesktop)).Size(14, 14).
				TextColor(pickColor(t, on, t.AccentText, t.TextMuted))
			myui.Text(c, tab.Label()).FontSize(12).SingleLine().Grow(1).
				TextColor(pickColor(t, on, t.AccentText, t.Text))
		})
		myui.Text(c, tab.Title).FontSize(10).SingleLine().
			TextColor(pickColor(t, on, t.AccentText, t.TextMuted))
		myui.Text(c, pickStr(tab.Open, T("Open"), T("Closed"))).FontSize(10).
			TextColor(pickColor(t, on, t.AccentText, t.TextMuted))
	})
}

// openSettings opens the settings dialog.
func (st *State) openSettings() { st.SettingsOpen = true }

// dumpRail prints the rail's size, when DebugSizes asks for it.
func dumpRail(c *myui.Context, e myui.Element) {
	if !DebugSizes {
		return
	}
	b := e.Bounds()
	println("rail", int(b.X), int(b.Y), int(b.W), int(b.H))
}
