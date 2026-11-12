// form.go is the console: the form of the session to open, and what acts on
// it.
package ui

import (
	"strconv"

	myui "github.com/egoist/mygo/ui"

	"localrdp/internal/store"
	"localrdp/locales"
)

// console is the form of the session to open: one line at the top that
// opens it, and two columns of cards under it that say what it opens. The
// cards scroll; the line above them does not.
func (st *State) console(c *myui.Context) {
	myui.Column(c).Fill().Grow(1).Shrink(1).Children(func() {
		st.consoleHead(c)
		// 列布局会按内容给子元素定高，滚动区必须把高度说明白：窗口高度减去
		// 标题栏与页头这两段固定的。
		height := st.WindowH - max32(c.TitleBar().Height, 40) - kConsoleHead
		myui.Scroll(c).Height(max32(height, 120)).Shrink(0).Children(func() {
			myui.Column(c).FillWidth().Padding(0, 18, 16, 18).Gap(12).Children(func() {
				st.form(c)
			})
		})
	})
}

// kConsoleHead is the room the console keeps for its head, out of the
// scroll.
const kConsoleHead = 56

// consoleHead is the line over the cards: what the console is, and the
// button that opens the session they describe.
func (st *State) consoleHead(c *myui.Context) {
	t := c.Theme()
	myui.Row(c).Padding(14, 18, 10, 18).Shrink(0).Gap(10).AlignItems(myui.Center).Children(func() {
		myui.Icon(c, iconDesktop.svg).Size(18, 18).TextColor(t.Accent)
		myui.Text(c, locales.NewSessionMenu()).FontSize(17).Bold().TextColor(t.Text).SingleLine()
		myui.Spacer(c)
		if st.Busy {
			myui.Text(c, st.Status).FontSize(11).TextColor(t.TextMuted).SingleLine()
		}
		if st.hasSession() {
			myui.Button(c, locales.BackToSession()).Height(30).OnClick(func() {
				st.SessionOpen = true
			})
		}
		myui.PrimaryButton(c.Key("open"), locales.OpenChildSession()).Height(30).
			Disabled(st.Busy).OnClick(func() { st.Act.Do(st.Act.OpenSession) })
	})
}

// form is the cards: the account, the session, what is open and the log on
// the left; the host window and the connection on the right. A window too
// narrow for two columns stacks them in that order instead.
func (st *State) form(c *myui.Context) {
	wide := st.WindowW >= 980
	left := func() {
		st.accountCard(c)
		st.sessionCard(c)
		st.statusCard(c)
		st.logCard(c)
	}
	if wide {
		myui.Row(c).Gap(12).AlignItems(myui.Start).Children(func() {
			myui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(12).Children(left)
			myui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(12).Children(func() {
				st.displayCard(c)
			})
		})
		return
	}
	myui.Column(c).Gap(12).Children(func() {
		left()
		st.displayCard(c)
	})
}

// card is a titled box the form is made of; it takes the width it is given,
// and never asks for more than that. head, when it is not nil, is what acts
// on the card, on the right of its title.
func card(c *myui.Context, title string, head func(), fn func()) {
	t := c.Theme()
	myui.Column(c).Padding(14).Gap(10).Radius(10).MinWidth(0).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		myui.Row(c).Gap(6).AlignItems(myui.Center).Children(func() {
			myui.Text(c, title).FontSize(12).Bold().TextColor(t.Text).Grow(1).SingleLine()
			if head != nil {
				head()
			}
		})
		fn()
	})
}

