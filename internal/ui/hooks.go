// hooks.go is what the app installs into the window, so a view can ask the
// things only the app knows, and small helpers of two views.
package ui

// storeHasSecret reports whether a password is cached; the form reads it
// through the window, so a test can answer it without a file.
func storeHasSecret() bool { return hasSecret() }

// hasSecret is replaced by the app, which knows where the cache lives.
var hasSecret = func() bool { return false }

// SetHasSecret lets the window answer whether a password is cached.
func SetHasSecret(fn func() bool) { hasSecret = fn }

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
