package rdp

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"localrdp/internal/fault"
)

// versionString is what the command line prints and what the MCP server calls
// itself; build.ps1 rewrites it at link time from mygo.json's version, so a
// bare `go build` reports "dev" rather than a version it does not have.
var versionString = "dev"

const (
	kNoSession   = 0xFFFFFFFF
	cwUseDefault = -2147483648 // CW_USEDEFAULT as LONG
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
	Scale      int // 0 = no scale asked for (the window's own pixels); otherwise 100-500
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
	// SizeMode is how the window relates to the session resolution: fit
	// (default), fixed or scale. See store.NormalizeSizeMode.
	SizeMode string
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
	EditionID          string
	BuildNumber        string
	HomeEdition        bool
	ChildSessionsOn    bool
	TermServiceRunning bool
	RdpListenerEnabled bool
	RdpControlOK       bool
	InsideChildSession bool
	Elevated           bool
	ChildSessionID     uint32
	CurrentSessionID   uint32
	TransportStatus    uint32
	TransportPipe      string
	// Fault is the code of the first thing that stops this machine, empty
	// when nothing does; NextAction says the same in prose.
	Fault      fault.Code
	NextAction string
}

func (r EnvReport) blocked() bool {
	return r.HomeEdition || !r.RdpControlOK || r.InsideChildSession || !r.TermServiceRunning
}

func (r EnvReport) ready() bool { return !r.blocked() }

var (
	kernel32           = syscall.NewLazyDLL("kernel32.dll")
	user32             = syscall.NewLazyDLL("user32.dll")
	advapi32           = syscall.NewLazyDLL("advapi32.dll")
	wtsapi32           = syscall.NewLazyDLL("wtsapi32.dll")
	shell32            = syscall.NewLazyDLL("shell32.dll")
	procLocalFree      = kernel32.NewProc("LocalFree")
	procFormatMessage  = kernel32.NewProc("FormatMessageW")
	procGetCommandLine = kernel32.NewProc("GetCommandLineW")
	procSetConsoleCP   = kernel32.NewProc("SetConsoleOutputCP")
)

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
