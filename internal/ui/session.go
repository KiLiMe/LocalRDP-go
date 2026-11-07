// session.go is the tab of the session the window opened: what it runs, and
// the buttons that act on its window.
package ui

import (
	myui "github.com/egoist/mygo/ui"
)

// sessionView is a session's tab: what it runs, and the buttons that act
// on its window.
func (st *State) sessionView(c *myui.Context) {
	t := c.Theme()
	tab := st.Sessions[st.Active]
	myui.Column(c).Fill().Grow(1).Shrink(1).Padding(20).Gap(14).Children(func() {
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
		myui.Row(c).Gap(8).Wrap().Children(func() {
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
			FontSize(11).TextColor(t.TextMuted).Wrap()
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
