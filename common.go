package main

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

const (
	versionString = "1.0.0"
	kNoSession    = 0xFFFFFFFF
	cwUseDefault  = -2147483648 // CW_USEDEFAULT as LONG
)

// ExitCode mirrors the C++ ExitCode enum.
type ExitCode int

const (
	ExitOk ExitCode = iota
	ExitError
	ExitNotReady
	ExitConnectFailed
	ExitCancelled
	ExitConflict
)

// StartOptions mirrors StartOptions in src/common.hpp.
type StartOptions struct {
	Width      int
	Height     int
	X          int
	Y          int
	Scale      int // 0 = follow host DPI; otherwise 100-500
	TimeoutSec int
	WorkDir    string
	Title      string
	User       string
	Password   string
	Command    string // full command line; empty means interactive desktop
	Minimized  bool
	Hidden     bool
	Clipboard  bool
	Drives     bool
	ViewOnly   bool
	KeepSess   bool
	Desktop    bool
	Fullscreen bool
}

func defaultOptions() StartOptions {
	return StartOptions{
		X:          cwUseDefault,
		Y:          cwUseDefault,
		TimeoutSec: 90,
		Title:      "LocalRDP",
		Clipboard:  true,
	}
}

// EnvReport mirrors EnvReport in src/common.hpp.
type EnvReport struct {
	EditionID           string
	BuildNumber         string
	HomeEdition         bool
	ChildSessionsOn     bool
	TermServiceRunning  bool
	RdpListenerEnabled  bool
	RdpControlOK        bool
	InsideChildSession  bool
	Elevated            bool
	ChildSessionID      uint32
	CurrentSessionID    uint32
	TransportStatus     uint32
	TransportPipe       string
	NextAction          string
}

func (r EnvReport) blocked() bool {
	return r.HomeEdition || !r.RdpControlOK || r.InsideChildSession
}

func (r EnvReport) ready() bool { return !r.blocked() }

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	user32              = syscall.NewLazyDLL("user32.dll")
	user32dll           = user32
	advapi32            = syscall.NewLazyDLL("advapi32.dll")
	wtsapi32            = syscall.NewLazyDLL("wtsapi32.dll")
	ole32               = syscall.NewLazyDLL("ole32.dll")
	oleaut32            = syscall.NewLazyDLL("oleaut32.dll")
	shell32             = syscall.NewLazyDLL("shell32.dll")
	procLocalFree       = kernel32.NewProc("LocalFree")
	procFormatMessage   = kernel32.NewProc("FormatMessageW")
	procGetCommandLine  = kernel32.NewProc("GetCommandLineW")
	procSetConsoleCP    = kernel32.NewProc("SetConsoleOutputCP")
	procGetModuleHandle = kernel32.NewProc("GetModuleFileNameW")
)

func utf16Ptr(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		u := syscall.StringToUTF16(s)
		return &u[0]
	}
	return p
}

func utf16Slice(s string) []uint16 { return syscall.StringToUTF16(s) }

func u2s(p *uint16) string {
	if p == nil {
		return ""
	}
	var n int
	for q := p; ; q = (*uint16)(unsafe.Add(unsafe.Pointer(q), 2)) {
		if *q == 0 {
			break
		}
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

//itoa is a small helper used all over the log strings.
func itoa(v int) string { return fmt.Sprintf("%d", v) }

func formatWin32(err uint32) string {
	var buf unsafe.Pointer
	const flags = 0x00000100 | 0x00001000 | 0x00000200 // ALLOCATE_BUFFER|FROM_SYSTEM|IGNORE_INSERTS（0x100 必须带上，否则会把栈当地址写）
	procFormatMessage.Call(uintptr(flags), 0, uintptr(err), 0,
		uintptr(unsafe.Pointer(&buf)), 0, 0)
	msg := ""
	if buf != nil {
		msg = u2s((*uint16)(buf))
		procLocalFree.Call(uintptr(buf))
	}
	msg = strings.TrimRight(msg, "\r\n ")
	code := fmt.Sprintf("0x%08X", err)
	if msg == "" {
		return code
	}
	return code + " (" + msg + ")"
}

func formatHr(hr uint32) string { return formatWin32(hr) }

func hrFailed(hr uintptr) bool { return int32(uint32(hr)) < 0 }
