package app

import (
	"localrdp/internal/i18n"
	"localrdp/internal/store"
	"localrdp/internal/ui"
)

// installAuth lets the interface ask the store and the words, which it
// does not know itself: the window draws, the app knows where things are.
func installAuth(st *ui.State) {
	i18n.Set(st.Settings.Lang)
	ui.SetHasSecret(store.HasSecret)
	ui.SetLang(i18n.Set)
}

// firstInstall remembers the last session on a fresh install, so the form
// opens with what was left rather than a default.
func firstInstall(st *ui.State) {
	s := st.Settings
	if s.Last.Mode == "" {
		s.Remember = true
		s.Last = st.Form
		_ = store.SaveSettings(s)
		st.Settings = s
	}
}
