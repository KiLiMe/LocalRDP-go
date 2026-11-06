// Package ui draws the window: the band with its menus, the console of the
// session form, the panel of what is open, and the log.
//
// It draws a State and does nothing else: a click calls an Actions method,
// which changes the state and asks for a frame. The state lives here, so
// the package that owns the window is kept free of layout.
package ui

import (
	myui "github.com/egoist/mygo/ui"

	"localrdp/internal/i18n"
	"localrdp/internal/store"
)

// T is the text of the interface in the chosen language.
func T(s string) string { return i18n.T(s) }

// Actions is what a click does: the window, as the interface sees it.
type Actions interface {
	// OpenSession opens the session of the form, as it is now.
	OpenSession()
	// RefreshStatus asks what is open and what is granted.
	RefreshStatus()
	// LogOff ends the child session the window is showing.
	LogOff()
	// GrantPrivileges runs the privileged script, which asks for UAC.
	GrantPrivileges()
	// ForgetPassword drops the cached password.
	ForgetPassword()
	// Shortcut puts one on the desktop that opens this window.
	Shortcut()
	// OpenDataFolder shows %LOCALAPPDATA%\LocalRDP.
	OpenDataFolder()
	// ShowSession brings a session's window up.
	ShowSession(id string) bool
	// CloseSession ends one and closes its tab.
	CloseSession(id string)
	// FocusSession picks a tab.
	FocusSession(index int)
	// ClearLog drops the log the window shows.
	ClearLog()
	// CopyLog puts the log on the clipboard.
	CopyLog()
	// SetLang switches the language and remembers it.
	SetLang(lang string)
	// SaveSettings writes what the app remembers.
	SaveSettings()
	// Count asks whether any process is doing something, so the dialogs
	// know whether they may close yet.
	Busy() bool
	// Do runs fn on the UI thread and redraws.
	Do(fn func())
	Invalidate()
	Quit()
}

// State is what the window draws, and what a click acts on.
type State struct {
	// Act is the window: what the buttons call.
	Act Actions

	// Form is the session the console opens, as the user typed it: the
	// password is not part of it, as it is cached, or typed for one run.
	Form store.Session
	// Password is what the user typed this run; empty means the cached one
	// is used. SavePassword keeps it, encrypted, when the session opens.
	Password     string
	SavePassword bool
	// FormErr names what the form complains about, and FormErrOn the field
	// it belongs to, as the form shows it.
	FormErr   string
	FormErrOn string

	// Sessions are what this window opened, the console a tab of its own.
	Sessions []Tab
	// Active is the tab in front, -1 for none: the console.
	Active int

	// Open is whether a child session runs, SessionID its id as status
	// names it, empty when none does.
	Open      bool
	SessionID string
	// PrivOK is whether this process holds the two privileges start needs,
	// PrivNote what to say about it.
	PrivOK   bool
	PrivNote string

	// Log is the tail of the child processes' log, and LogBusy whether it
	// is being read.
	Log string

	// Settings is what the app remembers about itself, and SettingsOpen
	// whether its dialog shows. Dark is the theme, LangLabel the language
	// picker's choice as it shows it.
	Settings     store.Settings
	SettingsOpen bool
	About        bool

	// SessionsOpen and CloseOpen show the lists of what is open.
	SessionsOpen bool
	CloseOpen    bool

	// Status is what the window says it is doing, in one line.
	Status string
	// Busy is whether a child process runs, so the buttons wait for it.
	Busy bool

	// modeIndex is the segment the mode card shows, and the text fields
	// that are not part of the form are the numbers, kept as typed so a
	// half-written number does not turn into a zero.
	modeIndex   int
	WidthText   string
	HeightText  string
	TimeoutText string
	// WindowW and WindowH are the window's size, measured by view().
	WindowW, WindowH float32
	// ModeText is the mode the form last asked for, as the badge shows it.
	ModeText string
	// Dark is the theme in front, as the window draws it.
	Dark bool
	// quitting is set once the window has decided to close, so the
	// question is not asked twice.
	quitting bool
}

// Tab is a session the window opened: a child window it can bring up and
// close again.
type Tab struct {
	Session store.Session
	// Title is the host window's title, as the child reports it.
	Title string
	// Open is false once the child process has ended.
	Open bool
	// PID of the child process, so it can be brought to the front.
	PID int
	// Status is what the child said it was doing, when it ended.
	Status string
}

// Label is the tab's name.
func (t Tab) Label() string { return t.Session.Label() }

// View builds a frame of the window.
func View(c *myui.Context, st *State) { st.view(c) }
