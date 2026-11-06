// Package win32 is the little of the Windows API that the command line and
// the window both need: where this program is, how a child process is
// started without a console, and the wide strings the API takes.
package win32

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// Hidden is what a child process runs with, so no console flashes up.
const Hidden = 0x08000000

// HideWindow keeps a child process from showing a console.
func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: Hidden}
}

// ExePath is this program.
func ExePath() string {
	if path, err := os.Executable(); err == nil {
		return path
	}
	return os.Args[0]
}

// ExeDir is the folder of this program, where its scripts are.
func ExeDir() string {
	return filepath.Dir(ExePath())
}

// UTF16Ptr is s as the wide string the Windows API takes.
func UTF16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		u := syscall.StringToUTF16(s)
		return &u[0]
	}
	return p
}

// Itoa is the number as a string, as the log lines want it.
func Itoa(v int) string { return strconv.Itoa(v) }
