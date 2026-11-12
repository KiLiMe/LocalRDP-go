package app

import (
	"github.com/egoist/mygo"

	"localrdp/internal/i18n"
	"localrdp/internal/store"
	"localrdp/internal/ui"
)

// installAuth lets the interface ask the store and the words, which it
// does not know itself: the window draws, the app knows where things are.
func installAuth(st *ui.State) {
	i18n.Set(st.Settings.Lang)
	ui.SetHasSecret(store.HasSecret)
	ui.SetLang(setLanguage)
}

// setLanguage says the interface's language: to the catalogs, and to MyGo,
// whose own menus and controls say their items in it too.
func setLanguage(lang string) {
	i18n.Set(lang)
	mygo.App.SetLocale(i18n.Locale())
}

// firstInstall remembers the last session on a fresh install, so the form
// opens with what was left rather than a default.
func firstInstall(st *ui.State) {
	s := st.Settings
	if s.Last.Empty() {
		s.Remember = true
		s.Last = st.Form
		_ = store.SaveSettings(s)
		st.Settings = s
	}
}
