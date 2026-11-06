// view.go is the window: the title bar, the tabs of the sessions it
// opened, the console of the form, the panel of what is open, and the log.
package ui

import (
	myui "github.com/egoist/mygo/ui"
)

// view builds a frame of the window.
func (st *State) view(c *myui.Context) {
	st.Dark = c.Theme().Dark
	dumpSizes(c, st)
	st.WindowW, st.WindowH = c.Size()
	myui.Column(c).Fill().Children(func() {
		st.titleBar(c)
		main := myui.Row(c).Fill().AlignItems(myui.Stretch).Justify(myui.Start)
		dumpMain(c, main)
		main.Children(func() {
			st.rail(c)
			if st.Active >= 0 && st.Active < len(st.Sessions) {
				st.sessionView(c)
			} else {
				st.console(c)
			}
			st.panel(c)
		})
	})
	if st.SettingsOpen {
		st.settingsDialog(c)
	}
	if st.SessionsOpen {
		st.sessionsDialog(c)
	}
	if st.CloseOpen {
		st.closeDialog(c)
	}
	if st.About {
		st.aboutDialog(c)
	}
}

// sessionView is a session's tab: what it runs, and the buttons that act
// on its window.
func (st *State) sessionView(c *myui.Context) {
	t := c.Theme()
	tab := st.Sessions[st.Active]
	myui.Column(c).Fill().Grow(1).Shrink(1).Padding(28, 24).Gap(14).Children(func() {
		myui.Row(c).Gap(10).AlignItems(myui.Center).Children(func() {
			myui.Icon(c, pickIcon(tab.Open, iconDesktopOn, iconDesktop)).Size(20, 20).TextColor(t.Accent)
			myui.Text(c, tab.Label()).FontSize(18).Bold().TextColor(t.Text).SingleLine()
			myui.Badge(c, modeBadge(tab.Session.Mode))
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
			myui.Button(c, T("Close session")).OnClick(func() {
				st.Act.Do(func() { st.Act.CloseSession(tab.Session.ID()) })
			})
			myui.Button(c, T("Refresh")).OnClick(func() {
				st.Act.Do(st.Act.RefreshStatus)
			})
		})
		myui.Spacer(c)
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

// console is the form of the session to open, and what acts on it.
func (st *State) console(c *myui.Context) {
	t := c.Theme()
	myui.Scroll(c).Fill().Grow(1).Shrink(1).Children(func() {
		myui.Column(c).FillWidth().Padding(28, 24).Gap(16).Children(func() {
			myui.Row(c).Gap(10).AlignItems(myui.Center).Children(func() {
				myui.Icon(c, iconDesktop.svg).Size(20, 20).TextColor(t.Accent)
				myui.Text(c, T("New session…")).FontSize(18).Bold().TextColor(t.Text)
				myui.Spacer(c)
				if st.Busy {
					myui.Text(c, st.Status).FontSize(11).TextColor(t.TextMuted).SingleLine()
				}
			})
			st.form(c)
			st.actions(c)
		})
	})
}

// form is the session's fields: the account across the top, then the
// mode, the host window's size, and the switches.
func (st *State) form(c *myui.Context) {
	wide := st.WindowW >= 980
	account := func() {
		st.accountCard(c)
		st.windowCard(c)
	}
	mode := func() {
		st.modeCard(c)
		st.optionCard(c)
	}
	if wide {
		myui.Row(c).Gap(16).AlignItems(myui.Start).Children(func() {
			myui.Column(c).Grow(1).Basis(0).Gap(12).Children(account)
			myui.Column(c).Grow(1).Basis(0).Gap(12).Children(mode)
		})
		return
	}
	myui.Column(c).Gap(12).Children(func() {
		account()
		myui.Divider(c)
		mode()
	})
}

// card is a titled box the form is made of; it takes the width it is
// given, and never asks for more than that.
func card(c *myui.Context, title string, fn func()) {
	t := c.Theme()
	myui.Column(c).Padding(14).Gap(10).Radius(10).MinWidth(0).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		myui.Text(c, title).FontSize(11).Bold().TextColor(t.TextMuted)
		fn()
	})
}

// field is a labeled input, with the form's complaint under it when it
// belongs there.
func (st *State) field(c *myui.Context, name, label string, input func()) {
	t := c.Theme()
	myui.Column(c).Gap(4).Children(func() {
		myui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
		input()
		if st.FormErrOn == name && st.FormErr != "" {
			myui.Text(c, st.FormErr).FontSize(11).TextColor(t.Danger).Wrap()
		}
	})
}

// accountCard is the account of the child session and its password.
func (st *State) accountCard(c *myui.Context) {
	t := c.Theme()
	card(c, T("Account"), func() {
		st.field(c, "user", T("User name"), func() {
			myui.TextInput(c.Key("user"), &st.Form.User).Placeholder(currentUser())
		})
		st.field(c, "password", T("Password"), func() {
			myui.TextInput(c.Key("password"), &st.Password).Password().
				Placeholder(pickText(storeHasSecret(), T("Password is cached. Type a new one to replace it."), ""))
		})
		myui.Checkbox(c, &st.SavePassword, T("Keep the password"))
		myui.Text(c, T("Keep the password between runs, encrypted for this user.")).
			FontSize(11).TextColor(t.TextMuted).Wrap()
	})
}

// windowCard is the host window's title and size.
func (st *State) windowCard(c *myui.Context) {
	card(c, T("Host window"), func() {
		st.field(c, "title", T("Title"), func() {
			myui.TextInput(c.Key("title"), &st.Form.Title).Placeholder("LocalRDP")
		})
		myui.Row(c).Gap(10).AlignItems(myui.Start).Children(func() {
			myui.Column(c).Grow(1).Basis(0).Children(func() {
				st.field(c, "width", T("Width"), func() {
					myui.TextInput(c.Key("width"), &st.WidthText)
				})
			})
			myui.Column(c).Grow(1).Basis(0).Children(func() {
				st.field(c, "height", T("Height"), func() {
					myui.TextInput(c.Key("height"), &st.HeightText)
				})
			})
		})
	})
}

// modeCard is what the child session runs: the whole desktop, or one
// program.
func (st *State) modeCard(c *myui.Context) {
	t := c.Theme()
	card(c, T("Session"), func() {
		myui.Row(c).Gap(4).Children(func() {
			myui.Segmented(c.Key("mode"), &st.modeIndex, T("Desktop"), T("Start"))
		})
		if st.FormMode() == "start" {
			st.field(c, "program", T("Program"), func() {
				myui.TextInput(c.Key("program"), &st.Form.Program).
					Placeholder(T("Program and arguments, as a command line"))
			})
		} else {
			myui.Text(c, T("desktop — the whole child desktop")).FontSize(11).TextColor(t.TextMuted).Wrap()
		}
	})
}

// optionCard is the switches and the timeout.
func (st *State) optionCard(c *myui.Context) {
	t := c.Theme()
	card(c, T("Options"), func() {
		myui.Row(c).Gap(16).Wrap().Children(func() {
			myui.Checkbox(c, &st.Form.FullScreen, T("Full screen"))
			myui.Checkbox(c, &st.Form.KeepSession, T("Keep session"))
			myui.Checkbox(c, &st.Form.Minimized, T("Minimized"))
		})
		st.field(c, "timeout", T("Timeout")+" ("+T("Second")+")", func() {
			myui.TextInput(c.Key("timeout"), &st.TimeoutText)
		})
		myui.Text(c, T("Privileges")+": "+st.PrivNote).FontSize(11).
			TextColor(pickColor(t, st.PrivOK, t.TextMuted, t.Warning)).Wrap()
	})
}

// actions is the row of buttons that act on the console.
func (st *State) actions(c *myui.Context) {
	t := c.Theme()
	myui.Column(c).Gap(8).Children(func() {
		if st.FormErr != "" && st.FormErrOn == "" {
			myui.Text(c, st.FormErr).FontSize(12).TextColor(t.Danger).Wrap()
		}
		myui.Row(c).Gap(8).Wrap().Children(func() {
			myui.PrimaryButton(c.Key("open"), T("Open child session")).
				Disabled(st.Busy).OnClick(func() { st.Act.Do(st.Act.OpenSession) })
			myui.Button(c.Key("status"), T("Status")).
				Disabled(st.Busy).OnClick(func() { st.Act.Do(st.Act.RefreshStatus) })
			myui.Button(c.Key("logoff"), T("Log off the child session")).
				Disabled(st.Busy || !st.Open).OnClick(func() { st.Act.Do(st.Act.LogOff) })
			myui.Button(c.Key("priv"), T("Grant privileges")).
				Disabled(st.Busy).OnClick(func() { st.Act.Do(st.Act.GrantPrivileges) })
			myui.Button(c.Key("forget"), T("Forget password")).
				Disabled(st.Busy).OnClick(func() { st.Act.Do(st.Act.ForgetPassword) })
			myui.Button(c.Key("shortcut"), T("Shortcut")).
				Disabled(st.Busy).OnClick(func() { st.Act.Do(st.Act.Shortcut) })
			myui.Button(c.Key("dir"), T("Open the data folder")).
				OnClick(func() { st.Act.Do(st.Act.OpenDataFolder) })
		})
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

// firstNonEmpty is s, or the stand-in when it is blank.
func firstNonEmpty(s, stand string) string {
	if s = trimSpace(s); s == "" {
		return stand
	}
	return s
}

// pickText picks one of two strings.
func pickText(on bool, yes, no string) string {
	if on {
		return yes
	}
	return no
}

// pickColor picks one of two colors.
func pickColor(t *myui.Theme, on bool, yes, no myui.Color) myui.Color {
	if on {
		return yes
	}
	return no
}

// pickStr picks one of two strings.
func pickStr(on bool, yes, no string) string {
	if on {
		return yes
	}
	return no
}

// dumpMain prints the main row's size, when DebugSizes asks for it.
func dumpMain(c *myui.Context, e myui.Element) {
	if !DebugSizes {
		return
	}
	b := e.Bounds()
	println("main row", int(b.X), int(b.Y), int(b.W), int(b.H))
}

// dumpSizes is next to the debug switch below, and says nothing by itself.
func dumpSizes(c *myui.Context, st *State) {
	if !DebugSizes {
		return
	}
	println("window", int(st.WindowW), int(st.WindowH), "form", st.FormErr)
}
