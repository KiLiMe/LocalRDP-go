// Package store keeps what the app remembers between runs: the sessions it
// opens when asked to, what it remembers about itself, and the password,
// encrypted with DPAPI.
//
// It is a leaf: it imports nothing of the app, the window or the session
// code, so the window and the actions can both depend on it.
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"localrdp/internal/i18n"
)

// Session is what the console opens: a child desktop, or one program in
// the child session.
type Session struct {
	// Name is the session as this desktop sees it: "Desktop" for the whole
	// child desktop, "Notepad" for a program.
	Name string `json:"name"`
	// Mode is the verb of earlier versions: the window always opens the
	// child desktop now, and Program starts on it. Kept so a settings file
	// from before still reads.
	Mode string `json:"mode,omitempty"`
	// Program is the command line start runs, empty for desktop.
	Program string `json:"program,omitempty"`
	// User is the account of the child session, as DOMAIN\user.
	User string `json:"user,omitempty"`
	// Width and Height are the host window's client size; 0 asks for the
	// default of about 5/6 of the monitor.
	Width  int `json:"width,omitempty"`
	Height int `json:"height,omitempty"`
	// Title of the host window; empty for the default.
	Title string `json:"title,omitempty"`
	// FullScreen, Hidden and Minimized are the window's first state.
	FullScreen bool `json:"fullscreen,omitempty"`
	Hidden     bool `json:"hidden,omitempty"`
	Minimized  bool `json:"minimized,omitempty"`
	// Timeout is how long the connection may take, in seconds.
	Timeout int `json:"timeout,omitempty"`
	// KeepSession leaves the child session up when the host window ends.
	KeepSession bool `json:"keep_session,omitempty"`
}

// DefaultTimeout is the connection timeout the form offers, as the command
// line defaults to.
const DefaultTimeout = 90

// Empty is whether the session holds nothing, so a settings file from an
// older install does not leave the form blank.
func (s Session) Empty() bool {
	return s.Mode == "" && s.Name == "" && s.Program == "" && s.User == "" && s.Title == ""
}

// Label is the session as the window names it.
func (s Session) Label() string {
	if strings.TrimSpace(s.Name) == "" {
		return s.Program
	}
	return s.Name
}

// ID names the session, the same for the same program and account, so the
// window knows which row is open.
func (s Session) ID() string {
	return strings.ToLower(strings.TrimSpace(s.Program) + "|" + strings.ToLower(s.User))
}

// Args is the command line the session runs, as the child process gets it:
// no password, which travels in the environment.
//
// The verb is always desktop: the window shows the child desktop, and the
// program, when there is one, starts on it and leaves it up when it exits.
func (s Session) Args() []string {
	args := []string{"desktop"}
	if s.User != "" {
		args = append(args, "--user", s.User, "--password-env", PasswordEnv)
	}
	if t := strings.TrimSpace(s.Title); t != "" {
		args = append(args, "--title", t)
	} else if label := strings.TrimSpace(s.Label()); label != "" {
		args = append(args, "--title", label)
	}
	if s.Width > 0 {
		args = append(args, "--width", fmt.Sprint(s.Width))
	}
	if s.Height > 0 {
		args = append(args, "--height", fmt.Sprint(s.Height))
	}
	if s.Timeout > 0 {
		args = append(args, "--timeout", fmt.Sprint(s.Timeout))
	}
	if s.FullScreen {
		args = append(args, "--fullscreen")
	}
	if s.Hidden {
		args = append(args, "--hidden")
	}
	if s.Minimized {
		args = append(args, "--minimized")
	}
	if s.KeepSession {
		args = append(args, "--keep-session")
	}
	if p := strings.TrimSpace(s.Program); p != "" {
		args = append(args, "--")
		args = append(args, SplitCommandLine(p)...)
	}
	return args
}

