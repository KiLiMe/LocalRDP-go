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
	"unsafe"
)

// Hidden is what a child process runs with, so no console flashes up.
const Hidden = 0x08000000

// HideWindow keeps a child process from showing a console.
func HideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: Hidden}
}

// NoConsole starts a child process without a console window, and leaves its
// first ShowWindow alone. HideWindow sets STARTF_USESHOWWINDOW/SW_HIDE, which
// Windows applies to the first window the child shows itself - a window drew
// by the child would stay invisible.
func NoConsole(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: Hidden}
}

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowThreadProcessID = user32.NewProc("GetWindowThreadProcessId")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procShowWindow               = user32.NewProc("ShowWindow")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procSwitchToThisWindow       = user32.NewProc("SwitchToThisWindow")
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procFreeConsole              = kernel32.NewProc("FreeConsole")
)

// swRestore is ShowWindow's SW_RESTORE.
const swRestore = 9

// FreeConsole drops the console this process was given, so a window that is
// started by double clicking does not come with a black console behind it.
func FreeConsole() {
	procFreeConsole.Call()
}

// BringToFront shows and focuses the host window of a child process. The
// process is named instead of the title, because two sessions can share one.
func BringToFront(pid int) bool {
	if pid <= 0 {
		return false
	}
	target := uint32(pid)
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		var owner uint32
		procGetWindowThreadProcessID.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner != target {
			return 1
		}
		var buf [64]uint16
		n, _, _ := procGetClassNameW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if n == 0 || classString(buf[:n]) != HostClassName {
			return 1
		}
		found = hwnd
		return 0
	})
	procEnumWindows.Call(cb, 0)
	if found == 0 {
		return false
	}
	procShowWindow.Call(found, swRestore)
	procSetForegroundWindow.Call(found)
	// Windows often refuses SetForegroundWindow to a process that is not in
	// front already; SwitchToThisWindow still brings the window up.
	procSwitchToThisWindow.Call(found, 1)
	return true
}

// HostClassName is the window class of the remote desktop host window.
const HostClassName = "LocalRDP.Host"

// classString is the wide characters n counted by GetClassNameW as a string.
func classString(buf []uint16) string {
	return syscall.UTF16ToString(buf)
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
