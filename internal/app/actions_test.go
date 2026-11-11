package app

import (
	"testing"

	"localrdp/internal/store"
)

func TestBadUserName(t *testing.T) {
	cases := map[string]bool{
		`DESKTOP-1\John`: false,
		"anna":           false,
		"JOHN":           false,
		"a`b":            false,
		`a"b`:            true,
		"a\nb":           true,
		"a\rb":           true,
	}
	for in, want := range cases {
		if got := badUserName(in); got != want {
			t.Errorf("badUserName(%q) = %v, want %v", in, got, want)
		}
	}
}

// with copies a session and applies one change, so a case only spells out
// what it is about.
func with(s store.Session, change func(*store.Session)) store.Session {
	change(&s)
	return s
}

// validate is what the Open button asks: it trims the text, forces the
// console mode, and names the field at fault.
func TestValidateChecksTheForm(t *testing.T) {
	good := store.Session{Name: " work ", Width: 1280, Height: 800, Timeout: 90, Scale: 150}
	cases := []struct {
		name  string
		form  store.Session
		ok    bool
		errOn string
	}{
		{"a filled form passes", good, true, ""},
		{"the timeout is at least 5", with(good, func(s *store.Session) { s.Timeout = 4 }), false, "timeout"},
		{"the width is at least 320", with(good, func(s *store.Session) { s.Width = 319 }), false, "width"},
		{"the height is at least 200", with(good, func(s *store.Session) { s.Height = 199 }), false, "width"},
		{"a quote in the user name is refused", with(good, func(s *store.Session) { s.User = `a"b` }), false, "user"},
		{"a scale under 100 is refused", with(good, func(s *store.Session) { s.Scale = 50 }), false, "scale"},
		{"a scale over 500 is refused", with(good, func(s *store.Session) { s.Scale = 501 }), false, "scale"},
		{"an empty scale is fine", with(good, func(s *store.Session) { s.Scale = 0 }), true, ""},
		{"500 is the top scale", with(good, func(s *store.Session) { s.Scale = 500 }), true, ""},
		{"100 is the lowest scale", with(good, func(s *store.Session) { s.Scale = 100 }), true, ""},
	}
	for _, c := range cases {
		a := &actions{}
		a.St.Form = c.form
		got, ok := a.validate()
		if ok != c.ok || a.St.FormErrOn != c.errOn {
			t.Errorf("%s: ok = %v, errOn = %q, want %v / %q", c.name, ok, a.St.FormErrOn, c.ok, c.errOn)
		}
		if !ok && a.St.FormErr == "" {
			t.Errorf("%s: no message for the field at fault", c.name)
		}
		if ok {
			if got.Mode != "desktop" || got.Name != "work" {
				t.Errorf("%s: session = %+v, want the desktop mode and the trimmed name", c.name, got)
			}
		}
	}
}
