// session.go is the card of the session the window opened: what it runs, and
// the buttons that act on its window.
package ui

import (
	myui "github.com/egoist/mygo/ui"

	"localrdp/locales"
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
		st.dialog(c, tab.Label(), locales.SessionNote(), func() {
			myui.Button(c, locales.Refresh()).Height(30).OnClick(func() {
				st.Act.Do(st.Act.RefreshStatus)
			})
			myui.Button(c, locales.Reconnect()).Height(30).Disabled(!tab.Open).OnClick(func() {
				st.Act.Do(func() { st.Act.ReconnectSession(id) })
			})
			myui.Button(c, locales.CloseSession()).Height(30).OnClick(func() {
				st.SessionOpen = false
				st.Act.Do(func() { st.Act.CloseSession(id) })
			})
			myui.PrimaryButton(c, locales.ShowSession()).Height(30).Disabled(!tab.Open).OnClick(func() {
				st.Act.Do(func() { st.Act.ShowSession(id) })
			})
		}, func() {
			myui.Column(c).Gap(8).Children(func() {
				st.fact(c, locales.Program(), firstNonEmpty(tab.Session.Program, "—"))
				st.fact(c, locales.UserName(), firstNonEmpty(tab.Session.User, currentUser()))
				st.fact(c, locales.Status(), pickText(tab.Open, locales.StateOpen(), locales.Done()))
				st.fact(c, locales.HostWindow(), tab.Title)
				st.fact(c, locales.ExitCode(), pickText(tab.Open, "—", tab.Status))
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
