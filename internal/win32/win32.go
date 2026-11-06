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
	procAttachConsole            = kernel32.NewProc("AttachConsole")
	procSetStdHandle             = kernel32.NewProc("SetStdHandle")
)

// swRestore is ShowWindow's SW_RESTORE.
const swRestore = 9

// FreeConsole drops the console this process was given, so a window that is
// started by double clicking does not come with a black console behind it.
// It only matters for a console subsystem build: the released exe is linked
// with -H windowsgui and is never given a console in the first place.
func FreeConsole() {
	procFreeConsole.Call()
}

// attachParentProcess is AttachConsole's ATTACH_PARENT_PROCESS.
const attachParentProcess = ^uint32(0)

// AttachStdio connects the standard streams to the terminal that started this
// program and reports whether it did. The released exe is a GUI subsystem
// program (-H windowsgui), so Windows gives it no console at all and
// `localrdp status` would print into nothing; a program that was double
// clicked has no parent console and this does nothing. Streams that were
// redirected to a pipe or a file are left alone: the handles are already the
// ones the caller handed over.
func AttachStdio() bool {
	if h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE); err == nil && h != 0 && h != syscall.InvalidHandle {
		return false
	}
	if r, _, _ := procAttachConsole.Call(uintptr(attachParentProcess)); r == 0 {
		return false
	}
	rebind(syscall.STD_OUTPUT_HANDLE, "CONOUT$", "stdout")
	rebind(syscall.STD_ERROR_HANDLE, "CONOUT$", "stderr")
	rebind(syscall.STD_INPUT_HANDLE, "CONIN$", "stdin")
	return true
}

// rebind opens name and makes it the standard stream id, for this process and
// for the children it starts.
func rebind(id int, name, label string) {
	fd, err := syscall.Open(name, syscall.O_RDWR, 0)
	if err != nil {
		return
	}
	file := os.NewFile(uintptr(fd), label)
	procSetStdHandle.Call(uintptr(id), uintptr(fd))
	switch id {
	case syscall.STD_OUTPUT_HANDLE:
		os.Stdout = file
	case syscall.STD_ERROR_HANDLE:
		os.Stderr = file
	case syscall.STD_INPUT_HANDLE:
		os.Stdin = file
	}
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
