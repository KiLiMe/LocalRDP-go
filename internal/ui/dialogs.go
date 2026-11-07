// dialogs.go is the dialog the window opens over itself: the settings, and
// the question before it quits.
package ui

import (
	"strconv"

	"localrdp/internal/store"

	myui "github.com/egoist/mygo/ui"
)

// settingsDialog is the settings: the language, and what the window keeps
// between runs. It edits a draft, so Cancel leaves everything as it was.
func (st *State) settingsDialog(c *myui.Context) {
	t := c.Theme()
	myui.Modal(c, &st.SettingsOpen, func() {
		d := &st.SettingsDraft
		st.dialog(c, T("Settings"), T("Settings hint"), func() {
			myui.Button(c, T("Cancel")).Height(30).OnClick(func() {
				setLang(st.Settings.Lang)
				st.SettingsOpen = false
			})
			myui.PrimaryButton(c, T("Save")).Height(30).OnClick(func() {
				st.Settings = *d
				if !st.Settings.SavePasswords {
					st.SavePassword = false
				}
				st.SettingsOpen = false
				st.Act.Do(st.Act.SaveSettings)
			})
		}, func() {
			myui.Text(c, T("Language")).FontSize(12).TextColor(t.TextMuted).Margin(16, 0, 6, 0)
			lang := st.settingsLangIndex()
			if myui.Segmented(c, &lang, "English", "中文").Height(30).Changed() {
				d.Lang = pickLang(lang)
				setLang(d.Lang)
			}
			myui.Divider(c).Margin(18, 0, 16, 0)
			checkRow(c, &d.SavePasswords, T("Keep passwords"), T("Keep passwords hint"))
			checkRow(c, &d.Remember, T("Remember the last session"), T("Remember hint")).
				Margin(14, 0, 0, 0)
		})
	})
}

// closeDialog asks before the window closes with a session open.
func (st *State) closeDialog(c *myui.Context) {
	myui.Modal(c, &st.CloseOpen, func() {
		st.dialog(c, T("Close the window?"), "", func() {
			myui.Button(c, T("Cancel")).Height(30).OnClick(func() { st.CloseOpen = false })
			myui.PrimaryButton(c, T("Close")).Height(30).OnClick(func() {
				st.CloseOpen = false
				st.quitting = true
				st.Act.Do(st.Act.Quit)
			})
		}, func() {
			myui.Text(c, T("A child session is open. Closing the window ends it.")).
				FontSize(13).Wrap().Margin(14, 0, 0, 0)
		})
	})
}

// dialog is the card every dialog is made of: a title with a line saying
// what it is for, the body, and the buttons under a hairline. It is pulled
// over the modal's own padding so the card is all that shows.
func (st *State) dialog(c *myui.Context, title, hint string, foot func(), body func()) {
	t := c.Theme()
	myui.Column(c).Width(440).Margin(-20).Radius(10).Background(t.Surface).Border(1, t.Border).
		Shadow(0, 12, 32, 0, myui.RGBA(0, 0, 0, 0.22)).Children(func() {
		myui.Column(c).Padding(22, 24, 2, 24).Gap(3).Children(func() {
			myui.Text(c, title).FontSize(17).Bold().TextColor(t.Text)
			if hint != "" {
				myui.Text(c, hint).FontSize(12).TextColor(t.TextMuted).Wrap()
			}
		})
		myui.Column(c).Padding(0, 24, 14, 24).Children(body)
		myui.Column(c).Children(func() {
			myui.Divider(c)
			myui.Row(c).Padding(14, 20, 16, 20).Gap(8).Justify(myui.End).
				Background(t.SurfaceHover).Radius(0, 0, 10, 10).Children(foot)
		})
	})
}

// checkRow is a checkbox with a line saying what it does under it.
func checkRow(c *myui.Context, v *bool, label, hint string) myui.Element {
	t := c.Theme()
	return myui.Column(c).Gap(2).Children(func() {
		myui.Checkbox(c, v, label)
		myui.Text(c, hint).FontSize(11).TextColor(t.TextMuted).Wrap().Padding(0, 0, 0, 26)
	})
}

// settingsLangIndex is the segment the settings show, from the draft, so
// the choice can be dropped again without touching what is saved.
func (st *State) settingsLangIndex() int {
	if st.SettingsDraft.Lang == "zh" {
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
