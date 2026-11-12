// Package i18n picks the language the interface speaks. The messages
// themselves are the catalogs in ../locales, which `mygo generate` turned
// into the typed accessors the code calls: this package only says which of
// their languages to read, so a string is never looked up by its English.
package i18n

import (
	"os"
	"strings"
	"sync"

	mygoi18n "github.com/egoist/mygo/i18n"
)

const (
	// English and Chinese are the languages to pick between; Chinese is
	// Simplified in the catalogs, which name it "zh-Hans".
	English = "en"
	Chinese = "zh"
	// ChineseTag is Chinese as a catalog names it.
	ChineseTag = "zh-Hans"
)

var (
	mu   sync.RWMutex
	lang = English
)

func init() {
	// The catalogs say their messages in the language picked here.
	mygoi18n.SetLocaleFunc(Locale)
}

// Set picks the language: "zh" for Chinese, anything else English.
func Set(l string) {
	mu.Lock()
	if l == Chinese {
		lang = Chinese
	} else {
		lang = English
	}
	mu.Unlock()
	// MyGo's own root package installs its language in an init, which may
	// run after this package's: say ours again, and on every change, so the
	// catalogs follow the settings rather than the system.
	mygoi18n.SetLocaleFunc(Locale)
}

// Lang is the language in use.
func Lang() string {
	mu.RLock()
	defer mu.RUnlock()
	return lang
}

// Locale is Lang as a BCP 47 tag, the form the catalogs name a language in.
func Locale() string {
	if Lang() == Chinese {
		return ChineseTag
	}
	return English
}

// Detect is the language the system asks for, used until the user picks
// one: Chinese on a Chinese Windows, English elsewhere.
func Detect() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		v := strings.ToLower(os.Getenv(k))
		if strings.HasPrefix(v, "zh") {
			return Chinese
		}
		if v != "" {
			return English
		}
	}
	return systemLang()
}
