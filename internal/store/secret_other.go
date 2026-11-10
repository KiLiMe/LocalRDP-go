//go:build !windows

package store

import "errors"

// errNoDPAPI is what the platforms without DPAPI answer: the console and
// the child sessions are Windows only.
var errNoDPAPI = errors.New("store: DPAPI is Windows only")

func Protect(string) (string, error)   { return "", errNoDPAPI }
func Unprotect(string) (string, error) { return "", errNoDPAPI }
func LoadSecret() string               { return "" }
func SaveSecret(string) error          { return errNoDPAPI }
func ForgetSecret() error              { return nil }
func HasSecret() bool                  { return false }
