package ui

import (
	"strconv"
	"testing"

	"localrdp/internal/store"
)

// TestHasSessionFollowsTheWindow: 回到会话 只能指向还开着的窗口——标签留着是为了
// 看退出码，窗口没了就不算会话。
func TestHasSessionFollowsTheWindow(t *testing.T) {
	open := Tab{Open: true}
	done := Tab{Open: false}
	cases := []struct {
		name string
		st   State
		want bool
	}{
		{"no tabs at all", State{Active: -1}, false},
		{"the active tab is closed", State{Sessions: []Tab{done}, Active: 0}, false},
		{"the active tab is open", State{Sessions: []Tab{open}, Active: 0}, true},
		{"active past the end", State{Sessions: []Tab{open}, Active: 3}, false},
	}
	for _, c := range cases {
		if got := c.st.hasSession(); got != c.want {
			t.Errorf("%s: hasSession() = %v, want %v", c.name, got, c.want)
		}
	}
}

// formNumbers is the only place the typed text becomes numbers: empty keeps
// what the form had, garbage falls back to the store's defaults, the timeout
// never drops below 5 and an empty scale stays "no scale asked for".
func TestFormNumbers(t *testing.T) {
	cases := []struct {
		name  string
		state State
		want  store.Session
	}{
		{
			name:  "empty text keeps the form's numbers",
			state: State{Form: store.Session{Width: 1600, Height: 900, Timeout: 60}},
			want:  store.Session{Width: 1600, Height: 900, Timeout: 60},
		},
		{
			name:  "typed numbers win",
			state: State{WidthText: "1366", HeightText: "768", TimeoutText: "45"},
			want:  store.Session{Width: 1366, Height: 768, Timeout: 45},
		},
		{
			name:  "spaces around a number are fine",
			state: State{WidthText: " 1024 ", HeightText: "\t600\n"},
			want:  store.Session{Width: 1024, Height: 600, Timeout: store.DefaultTimeout},
		},
		{
			name:  "garbage falls back to the defaults",
			state: State{WidthText: "abc", HeightText: "-1", TimeoutText: "0"},
			want: store.Session{Width: store.DefaultWidth, Height: store.DefaultHeight,
				Timeout: store.DefaultTimeout},
		},
		{
			name:  "a timeout below 5 is raised to 5",
			state: State{TimeoutText: "3"},
			want:  store.Session{Width: store.DefaultWidth, Height: store.DefaultHeight, Timeout: 5},
		},
		{
			name:  "an empty scale asks for none",
			state: State{ScaleText: ""},
			want:  store.Session{Width: store.DefaultWidth, Height: store.DefaultHeight, Timeout: store.DefaultTimeout},
		},
		{
			name:  "a typed scale is kept",
			state: State{ScaleText: "150"},
			want: store.Session{Width: store.DefaultWidth, Height: store.DefaultHeight,
				Timeout: store.DefaultTimeout, Scale: 150},
		},
		{
			name:  "zero and garbage mean no scale",
			state: State{ScaleText: "0"},
			want:  store.Session{Width: store.DefaultWidth, Height: store.DefaultHeight, Timeout: store.DefaultTimeout},
		},
	}
	for _, c := range cases {
		st := c.state
		st.formNumbers()
		if st.Form != c.want {
			t.Errorf("%s: form = %+v, want %+v", c.name, st.Form, c.want)
		}
	}
}

// The text the user typed stays: only the numbers are derived from it.
func TestFormNumbersKeepsTheText(t *testing.T) {
	st := &State{Form: store.Session{Width: 1600, Height: 900, Timeout: 60}}
	st.formNumbers()
	if st.WidthText != strconv.Itoa(1600) || st.HeightText != strconv.Itoa(900) || st.TimeoutText != strconv.Itoa(60) {
		t.Errorf("text = %q %q %q, want 1600 900 60", st.WidthText, st.HeightText, st.TimeoutText)
	}
	st = &State{WidthText: "abc", TimeoutText: "1"}
	st.formNumbers()
	if st.WidthText != "abc" || st.TimeoutText != "1" {
		t.Errorf("typed text was rewritten: %q %q", st.WidthText, st.TimeoutText)
	}
}
