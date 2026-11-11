// Package app runs the window: it gives the interface its actions, starts
// the child processes the buttons ask for, and reads what they say.
//
// The interface draws the state this package keeps; a click calls an action
// here, which changes the state on the UI thread and asks for a frame.
package app

import (
	"strings"
	"sync"

	"localrdp/internal/i18n"
	"localrdp/internal/store"
	"localrdp/internal/ui"
	"localrdp/internal/win32"

	"github.com/egoist/mygo"
)

// App is the window and what it acts on.
type App struct {
	Win  *mygo.Window
	St   ui.State
	Opts Options

	mu   sync.Mutex
	busy bool
}

// newApp makes the state of a window from what the store remembers.
func newApp(session store.Session, opts Options) *App {
	a := &App{Opts: opts}
	st := &a.St
	st.Act = (*actions)(a)
	st.Settings = store.LoadSettings()
	st.Form = session
	st.Password = ""
	st.SavePassword = st.Settings.SavePasswords
	st.TimeoutText = win32.Itoa(session.Timeout)
	st.WidthText = win32.Itoa(session.Width)
	st.HeightText = win32.Itoa(session.Height)
	// The scale stays empty until someone types one: empty means "no scale
	// asked for", which is not the same as 100. A remembered scale comes back.
	if session.Scale > 0 {
		st.ScaleText = win32.Itoa(session.Scale)
	}
	st.Sessions = opts.Sessions
	st.Active = opts.Active
	if st.Settings.Lang == "" {
		st.Settings.Lang = i18n.Detect()
	}
	installAuth(st)
	firstInstall(st)
	return a
}

// Do runs fn on the UI thread and draws the frame after it.
func (a *App) Do(fn func()) {
	w := a.Win
	if w == nil {
		fn()
		return
	}
	w.Update(func() {
		fn()
		a.Win.Invalidate()
	})
}

// Invalidate asks for a frame.
func (a *App) Invalidate() {
	if a.Win != nil {
		a.Win.Invalidate()
	}
}

// setBusy says whether a child process runs; the buttons wait for it.
func (a *App) setBusy(on bool, status string) {
	a.mu.Lock()
	a.busy = on
	a.mu.Unlock()
	a.Do(func() {
		a.St.Busy = on
		a.St.Status = status
	})
}

// Busy reports whether a child process runs.
func (a *App) Busy() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.busy
}

// appendLog adds the lines of a child process to the log the window shows,
// keeping the last 200.
func (a *App) appendLog(lines []string) {
	a.Do(func() {
		joined := strings.TrimRight(strings.Join(lines, "\n"), "\n")
		if joined == "" {
			return
		}
		if a.St.Log != "" {
			a.St.Log += "\n"
		}
		a.St.Log += joined
		if all := strings.Split(a.St.Log, "\n"); len(all) > 200 {
			a.St.Log = strings.Join(all[len(all)-200:], "\n")
		}
	})
}
