package app

import (
	"bytes"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"localrdp/internal/fault"
	"localrdp/internal/i18n"
	"localrdp/internal/store"
	"localrdp/internal/ui"
	"localrdp/internal/win32"
)

// actions is the App as the interface calls it: every method changes the
// state on the UI thread and asks for a frame.
type actions App

// The actions are the App as the interface calls it, so its own methods
// are the App's.

func (a *actions) Do(fn func())              { (*App)(a).Do(fn) }
func (a *actions) Invalidate()               { (*App)(a).Invalidate() }
func (a *actions) Busy() bool                { return (*App)(a).Busy() }
func (a *actions) setBusy(on bool, s string) { (*App)(a).setBusy(on, s) }
func (a *actions) appendLog(lines []string)  { (*App)(a).appendLog(lines) }

// validate checks the form and says what is wrong with it.
func (a *actions) validate() (store.Session, bool) {
	st := &a.St
	st.FormErr, st.FormErrOn = "", ""
	s := st.Form
	s.Name = strings.TrimSpace(s.Name)
	s.Program = strings.TrimSpace(s.Program)
	s.User = strings.TrimSpace(s.User)
	s.Title = strings.TrimSpace(s.Title)
	s.Mode = "desktop"
	if s.Timeout < 5 {
		st.FormErr, st.FormErrOn = ui.T("The timeout is at least 5 seconds."), "timeout"
		return s, false
	}
	if s.Width < 320 || s.Height < 200 {
		st.FormErr, st.FormErrOn = ui.T("The size is at least 320 by 200."), "width"
		return s, false
	}
	if strings.ContainsAny(s.User, "\"`n\r") {
		st.FormErr, st.FormErrOn = ui.T("The user name cannot hold a quote."), "user"
		return s, false
	}
	if s.Scale < 0 || s.Scale > 500 || (s.Scale > 0 && s.Scale < 100) {
		st.FormErr, st.FormErrOn = ui.T("The scale is between 100 and 500 percent, or empty."), "scale"
		return s, false
	}
	return s, true
}

// OpenSession starts the child session of the form.
func (a *actions) OpenSession() {
	st := &a.St
	s, ok := a.validate()
	if !ok {
		a.Invalidate()
		return
	}
	password := st.Password
	if password == "" && store.HasSecret() {
		password = store.LoadSecret()
	}
	if s.User != "" && password == "" {
		st.FormErr, st.FormErrOn = ui.T("A password is needed for that account."), "password"
		a.Invalidate()
		return
	}
	st.Form = s
	st.Busy = true
	st.Status = ui.T("Opening…")
	a.Invalidate()
	go a.open(s, password)
}

// open runs the child process and follows it: the tab shows up as soon as
// the session exists, and closes once the process is done.
func (a *actions) open(s store.Session, password string) {
	defer a.setBusy(false, "")
	id := s.ID()
	cmd := childCommand(s, password)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Start(); err != nil {
		a.Do(func() {
			a.St.FormErr, a.St.FormErrOn = err.Error(), ""
			a.St.Status = err.Error()
		})
		return
	}
	pid := cmd.Process.Pid
	tab := ui.Tab{Session: s, Title: s.Label(), Open: true, PID: pid}
	a.Do(func() {
		if a.St.Password != "" && a.St.SavePassword {
			_ = store.SaveSecret(a.St.Password)
		}
		a.St.Password = ""
		stored := s
		if a.St.Settings.Remember {
			settings := store.LoadSettings()
			settings.Last = stored
			_ = store.SaveSettings(settings)
		}
		replaced := false
		for i := range a.St.Sessions {
			if a.St.Sessions[i].Session.ID() == id {
				a.St.Sessions[i] = tab
				replaced = true
			}
		}
		if !replaced {
			a.St.Sessions = append(a.St.Sessions, tab)
		}
		for i := range a.St.Sessions {
			if a.St.Sessions[i].Session.ID() == id {
				a.St.Active = i
			}
		}
		a.St.Open = true
		a.St.SessionOpen = true
		a.St.Status = ui.T("Running")
		a.St.Notify(ui.T("Running") + " · " + s.Label())
	})
	time.Sleep(1200 * time.Millisecond)
	a.Do(a.RefreshStatus)
	err := cmd.Wait()
	lines := nonEmptyLines(buf.String())
	a.appendLog(lines)
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
	} else if err != nil {
		code = 1
		a.appendLog([]string{err.Error()})
	}
	a.finish(id, code)
	if line, ok := fault.Find(lines); ok {
		// 会话进程已经退出，它为什么失败只剩这一行：留在状态栏上，
		// 别让它跟着标签一起消失。
		a.Do(func() {
			a.St.Status = line
			a.St.Notify(line)
		})
	}
}

// finish marks a session done once its child process has ended, and
// drops its tab when the window closed it.
func (a *actions) finish(id string, code int) {
	a.Do(func() {
		for i := range a.St.Sessions {
			if a.St.Sessions[i].Session.ID() == id {
				a.St.Sessions[i].Open = false
			}
		}
		a.St.Open = false
	})
	a.Do(a.RefreshStatus)
	if code == 0 || code == 4 {
		a.Do(func() {
			kept := make([]ui.Tab, 0, len(a.St.Sessions))
			for _, tab := range a.St.Sessions {
				if tab.Session.ID() != id || tab.Open {
					kept = append(kept, tab)
				}
			}
			a.St.Sessions = kept
			if a.St.Active >= len(a.St.Sessions) {
				a.St.Active = -1
			}
			if code != 0 {
				a.St.Status = ui.T("Done") + " (" + strconv.Itoa(code) + ")"
			}
		})
	}
}

