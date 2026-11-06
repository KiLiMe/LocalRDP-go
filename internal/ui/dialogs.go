// dialogs.go is the dialog the window opens over itself: the settings,
// the sessions of the store, and the question before it quits.
package ui

import (
	"strconv"

	"localrdp/internal/store"

	myui "github.com/egoist/mygo/ui"
)

// settingsDialog is the settings: the language, and whether passwords
// are kept.
func (st *State) settingsDialog(c *myui.Context) {
	myui.Modal(c, &st.SettingsOpen, func() {
		st.dialog(c, T("Settings"), func() {
			lang := st.langIndex()
			myui.Text(c, T("Language")).FontSize(12)
			if myui.Segmented(c, &lang, "English", "中文").Changed() {
				st.Settings.Lang = pickLang(lang)
				setLang(st.Settings.Lang)
			}
			myui.Checkbox(c, &st.Settings.SavePasswords, T("Keep passwords"))
			myui.Checkbox(c, &st.Settings.Remember, T("Remember the last session"))
			myui.Row(c).Gap(8).Justify(myui.End).Children(func() {
				myui.Button(c, T("Cancel")).OnClick(func() { st.SettingsOpen = false })
				myui.PrimaryButton(c, T("Save")).OnClick(func() {
					st.Act.Do(st.Act.SaveSettings)
					st.SettingsOpen = false
				})
			})
		})
	})
}

// sessionsDialog is the sessions the store remembers.
func (st *State) sessionsDialog(c *myui.Context) {
	myui.Modal(c, &st.SessionsOpen, func() {
		st.dialog(c, T("Sessions"), func() {
			myui.Button(c, "+ "+T("New session")).OnClick(func() {
				st.SessionsOpen = false
				st.Active = -1
			})
			for i := range st.Sessions {
				tab := &st.Sessions[i]
				myui.Row(c).Gap(8).AlignItems(myui.Center).Children(func() {
					myui.Text(c, tab.Label()).FontSize(12).Grow(1).SingleLine()
					if myui.Button(c, T("Show session")).Clicked() {
						st.SessionsOpen = false
						st.Act.Do(func() { st.Act.ShowSession(tab.Session.ID()) })
					}
				})
			}
			myui.Row(c).Justify(myui.End).Children(func() {
				myui.Button(c, T("Close")).OnClick(func() { st.SessionsOpen = false })
			})
		})
	})
}

// closeDialog asks before the window closes with a session open.
func (st *State) closeDialog(c *myui.Context) {
	myui.Modal(c, &st.CloseOpen, func() {
		st.dialog(c, T("Close the window?"), func() {
			myui.Text(c, T("A child session is open. Closing the window ends it.")).
				FontSize(12).Wrap()
			myui.Row(c).Gap(8).Justify(myui.End).Children(func() {
				myui.Button(c, T("Cancel")).OnClick(func() { st.CloseOpen = false })
				myui.PrimaryButton(c, T("Close")).OnClick(func() {
					st.CloseOpen = false
					st.quitting = true
					st.Act.Do(st.Act.Quit)
				})
			})
		})
	})
}

// aboutDialog is what the program is.
func (st *State) aboutDialog(c *myui.Context) {
	t := c.Theme()
	myui.Modal(c, &st.About, func() {
		st.dialog(c, T("About LocalRDP"), func() {
			myui.Text(c, "LocalRDP "+Version).FontSize(14).Bold().TextColor(t.Text)
			myui.Text(c, T("A child session of this account, in a window of its own.")).
				FontSize(12).TextColor(t.TextMuted).Wrap()
			myui.Text(c, T("Made with mygo")).FontSize(11).TextColor(t.TextMuted)
			myui.Row(c).Justify(myui.End).Children(func() {
				myui.Button(c, T("Close")).OnClick(func() { st.About = false })
			})
		})
	})
}

// dialog is the card every dialog is made of.
func (st *State) dialog(c *myui.Context, title string, body func()) {
	t := c.Theme()
	myui.Column(c).Width(420).Padding(20).Gap(14).Radius(12).
		Background(t.Surface).Border(1, t.Border).Shadow(0, 8, 24, 0, myui.RGBA(0, 0, 0, 0.28)).Children(func() {
		myui.Text(c, title).FontSize(16).Bold().TextColor(t.Text)
		body()
	})
}

// langIndex is the segment the settings show.
func (st *State) langIndex() int {
	if st.Settings.Lang == "zh" {
		return 1
	}
	return 0
}

// askQuit closes the window, after asking when a session is open.
func (st *State) askQuit() {
	if st.Open {
		st.CloseOpen = true
		return
	}
	st.quitting = true
	st.Act.Do(st.Act.Quit)
}

// formNumbers turns the numbers typed in the form into the session's, so
// the form's text stays what the user typed.
func (st *State) formNumbers() {
	number := func(s string, fallback int) int {
		if n, err := strconv.Atoi(trimSpace(s)); err == nil && n > 0 {
			return n
		}
		return fallback
	}
	if st.WidthText == "" {
		st.WidthText = strconv.Itoa(st.Form.Width)
	}
	if st.HeightText == "" {
		st.HeightText = strconv.Itoa(st.Form.Height)
	}
	if st.TimeoutText == "" {
		st.TimeoutText = strconv.Itoa(st.Form.Timeout)
	}
	st.Form.Width = number(st.WidthText, store.DefaultWidth)
	st.Form.Height = number(st.HeightText, store.DefaultHeight)
	st.Form.Timeout = number(st.TimeoutText, store.DefaultTimeout)
	if st.Form.Timeout < 5 {
		st.Form.Timeout = 5
	}
}

// currentUser is the account Windows is running, as DOMAIN\user.
func currentUser() string { return store.CurrentUser() }
