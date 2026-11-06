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
	// SizeMode says what the host window does with its size: SizeFit
	// (default) lets the window decide the session resolution, SizeFixed
	// pins both to Width×Height, SizeScale keeps the resolution and scales
	// the picture into the window.
	SizeMode string `json:"size_mode,omitempty"`
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

// Size modes of the host window, as --size spells them.
const (
	// SizeFit lets the window decide: dragging it re-negotiates the
	// session resolution (the default).
	SizeFit = "fit"
	// SizeFixed pins the window and the session to Width×Height, and takes
	// the resize border away.
	SizeFixed = "fixed"
	// SizeScale keeps the session resolution and scales the picture into
	// the window.
	SizeScale = "scale"
)

// SizeModes is the order the form offers the modes in.
func SizeModes() []string { return []string{SizeFixed, SizeFit, SizeScale} }

// NormalizeSizeMode maps a name from the command line or a settings file to
// one of the modes; empty and unknown mean SizeFit.
func NormalizeSizeMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case SizeFixed, "fix", "locked", "lock":
		return SizeFixed
	case SizeScale, "scaled", "zoom", "stretch":
		return SizeScale
	}
	return SizeFit
}

// SizeModeIndex is the position of a mode in SizeModes.
func SizeModeIndex(s string) int {
	m := NormalizeSizeMode(s)
	for i, v := range SizeModes() {
		if v == m {
			return i
		}
	}
	return 1
}

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
	if m := NormalizeSizeMode(s.SizeMode); m != SizeFit {
		args = append(args, "--size", m)
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

// CommandFile is where `localrdp click` leaves a button the window should
// press. The window removes the file once it has done so.
func CommandFile() string { return filepath.Join(Dir(), "command.txt") }

// GuiCommands are the buttons of the window that can be pressed from
// outside: the name `localrdp click` takes for each.
var GuiCommands = []string{
	"open",            // 打开子会话
	"status",          // 状态
	"stop",            // 注销子会话
	"privilege",       // 授予特权
	"forget-password", // 清理密码
	"data-folder",     // 打开数据目录
	"reconnect",       // 会话页的重连
	"show",            // 会话页的显示窗口
	"close-session",   // 会话页的关闭会话
	"refresh",         // 刷新环境
	"clear-log",       // 清空日志
	"copy-log",        // 复制日志
}

// KnownGuiCommand says whether the window has a button of that name.
func KnownGuiCommand(name string) bool {
	for _, c := range GuiCommands {
		if c == name {
			return true
		}
	}
	return false
}

// WriteGuiCommand asks the running window to press a button. The name is
// checked against GuiCommands first.
func WriteGuiCommand(name string) error {
	if !KnownGuiCommand(name) {
		return errUnknownCommand{name}
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(CommandFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(name + "\n")
	return err
}

type errUnknownCommand struct{ name string }

func (e errUnknownCommand) Error() string { return "unknown command: " + e.name }

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