// RefreshStatus asks what is open and what is granted.
func (a *actions) RefreshStatus() {
	out, code := runSelf("status")
	id := ""
	if code == 0 || code == 2 {
		id = parseSessionID(out)
	}
	open := id != ""
	a.Do(func() {
		a.St.SessionID = id
		a.St.Open = open
	})
	a.appendLog(nonEmptyLines(out))
}

// LogOff ends the child session.
func (a *actions) LogOff() {
	st := &a.St
	if !st.Open {
		return
	}
	st.Busy = true
	st.Status = ui.T("Logging off…")
	a.Invalidate()
	go func() {
		out, code := runSelf("stop")
		a.appendLog(nonEmptyLines(out))
		if code != 0 && code != 2 {
			a.appendLog([]string{ui.T("Logging off failed.") + " (" + strconv.Itoa(code) + ")"})
		}
		a.setBusy(false, "")
		a.Do(func() { a.St.Notify(ui.T("Done")) })
		a.Do(a.RefreshStatus)
	}()
}

// ForgetPassword drops the cached password.
func (a *actions) ForgetPassword() {
	_ = store.ForgetSecret()
	a.Do(func() {
		a.St.Password = ""
		a.St.Status = ui.T("Password forgotten")
		a.St.Notify(ui.T("Password forgotten"))
	})
}

// OpenDataFolder shows the folder the program keeps its things in.
func (a *actions) OpenDataFolder() {
	_ = os.MkdirAll(store.Dir(), 0o755)
	_ = exec.Command("explorer.exe", store.Dir()).Start()
	a.Do(func() { a.St.Notify(ui.T("Open the data folder") + " · " + store.Dir()) })
}

// ShowSession brings a session's child window to the front.
func (a *actions) ShowSession(id string) bool {
	for _, tab := range a.St.Sessions {
		if tab.Session.ID() == id {
			if !tab.Open {
				return false
			}
			win32.BringToFront(tab.PID)
			return true
		}
	}
	return false
}

// ReconnectSession asks a running session's window to connect again: the
// same buttons the window's own bar has, pressed from here.
func (a *actions) ReconnectSession(id string) bool {
	for _, tab := range a.St.Sessions {
		if tab.Session.ID() != id || !tab.Open {
			continue
		}
		if !win32.PostCommand(tab.PID, win32.CmdReconnect) {
			return false
		}
		a.appendLog([]string{"Reconnecting " + tab.Label() + "…"})
		return true
	}
	return false
}

// CloseSession ends one and drops its tab.
func (a *actions) CloseSession(id string) {
	target := -1
	for i := range a.St.Sessions {
		if a.St.Sessions[i].Session.ID() == id {
			target = i
		}
	}
	a.Do(func() {
		if target >= 0 {
			a.St.Sessions = append(a.St.Sessions[:target], a.St.Sessions[target+1:]...)
			if a.St.Active >= len(a.St.Sessions) {
				a.St.Active = len(a.St.Sessions) - 1
			}
		}
	})
	if a.St.Open {
		a.Do(a.LogOff)
	}
}

// ClearLog drops the log the window shows.
func (a *actions) ClearLog() {
	if path := store.LogFile(); path != "" {
		_ = os.WriteFile(path, nil, 0o644)
	}
	a.Do(func() { a.St.Log = "" })
}

// CopyLog puts the log on the clipboard.
func (a *actions) CopyLog() {
	if err := copyText(a.St.Log); err != nil {
		a.appendLog([]string{err.Error()})
		return
	}
	a.Do(func() {
		a.St.Status = ui.T("Log copied")
		a.St.Notify(ui.T("Log copied"))
	})
}

// SetLang switches the words and keeps the choice.
func (a *actions) SetLang(lang string) {
	a.Do(func() {
		i18n.Set(lang)
		a.St.Settings.Lang = lang
	})
	settings := store.LoadSettings()
	settings.Lang = lang
	_ = store.SaveSettings(settings)
}

// SaveSettings keeps what the settings dialog asked for.
func (a *actions) SaveSettings() {
	s := a.St.Settings
	s.Lang = i18n.Lang()
	if s.Remember {
		if form, ok := a.validate(); ok {
			s.Last = form
		}
	}
	a.St.Settings = s
	a.Do(func() { a.St.Settings = s })
	_ = store.SaveSettings(s)
	a.Do(func() {
		a.St.Status = ui.T("Saved")
		a.St.Notify(ui.T("Saved"))
	})
}

// shutdown ends what the window opened, as it closes.
func (a *App) shutdown() {
	if a.Busy() {
		return
	}
	keep := a.St.Form.KeepSession
	if keep {
		return
	}
	if a.St.Open {
		_, _ = runSelf("stop")
	}
}

// tailLog is the last lines of the log, or nothing when there is none.
func tailLog(n int) string {
	data, err := os.ReadFile(store.LogFile())
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\r\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

var _ = sync.Mutex{}
var _ = time.Now
var _ = exec.Command
