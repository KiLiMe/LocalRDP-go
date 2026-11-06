package store

import (
	"strings"
	"testing"
)

func TestSplitCommandLine(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"notepad.exe", []string{"notepad.exe"}},
		{`"C:\Program Files\x.exe" a`, []string{`C:\Program Files\x.exe`, "a"}},
		{"a  b", []string{"a", "b"}},
		{`a\"b`, []string{`a"b`}},
	}
	for _, c := range cases {
		got := SplitCommandLine(c.in)
		if len(got) != len(c.want) {
			t.Errorf("SplitCommandLine(%q) = %q, want %q", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("SplitCommandLine(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestSessionArgsKeepThePasswordOffTheCommandLine(t *testing.T) {
	s := Session{Program: "notepad.exe", User: `DOM\me`, Timeout: 30}
	args := s.Args()
	joined := strings.Join(args, " ")
	found := false
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "--password-env" && args[i+1] == PasswordEnv {
			found = true
		}
	}
	if !found {
		t.Fatalf("the args do not ask for the password from %s: %q", PasswordEnv, joined)
	}
	if strings.Contains(joined, "--password ") {
		t.Fatalf("the args carry the password itself: %q", joined)
	}
	if !strings.Contains(joined, "-- notepad.exe") {
		t.Fatalf("the program is not after --: %q", joined)
	}
	if args[0] != "desktop" {
		t.Fatalf("the verb is %q, want desktop: %q", args[0], joined)
	}
}

func TestSessionArgsWithoutAProgramIsTheDesktop(t *testing.T) {
	s := Session{User: `DOM\me`, Timeout: 30}
	if got := strings.Join(s.Args(), " "); strings.Contains(got, " -- ") {
		t.Fatalf("a session with no program still asks for one: %q", got)
	}
}

func TestSessionIDFollowsTheProgram(t *testing.T) {
	a := Session{Program: "notepad.exe", User: `DOM\me`}
	b := Session{Mode: "start", Program: "notepad.exe", User: `dom\ME`}
	if a.ID() != b.ID() {
		t.Fatalf("the same program and account name two sessions: %q, %q", a.ID(), b.ID())
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())

	in := Settings{
		Lang:          "en",
		SavePasswords: true,
		Remember:      true,
		Last:          Session{Program: "notepad.exe", Timeout: 30},
	}
	if err := SaveSettings(in); err != nil {
		t.Fatal(err)
	}
	got := LoadSettings()
	if got.Lang != in.Lang || !got.Remember || !got.SavePasswords {
		t.Fatalf("settings changed: %+v", got)
	}
	if got.Last.Program != in.Last.Program || got.Last.Timeout != 30 {
		t.Fatalf("the last session changed: %+v", got.Last)
	}
}

func TestLoadSettingsFallsBackWhenMissing(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())
	got := LoadSettings()
	if got.Last.Mode != "desktop" {
		t.Fatalf("a fresh install does not open a desktop: %+v", got.Last)
	}
	if got.Last.Timeout != DefaultTimeout {
		t.Fatalf("a fresh install has timeout %d, want %d", got.Last.Timeout, DefaultTimeout)
	}
}
