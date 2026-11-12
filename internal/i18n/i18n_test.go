package i18n

import (
	"testing"

	"localrdp/locales"
)

func TestTranslatedStrings(t *testing.T) {
	old := Lang()
	defer Set(old)

	Set("zh")
	if got := locales.Settings(); got != "设置" {
		t.Fatalf("Settings = %q in Chinese, want 设置", got)
	}
	Set("en")
	if got := locales.Settings(); got != "Settings" {
		t.Fatalf("Settings = %q in English, want Settings", got)
	}
}

func TestSetOnlyKeepsTheTwoLanguages(t *testing.T) {
	old := Lang()
	defer Set(old)

	Set("zh")
	Set("fr")
	if got := Lang(); got != English {
		t.Fatalf("Lang() = %q after Set(fr), want en", got)
	}
}

func TestLocaleNamesTheCatalogLanguage(t *testing.T) {
	old := Lang()
	defer Set(old)

	Set("zh")
	if got := Locale(); got != "zh-Hans" {
		t.Fatalf("Locale() = %q in Chinese, want zh-Hans", got)
	}
	Set("en")
	if got := Locale(); got != "en" {
		t.Fatalf("Locale() = %q in English, want en", got)
	}
}

func TestDetectHonoursTheEnvironment(t *testing.T) {
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_MESSAGES", "")
	t.Setenv("LANG", "zh_CN.UTF-8")
	if got := Detect(); got != "zh" {
		t.Fatalf("Detect() = %q with LANG=zh_CN, want zh", got)
	}
	t.Setenv("LANG", "en_US.UTF-8")
	if got := Detect(); got != "en" {
		t.Fatalf("Detect() = %q with LANG=en_US, want en", got)
	}
}
