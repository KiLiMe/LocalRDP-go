package app

import "testing"

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
