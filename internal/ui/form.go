// form.go is the console: the form of the session to open, and what acts on
// it.
package ui

import (
	"strconv"

	myui "github.com/egoist/mygo/ui"

	"localrdp/internal/store"
)

// console is the form of the session to open, and what acts on it. The
// buttons stay at the bottom of the window, so they are there whatever the
// form's height asks for; only the form itself scrolls.
func (st *State) console(c *myui.Context) {
	t := c.Theme()
	myui.Column(c).Fill().Grow(1).Shrink(1).Children(func() {
		myui.Row(c).Padding(20, 22, 12, 20).Shrink(0).Gap(10).AlignItems(myui.Center).Children(func() {
			myui.Icon(c, iconDesktop.svg).Size(20, 20).TextColor(t.Accent)
			myui.Text(c, T("New session…")).FontSize(18).Bold().TextColor(t.Text)
			myui.Spacer(c)
			if len(st.Sessions) > 0 {
				myui.Button(c, T("Back to the session")).OnClick(func() {
					st.Active = len(st.Sessions) - 1
				})
			}
			if st.Busy {
				myui.Text(c, st.Status).FontSize(11).TextColor(t.TextMuted).SingleLine()
			}
		})
		// 列布局会按内容给子元素定高，滚动区必须把高度说明白：窗口高度减去
		// 标题栏、页头和页脚这固定的三段。
		height := st.WindowH - max32(c.TitleBar().Height, 40) - kConsoleHead - kConsoleFoot
		myui.Scroll(c).Height(max32(height, 120)).Shrink(0).Children(func() {
			myui.Column(c).FillWidth().Padding(0, 20, 12, 20).Gap(12).Children(func() {
				st.form(c)
			})
		})
		myui.Column(c).Padding(14, 20, 16, 20).Shrink(0).Gap(8).Children(func() {
			st.actions(c)
		})
	})
}

// The room the console keeps for its head and its foot, out of the scroll.
const (
	kConsoleHead = 56
	kConsoleFoot = 80
)

// form is the session's fields: the account across the top, then the
// session's program, the host window's size, and the switches.
func (st *State) form(c *myui.Context) {
	wide := st.WindowW >= 980
	left := func() {
		st.accountCard(c)
		st.windowCard(c)
	}
	right := func() {
		st.modeCard(c)
		st.optionCard(c)
	}
	if wide {
		myui.Row(c).Gap(16).AlignItems(myui.Start).Children(func() {
			myui.Column(c).Grow(1).Basis(0).Gap(12).Children(left)
			myui.Column(c).Grow(1).Basis(0).Gap(12).Children(right)
		})
		return
	}
	myui.Column(c).Gap(12).Children(func() {
		left()
		myui.Divider(c)
		right()
	})
}

// card is a titled box the form is made of; it takes the width it is given,
// and never asks for more than that.
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

// resolution is one of the sizes the window offers, so a common one is one
// click away instead of two numbers typed.
type resolution struct{ W, H int }

// resolutions are the sizes the quick pick offers, widest use first.
var resolutions = []resolution{
	{1024, 768}, {1280, 720}, {1280, 800}, {1280, 1024}, {1366, 768},
	{1440, 900}, {1600, 900}, {1920, 1080}, {1920, 1200}, {2560, 1440},
}

// label is the size as the button shows it.
func (r resolution) label() string {
	return strconv.Itoa(r.W) + "×" + strconv.Itoa(r.H)
}

// windowCard is the host window's title and size.
func (st *State) windowCard(c *myui.Context) {
	t := c.Theme()
	card(c, T("Host window"), func() {
		st.field(c, "title", T("Title"), func() {
			myui.TextInput(c.Key("title"), &st.Form.Title).Placeholder("LocalRDP")
		})
		myui.Text(c, T("Resolution")).FontSize(12).TextColor(t.TextMuted)
		myui.Row(c).Gap(6).Wrap().Children(func() {
			for i := range resolutions {
				r := resolutions[i]
				myui.Button(c, r.label()).Height(26).FontSize(11).OnClick(func() {
					st.WidthText = strconv.Itoa(r.W)
					st.HeightText = strconv.Itoa(r.H)
				})
			}
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

// modeCard is what runs in the child session once it connects.
func (st *State) modeCard(c *myui.Context) {
	t := c.Theme()
	card(c, T("Session"), func() {
		st.field(c, "program", T("Run a program"), func() {
			myui.TextInput(c.Key("program"), &st.Form.Program).
				Placeholder(T("Program and arguments, as a command line"))
		})
		myui.Text(c, T("Starts in the child session once it connects. Leave it empty for the desktop alone.")).
			FontSize(11).TextColor(t.TextMuted).Wrap()
	})
}

// optionCard is the window-size choice, the switches and the timeout.
func (st *State) optionCard(c *myui.Context) {
	t := c.Theme()
	card(c, T("Options"), func() {
		st.field(c, "size", T("Window size"), func() {
			st.sizeChoice(c)
		})
		myui.Text(c, T("Fixed keeps the window and the desktop at Width × Height; follow lets the window decide; scale keeps the layout while the window grows.")).
			FontSize(11).TextColor(t.TextMuted).Wrap()
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

// sizeChoice is the window-size segmented control: an index in the State
// kept in step with Form.SizeMode, which is what the child process gets.
func (st *State) sizeChoice(c *myui.Context) {
	modes := store.SizeModes()
	if st.sizeModeSeen != st.Form.SizeMode {
		st.SizeIndex = store.SizeModeIndex(st.Form.SizeMode)
		st.sizeModeSeen = st.Form.SizeMode
	}
	myui.Segmented(c.Key("size"), &st.SizeIndex,
		T("Fixed size"), T("Follow window"), T("Scale to fit"))
	if i := st.SizeIndex; i >= 0 && i < len(modes) && modes[i] != st.Form.SizeMode {
		st.Form.SizeMode = modes[i]
		st.sizeModeSeen = modes[i]
	}
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
			myui.Button(c.Key("dir"), T("Open the data folder")).
				OnClick(func() { st.Act.Do(st.Act.OpenDataFolder) })
		})
	})
}
