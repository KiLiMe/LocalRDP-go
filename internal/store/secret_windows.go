//go:build windows

package store

import (
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Protect encrypts plain for this user with DPAPI, as the console does:
// any process of this account can read it, no other account can.
func Protect(plain string) (string, error) {
	b, err := crypt([]byte(plain), true)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Unprotect decrypts what Protect wrote.
func Unprotect(secret string) (string, error) {
	if secret == "" {
		return "", nil
	}
	b, err := crypt([]byte(secret), false)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(b), "\x00"), nil
}

// crypt runs DPAPI's CryptProtectData or CryptUnprotectData, in the scope
// of the current user.
func crypt(in []byte, protect bool) ([]byte, error) {
	if len(in) == 0 {
		in = []byte{0}
	}
	var pin, pout dataBlob
	pin.size = uint32(len(in))
	pin.ptr = &in[0]
	addr := uintptr(unsafe.Pointer(&pin))
	var r uintptr
	var err error
	// CRYPTPROTECT_UI_FORBIDDEN: never ask the user anything.
	if protect {
		r, _, err = procCryptProtect.Call(addr, 0, 0, 0, 0, 0x1, uintptr(unsafe.Pointer(&pout)))
	} else {
		r, _, err = procCryptUnprotect.Call(addr, 0, 0, 0, 0, 0x1, uintptr(unsafe.Pointer(&pout)))
	}
	if r == 0 {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(pout.ptr)))
	out := make([]byte, pout.size)
	copy(out, unsafe.Slice(pout.ptr, pout.size))
	return out, nil
}

type dataBlob struct {
	size uint32
	ptr  *byte
}

var (
	crypt32            = windows.NewLazySystemDLL("crypt32.dll")
	procCryptProtect   = crypt32.NewProc("CryptProtectData")
	procCryptUnprotect = crypt32.NewProc("CryptUnprotectData")
)

// LoadSecret is the cached password, empty when none is cached or it cannot
// be read: a password that isn't there is not an error the user must see.
func LoadSecret() string {
	b, err := os.ReadFile(SecretFile())
	if err != nil || len(b) == 0 {
		return ""
	}
	if s, err := Unprotect(string(b)); err == nil {
		return s
	}
	return ""
}

// SaveSecret caches the password for this user.
func SaveSecret(plain string) error {
	b, err := Protect(plain)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	return os.WriteFile(SecretFile(), []byte(b), 0o600)
}

// ForgetSecret drops the cached password.
func ForgetSecret() error {
	err := os.Remove(SecretFile())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// HasSecret reports whether a password is cached.
func HasSecret() bool {
	st, err := os.Stat(SecretFile())
	return err == nil && st.Size() > 0
}
