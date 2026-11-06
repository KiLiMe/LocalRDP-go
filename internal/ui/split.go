package ui

import "strings"

func trimSpace(s string) string { return strings.TrimSpace(s) }

// modeIndex names the segment the form shows: 0 for desktop, 1 for start.
func modeIndex(mode string) int {
	if mode == "start" {
		return 1
	}
	return 0
}

// modeLabel is the mode a segment index asks for.
func modeLabel(i int) string {
	if i == 1 {
		return "start"
	}
	return "desktop"
}

// modeBadge is the badge a session's mode shows.
func modeBadge(mode string) string {
	if mode == "start" {
		return "start"
	}
	return "desktop"
}

// formMode is the mode the form asks for.
// FormMode is the mode the segments show.
func (st *State) FormMode() string { return modeLabel(st.modeIndex) }

// storeHasSecret reports whether a password is cached; the form reads it
// through the window, so a test can answer it without a file.
func storeHasSecret() bool { return hasSecret() }

// hasSecret is replaced by the app, which knows where the cache lives.
var hasSecret = func() bool { return false }

// SetHasSecret lets the window answer whether a password is cached.
func SetHasSecret(fn func() bool) { hasSecret = fn }

// ModeIndex is the segment the form shows.
func (st *State) ModeIndex() int { return st.modeIndex }

// SetMode picks the segment the form shows, from the form's mode.
func (st *State) SetMode(mode string) { st.modeIndex = modeIndex(mode) }

// ModeIndexChanged reports whether the segments asked for another mode
// since the last frame, and records the answer, so the window can turn
// the form's mode into the one the segments show.
func (st *State) ModeIndexChanged() bool {
	want := modeIndex(st.Form.Mode)
	if want == st.modeIndex {
		return false
	}
	st.Form.Mode = modeLabel(st.modeIndex)
	return true
}

// pickLang picks the language of a segment.
func pickLang(i int) string {
	if i == 1 {
		return "zh"
	}
	return "en"
}

// setLang is replaced by the app, which knows the words.
var setLang = func(string) {}

// SetLang is how the app installs the words of the language.
func SetLang(fn func(string)) { setLang = fn }
