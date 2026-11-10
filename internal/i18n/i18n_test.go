package i18n

import "testing"

func TestTranslatedStrings(t *testing.T) {
	old := Lang()
	defer Set(old)

	Set("zh")
	if got := T("Settings"); got != "设置" {
		t.Fatalf(`T("Settings") = %q in Chinese, want 设置`, got)
	}
	Set("en")
	if got := T("Settings"); got != "Settings" {
		t.Fatalf(`T("Settings") = %q in English, want Settings`, got)
	}
	if got := T("a string no one translated"); got != "a string no one translated" {
		t.Fatalf("an unknown string was changed: %q", got)
	}
	if got := T(""); got != "" {
		t.Fatalf("the empty string changed: %q", got)
	}
}

func TestSetOnlyKeepsTheTwoLanguages(t *testing.T) {
	old := Lang()
	defer Set(old)

	Set("zh")
	Set("fr")
	if got := Lang(); got != "en" {
		t.Fatalf("Lang() = %q after Set(fr), want en", got)
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