// PasswordEnv is the variable the password travels in, so it stays out of
// the command line, where every process can read it.
const PasswordEnv = "LRDP_PW"

// SplitCommandLine splits a command line the way Windows does: on spaces
// outside quotes, with backslashes before a quote escaping it.
func SplitCommandLine(s string) []string {
	var (
		out   []string
		cur   strings.Builder
		quote bool
		has   bool
	)
	flush := func() {
		if has {
			out = append(out, cur.String())
			cur.Reset()
			has = false
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\\' && i+1 < len(s) && (s[i+1] == '"' || s[i+1] == '\\'):
			cur.WriteByte(s[i+1])
			has = true
			i++
		case c == '"':
			quote = !quote
			has = true
		case (c == ' ' || c == '\t') && !quote:
			flush()
		default:
			cur.WriteByte(c)
			has = true
		}
	}
	flush()
	return out
}

// Settings is what the app remembers about itself.
type Settings struct {
	// Lang is the interface's language: "zh" or "en".
	Lang string `json:"lang"`
	// SavePasswords keeps the password of a session, encrypted.
	SavePasswords bool `json:"save_passwords"`
	// Remember keeps the sessions of the last run and opens them again.
	Remember bool `json:"remember"`
	// Last is the form as it was left: the arguments, never the password.
	Last Session `json:"last"`
}

// DefaultSettings is a fresh install: Chinese where the system is, keeping
// nothing of the form but its size.
func DefaultSettings() Settings {
	return Settings{
		Lang:          i18n.Detect(),
		SavePasswords: true,
		Remember:      true,
		Last:          Session{Mode: "desktop", Title: "LocalRDP", Timeout: DefaultTimeout},
	}
}

// Dir is where the app keeps its files: %LOCALAPPDATA%\LocalRDP.
func Dir() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "LocalRDP")
	}
	if d, err := os.UserConfigDir(); err == nil {
		return filepath.Join(d, "LocalRDP")
	}
	return "LocalRDP"
}

// SettingsFile is where the settings live.
func SettingsFile() string { return filepath.Join(Dir(), "settings.json") }

// SessionsFile is where the sessions of the last run live.
func SessionsFile() string { return filepath.Join(Dir(), "sessions.json") }

// SecretFile is the cached password, encrypted with DPAPI for this user.
func SecretFile() string { return filepath.Join(Dir(), "cred.dat") }

// LogFile is where the child processes' log goes.
func LogFile() string { return filepath.Join(Dir(), "localrdp.log") }

// LoadSettings reads the settings, and a fresh set when there are none or
// they can't be read: a broken file must not keep the window shut.
func LoadSettings() Settings {
	var s Settings
	if err := readJson(SettingsFile(), &s); err != nil {
		return DefaultSettings()
	}
	d := DefaultSettings()
	if s.Lang == "" {
		s.Lang = d.Lang
	}
	if s.Last.Empty() {
		s.Last = d.Last
	}
	if s.Last.Timeout == 0 {
		s.Last.Timeout = DefaultTimeout
	}
	return s
}

// SaveSettings writes the settings, making the folder it needs.
func SaveSettings(s Settings) error { return writeJson(SettingsFile(), s) }

// LoadSessions reads the sessions of the last run.
func LoadSessions() []Session {
	var f struct {
		Sessions []Session `json:"sessions"`
	}
	if err := readJson(SessionsFile(), &f); err != nil {
		return nil
	}
	return f.Sessions
}

// SaveSessions writes the sessions of this run.
func SaveSessions(list []Session) error {
	return writeJson(SessionsFile(), struct {
		Sessions []Session `json:"sessions"`
	}{list})
}

// readJson decodes the file at path into v.
func readJson(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// writeJson writes v to path as JSON, through a temporary file so a crash
// leaves the old one whole.
func writeJson(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(b, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// The defaults of a session the form has not filled in.
const (
	DefaultWidth  = 1280
	DefaultHeight = 800
)
