//go:build !windows

package i18n

// systemLang is English where there is no Windows UI language to ask.
func systemLang() string { return "en" }