// cardAction is a card's own action: a small icon and its name, which reads
// as a link until it is hovered, the way Windows 11's settings do.
func cardAction(c *myui.Context, ic icon, label string, fn func()) {
	t := c.Theme()
	link := myui.Row(c).Gap(4).PaddingX(6).PaddingY(3).Radius(5).Shrink(0).
		Cursor(myui.CursorPointer).OnClick(fn).Tooltip(label)
	if link.Hovered() {
		link.Background(t.SurfaceHover)
	}
	link.Children(func() {
		myui.Icon(c, ic.svg).Size(12, 12).TextColor(t.TextMuted)
		myui.Text(c, label).FontSize(11).TextColor(t.TextMuted).SingleLine()
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

// accountCard is the account of the child session, and its password.
func (st *State) accountCard(c *myui.Context) {
	t := c.Theme()
	card(c, locales.Account(), func() {
		cardAction(c, iconTrash, locales.ForgetPassword(), func() {
			st.Act.Do(st.Act.ForgetPassword)
		})
	}, func() {
		st.field(c, "user", locales.UserName(), func() {
			myui.TextInput(c.Key("user"), &st.Form.User).Placeholder(currentUser()).Height(30)
		})
		st.field(c, "password", locales.Password(), func() {
			myui.Row(c).Gap(6).AlignItems(myui.Center).Children(func() {
				myui.TextInput(c.Key("password"), &st.Password).Password().Grow(1).Height(30).
					Placeholder(pickText(storeHasSecret(), locales.PasswordCached(), ""))
				myui.Button(c, locales.Clear()).Height(30).Disabled(st.Password == "").
					OnClick(func() { st.Password = "" })
			})
		})
		myui.Checkbox(c, &st.SavePassword, locales.KeepPassword())
		myui.Text(c, locales.KeepPasswordNote()).
			FontSize(11).TextColor(t.TextMuted).Wrap()
	})
}

// sessionCard is what runs in the child session once it connects.
func (st *State) sessionCard(c *myui.Context) {
	t := c.Theme()
	card(c, locales.SessionSettings(), nil, func() {
		st.field(c, "program", locales.RunProgram(), func() {
			myui.TextInput(c.Key("program"), &st.Form.Program).Height(30).
				Placeholder(locales.ProgramNote())
		})
		myui.Text(c, locales.ProgramHint()).
			FontSize(11).TextColor(t.TextMuted).Wrap()
	})
}

// statusCard is what is open and what is cached: two tiles that read at a
// glance, and the buttons that ask again or end it.
func (st *State) statusCard(c *myui.Context) {
	card(c, locales.RunningState(), func() {
		iconButton(c, iconFolderOpen, locales.OpenDataFolder(), false, func() {
			st.Act.Do(st.Act.OpenDataFolder)
		})
		iconButton(c, iconRefresh, locales.Refresh(), false, func() {
			st.Act.Do(st.Act.RefreshStatus)
		})
		if st.Open {
			iconButton(c, iconLogoff, locales.LogOffChildSession(), false, func() {
				st.Act.Do(st.Act.LogOff)
			})
		}
	}, func() {
		myui.Row(c).Gap(8).Children(func() {
			st.tile(c, locales.ChildSession(), pickText(st.Open, st.SessionID, locales.None()), st.Open)
			hasSecret := storeHasSecret()
			st.tile(c, locales.CachedPassword(), pickText(hasSecret, locales.SavedEncrypted(), locales.NotSaved()), hasSecret)
		})
	})
}

// tile is one reading of the status card: a dot that says whether it is so,
// what it is, and what it is now.
func (st *State) tile(c *myui.Context, label, value string, ok bool) {
	t := c.Theme()
	myui.Row(c).Grow(1).Basis(0).MinWidth(0).Height(32).PaddingX(10).Radius(8).Gap(7).
		AlignItems(myui.Center).Background(tile(t)).Border(1, t.Border).Children(func() {
		myui.Box(c).Size(6, 6).Radius(3).Shrink(0).
			Background(pickColor(t, ok, t.Success, pickColor(t, !ok, t.TextMuted, t.Success)))
		myui.Text(c, label).FontSize(11).TextColor(t.TextMuted).SingleLine()
		myui.Spacer(c)
		myui.Text(c, value).FontSize(11).TextColor(t.Text).SingleLine()
	})
}

// logCard is the tail of the log, folded away until it is asked for: it is
// what the child process said, and the console is about what to start.
func (st *State) logCard(c *myui.Context) {
	t := c.Theme()
	chev := iconChevronDown
	if st.LogOpen {
		chev = iconChevronUp
	}
	card(c, locales.Log(), func() {
		iconButton(c, chev,
			pickText(st.LogOpen, locales.Collapse(), locales.Expand()), false, func() {
				st.LogOpen = !st.LogOpen
			})
		iconButton(c, iconCopy, locales.Copy(), false, func() { st.Act.Do(st.Act.CopyLog) })
		iconButton(c, iconBroom, locales.Clear(), false, func() { st.Act.Do(st.Act.ClearLog) })
	}, func() {
		if !st.LogOpen {
			return
		}
		if st.Log == "" {
			myui.Text(c, locales.NoLog()).FontSize(11).TextColor(t.TextMuted)
			return
		}
		myui.Column(c).Height(180).Radius(8).Padding(10).Background(tile(t)).Children(func() {
			myui.Scroll(c).Height(160).Children(func() {
				myui.Text(c, st.Log).FontSize(11).Font("Consolas, SF Mono, monospace").
					TextColor(t.TextMuted)
			})
		})
	})
}

// resolution is one of the sizes the window offers, so a common one is one
// click away instead of two numbers typed.
type resolution struct{ W, H int }

// resolutions are the sizes the quick pick offers.
var resolutions = []resolution{
	{1024, 768}, {1280, 720}, {1280, 800}, {1280, 1024}, {1366, 768},
	{1440, 900}, {1600, 900}, {1920, 1080}, {1920, 1200}, {2560, 1440},
}

// label is the size as the pill shows it.
func (r resolution) label() string {
	return strconv.Itoa(r.W) + "×" + strconv.Itoa(r.H)
}

// displayCard is the host window: its title and size, and how it connects.
func (st *State) displayCard(c *myui.Context) {
	t := c.Theme()
	card(c, locales.HostWindowDisplay(), nil, func() {
		st.field(c, "title", locales.WindowTitle(), func() {
			myui.TextInput(c.Key("title"), &st.Form.Title).Placeholder("LocalRDP").Height(30)
		})
		st.field(c, "size", locales.WindowSize(), func() { st.sizeChoice(c) })
		myui.Text(c, st.sizeHint()).FontSize(11).TextColor(t.TextMuted).Wrap()
		myui.Row(c).Gap(8).AlignItems(myui.Center).Children(func() {
			myui.Text(c, locales.Resolution()).FontSize(12).TextColor(t.TextMuted).Grow(1)
			myui.Text(c, pickText(st.CustomRes, locales.ResolutionCustom(), locales.ResolutionPreset())).
				FontSize(10).TextColor(pickColor(t, st.CustomRes, t.Accent, t.TextMuted))
		})
		st.resolutionPills(c)
		myui.Row(c).Gap(10).AlignItems(myui.Start).Children(func() {
			myui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				st.field(c, "width", locales.Width(), func() { st.lockedInput(c, "width", &st.WidthText) })
			})
			myui.Column(c).Grow(1).Basis(0).MinWidth(0).Children(func() {
				st.field(c, "height", locales.Height(), func() { st.lockedInput(c, "height", &st.HeightText) })
			})
			myui.Column(c).Grow(0.6).Basis(0).MinWidth(0).Children(func() {
				st.field(c, "scale", locales.Scale()+" %", func() {
					myui.TextInput(c.Key("scale"), &st.ScaleText).Height(30).
						Placeholder(locales.None())
				})
			})
		})
		myui.Text(c, locales.ScaleHint()).
			FontSize(11).TextColor(t.TextMuted).Wrap()
		myui.Divider(c)
		myui.Text(c, locales.WindowAndConnection()).FontSize(12).TextColor(t.TextMuted)
		myui.Row(c).Gap(18).Wrap().Children(func() {
			myui.Checkbox(c, &st.Form.FullScreen, locales.FullScreen())
			myui.Checkbox(c, &st.Form.KeepSession, locales.KeepSession())
			myui.Checkbox(c, &st.Form.Minimized, locales.Minimized())
			myui.Checkbox(c, &st.Form.NoScaleInherit, locales.NoInheritedScaling())
		})
		myui.Text(c, locales.NoInheritedScalingHint()).FontSize(11).TextColor(t.TextMuted).Wrap()
		st.field(c, "timeout", locales.Timeout()+" ("+locales.Second()+")", func() {
			myui.TextInput(c.Key("timeout"), &st.TimeoutText).Height(30)
		})
	})
}

// lockedInput is one of the two numbers of a size: it is typed in only
// while the size is the user's own, and reads as a value when a preset
// holds it.
func (st *State) lockedInput(c *myui.Context, key string, v *string) {
	t := c.Theme()
	in := myui.TextInput(c.Key(key), v).Height(30).Disabled(!st.CustomRes)
	if !st.CustomRes {
		in.Background(tile(t)).TextColor(t.TextMuted)
	}
}

// resolutionPills is the size pick: the common sizes, and the user's own,
// which is what the width and height are for.
func (st *State) resolutionPills(c *myui.Context) {
	myui.Grid(c).Columns(4).Gap(6).Children(func() {
		for i := range resolutions {
			r := resolutions[i]
			on := !st.CustomRes && st.WidthText == strconv.Itoa(r.W) && st.HeightText == strconv.Itoa(r.H)
			pill(c, r.label(), on, 1, nil, func() {
				st.WidthText = strconv.Itoa(r.W)
				st.HeightText = strconv.Itoa(r.H)
				st.CustomRes = false
			})
		}
		pill(c, locales.CustomResolution(), st.CustomRes, 2, &iconSliders, func() {
			st.CustomRes = true
		})
	})
}

// pill is one choice of the size pick: a rounded box, tinted when it is the
// one the form holds.
func pill(c *myui.Context, label string, on bool, span int, ic *icon, fn func()) {
	t := c.Theme()
	box := myui.Box(c).MinHeight(28).Center().Radius(6).Cursor(myui.CursorPointer).
		PaddingX(8).PaddingY(3).OnClick(fn).
		Background(pickColor(t, on, tint(t), t.Surface)).
		Border(1, pickColor(t, on, t.Accent, t.Border))
	if span > 1 {
		box.ColumnSpan(span)
	}
	if box.Hovered() && !on {
		box.Background(t.SurfaceHover)
	}
	box.Children(func() {
		myui.Row(c).Gap(5).AlignItems(myui.Center).Children(func() {
			if ic != nil {
				myui.Icon(c, ic.svg).Size(12, 12).TextColor(pickColor(t, on, t.Accent, t.TextMuted))
			}
			myui.Text(c, label).FontSize(11).
				TextColor(pickColor(t, on, t.Accent, t.Text)).SingleLine()
		})
	})
}

// sizeHint says what the size mode does, in the mode chosen.
func (st *State) sizeHint() string {
	switch store.NormalizeSizeMode(st.Form.SizeMode) {
	case store.SizeFit:
		return locales.FollowHint()
	case store.SizeScale:
		return locales.StretchHint()
	}
	return locales.FixedHint()
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
		locales.FixedSize(), locales.FollowWindow(), locales.Stretch())
	if i := st.SizeIndex; i >= 0 && i < len(modes) && modes[i] != st.Form.SizeMode {
		st.Form.SizeMode = modes[i]
		st.sizeModeSeen = modes[i]
	}
}
