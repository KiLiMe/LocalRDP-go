//go:build windows

package i18n

import "syscall"

var procGetUserDefaultUILanguage = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

// systemLang is the language of the user's Windows: zh for Chinese, en for
// everything else, so the first run of a Chinese machine opens in Chinese.
func systemLang() string {
	langID, _, _ := procGetUserDefaultUILanguage.Call()
	// The primary language id is the low 10 bits: 4 is Chinese, in either
	// script.
	if uint16(langID)&0x3ff == 0x04 {
		return Chinese
	}
	return English
}
