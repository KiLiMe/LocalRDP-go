// session.go is the tab of the session the window opened: what it runs, the
// buttons that act on its window, and the panel of what is open.
package ui

import (
	myui "github.com/egoist/mygo/ui"
)

// sessionView is a session's tab: what it runs, and the buttons that act
// on its window.
func (st *State) sessionView(c *myui.Context) {
	t := c.Theme()
	tab := st.Sessions[st.Active]
	myui.Column(c).Fill().Grow(1).Shrink(1).Padding(28, 24).Gap(14).Children(func() {
		myui.Row(c).Gap(10).AlignItems(myui.Center).Children(func() {
			myui.Icon(c, pickIcon(tab.Open, iconDesktopOn, iconDesktop)).Size(20, 20).TextColor(t.Accent)
			myui.Text(c, tab.Label()).FontSize(18).Bold().TextColor(t.Text).SingleLine()
		})
		myui.Column(c).Gap(6).Children(func() {
			st.fact(c, T("Program"), firstNonEmpty(tab.Session.Program, "—"))
			st.fact(c, T("User name"), firstNonEmpty(tab.Session.User, currentUser()))
			st.fact(c, T("Status"), pickText(tab.Open, T("Open"), T("Done")))
			st.fact(c, T("Host window"), tab.Title)
			st.fact(c, T("Exit code"), pickText(tab.Open, "—", tab.Status))
		})
		myui.Row(c).Gap(8).Children(func() {
			myui.PrimaryButton(c, T("Show session")).OnClick(func() {
				st.Act.Do(func() { st.Act.ShowSession(tab.Session.ID()) })
			})
			myui.Button(c, T("Reconnect")).Disabled(!tab.Open).OnClick(func() {
				st.Act.Do(func() { st.Act.ReconnectSession(tab.Session.ID()) })
			})
			myui.Button(c, T("Close session")).OnClick(func() {
				st.Act.Do(func() { st.Act.CloseSession(tab.Session.ID()) })
			})
			myui.Button(c, T("Refresh")).OnClick(func() {
				st.Act.Do(st.Act.RefreshStatus)
			})
		})
		myui.Text(c, T("The session runs on this computer, in a child session of this account.")).
			FontSize(12).TextColor(t.TextMuted).Wrap()
	})
}

// fact is one line of the session tab: what it is, and what it is now.
func (st *State) fact(c *myui.Context, label, value string) {
	t := c.Theme()
	myui.Row(c).Gap(10).AlignItems(myui.Center).Children(func() {
		myui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Width(110)
		myui.Text(c, value).FontSize(12).TextColor(t.Text).SingleLine()
	})
}

// panel is the right side: what is open, and the log's tail.
func (st *State) panel(c *myui.Context) {
	t := c.Theme()
	myui.Column(c).Key("panel").Width(252).FillHeight().Shrink(0).Background(t.Surface).
		BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
		myui.Row(c).Padding(12, 12, 2, 14).Shrink(0).Gap(6).AlignItems(myui.Center).Children(func() {
			myui.Text(c, T("Environment")).FontSize(10).Bold().TextColor(t.TextMuted).Grow(1).SingleLine()
			iconButton(c, iconRefresh, T("Refresh"), false, func() {
				st.Act.Do(st.Act.RefreshStatus)
			})
		})
		myui.Column(c).Padding(0, 12, 8, 12).Gap(2).Shrink(0).Children(func() {
			st.envRow(c, T("Child session"), pickText(st.Open, st.SessionID, T("None")), st.Open)
			hasSecret := storeHasSecret()
			st.envRow(c, T("Cached password"), pickText(hasSecret, T("Saved"), T("Not saved")), hasSecret)
			st.envRow(c, T("Privileges"), pickText(st.PrivOK, T("Ready"), T("Missing")), st.PrivOK)
		})
		myui.Divider(c)
		myui.Row(c).Padding(12, 12, 8, 6).Shrink(0).Gap(6).AlignItems(myui.Center).Children(func() {
			myui.Text(c, T("Log")).FontSize(10).Bold().TextColor(t.TextMuted).Grow(1)
			iconButton(c, iconCopy, T("Copy"), false, func() { st.Act.Do(st.Act.CopyLog) })
			iconButton(c, iconBroom, T("Clear"), false, func() { st.Act.Do(st.Act.ClearLog) })
		})
		myui.Scroll(c).Grow(1).Children(func() {
			myui.Column(c).Padding(0, 12, 12, 12).Children(func() {
				if st.Log == "" {
					myui.Text(c, T("No log yet.")).FontSize(11).TextColor(t.TextMuted)
					return
				}
				myui.Text(c, st.Log).FontSize(11).Font("Consolas, SF Mono, monospace").TextColor(t.TextMuted)
			})
		})
	})
}

// envRow is one line of the environment: what it is and what it is now.
func (st *State) envRow(c *myui.Context, label, value string, ok bool) {
	t := c.Theme()
	myui.Row(c).Height(22).Gap(8).AlignItems(myui.Center).Children(func() {
		myui.Box(c).Size(6, 6).Radius(3).Shrink(0).
			Background(pickColor(t, ok, t.Inverse, t.TextMuted))
		myui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Grow(1).SingleLine()
		myui.Text(c, value).FontSize(12).TextColor(t.Text).SingleLine()
	})
}
