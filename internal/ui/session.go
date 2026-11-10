// session.go is the card of the session the window opened: what it runs, and
// the buttons that act on its window.
package ui

import (
	myui "github.com/egoist/mygo/ui"
)

// sessionDialog is a session's card, opened over the form: what it runs, and
// the buttons that act on its window. Esc or a click outside drops it, and
// the window's own bar brings it back.
func (st *State) sessionDialog(c *myui.Context) {
	if !st.hasSession() {
		return
	}
	tab := st.Sessions[st.Active]
	id := tab.Session.ID()
	myui.Modal(c, &st.SessionOpen, func() {
		st.dialog(c, tab.Label(), T("The session runs on this computer, in a child session of this account."), func() {
			myui.Button(c, T("Refresh")).Height(30).OnClick(func() {
				st.Act.Do(st.Act.RefreshStatus)
			})
			myui.Button(c, T("Reconnect")).Height(30).Disabled(!tab.Open).OnClick(func() {
				st.Act.Do(func() { st.Act.ReconnectSession(id) })
			})
			myui.Button(c, T("Close session")).Height(30).OnClick(func() {
				st.SessionOpen = false
				st.Act.Do(func() { st.Act.CloseSession(id) })
			})
			myui.PrimaryButton(c, T("Show session")).Height(30).OnClick(func() {
				st.Act.Do(func() { st.Act.ShowSession(id) })
			})
		}, func() {
			myui.Column(c).Gap(8).Children(func() {
				st.fact(c, T("Program"), firstNonEmpty(tab.Session.Program, "—"))
				st.fact(c, T("User name"), firstNonEmpty(tab.Session.User, currentUser()))
				st.fact(c, T("Status"), pickText(tab.Open, T("Open"), T("Done")))
				st.fact(c, T("Host window"), tab.Title)
				st.fact(c, T("Exit code"), pickText(tab.Open, "—", tab.Status))
			})
		})
	})
}

// fact is one line of the session's card: what it is, and what it is now.
func (st *State) fact(c *myui.Context, label, value string) {
	t := c.Theme()
	myui.Row(c).Gap(10).AlignItems(myui.Center).Children(func() {
		myui.Text(c, label).FontSize(12).TextColor(t.TextMuted).Width(110)
		myui.Text(c, value).FontSize(12).TextColor(t.Text).SingleLine()
	})
}
