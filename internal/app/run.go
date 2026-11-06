package app

import (
	"os"
	"time"

	"github.com/egoist/mygo"
	myui "github.com/egoist/mygo/ui"

	"localrdp/internal/store"
	"localrdp/internal/ui"
)

// Options are the switches of the window's command line.
type Options struct {
	// Capture is a file the window is drawn into, once, then closes.
	Capture string
	// Sessions are the sessions it starts with, and Active picks one.
	Sessions []ui.Tab
	Active   int
}

// Run opens the window and returns the exit code.
func Run(opts Options) int {
	session, fresh := startupSession()
	a := newApp(session, opts)
	if fresh {
		a.St.Active = -1
	}
	a.Opts = opts
	launched := false
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:          "LocalRDP",
			Width:          1000,
			Height:         640,
			MinWidth:       760,
			MinHeight:      480,
			StateKey:       "localrdp",
			TitleBarStyle:  mygo.TitleBarHidden,
			TitleBarHeight: 40,
			Content: myui.View(func(c *myui.Context) {
				ui.View(c, &a.St)
			}),
		})
		launched = true
		a.Win = win
		win.OnClosed(a.shutdown)
		a.Do(func() {
			a.St.Open = false
			a.St.Log = tailLog(200)
		})
		a.Do((*actions)(a).RefreshStatus)
		if opts.Capture != "" {
			go captureSoon(win, opts.Capture)
		}
	})
	if err := mygo.App.Run(); err != nil {
		stderrf("localrdp: %v\n", err)
		return 1
	}
	if !launched {
		return 1
	}
	return 0
}

// captureSoon draws the window once it has settled, writes it, and closes
// it, so a screenshot can be taken without a person.
func captureSoon(win *mygo.Window, file string) {
	time.Sleep(1500 * time.Millisecond)
	if png, err := win.CapturePage(); err == nil {
		_ = os.WriteFile(file, png, 0o644)
	} else {
		stderrf("localrdp: capture: %v\n", err)
	}
	win.Close()
}

// startupSession is the session the form starts with: the last one, when
// the settings say to remember it.
func startupSession() (store.Session, bool) {
	settings := store.LoadSettings()
	if settings.Remember && !settings.Last.Empty() {
		s := settings.Last
		if s.Width <= 0 {
			s.Width = store.DefaultWidth
		}
		if s.Height <= 0 {
			s.Height = store.DefaultHeight
		}
		if s.Timeout <= 0 {
			s.Timeout = store.DefaultTimeout
		}
		return s, false
	}
	return store.Session{
		Mode:    "desktop",
		Width:   store.DefaultWidth,
		Height:  store.DefaultHeight,
		Timeout: store.DefaultTimeout,
	}, true
}
