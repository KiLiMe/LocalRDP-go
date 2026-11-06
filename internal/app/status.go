package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"localrdp/internal/win32"
)

// parseSessionID reads the child session of a status report.
func parseSessionID(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ChildSessionId") {
			continue
		}
		_, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			return value
		}
	}
	return ""
}

// hasPrivileges reports whether the process token holds both privileges
// needed to launch a process in the child session, enabled or not.
func hasPrivileges() bool {
	var assign, tcb windows.LUID
	if windows.LookupPrivilegeValue(nil, win32.UTF16Ptr("SeAssignPrimaryTokenPrivilege"), &assign) != nil {
		return false
	}
	if windows.LookupPrivilegeValue(nil, win32.UTF16Ptr("SeTcbPrivilege"), &tcb) != nil {
		return false
	}
	token := windows.GetCurrentProcessToken()
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &size); err != windows.ERROR_INSUFFICIENT_BUFFER {
		return false
	}
	buf := make([]byte, size)
	if windows.GetTokenInformation(token, windows.TokenPrivileges, &buf[0], size, &size) != nil {
		return false
	}
	var hasAssign, hasTCB bool
	for _, privilege := range (*windows.Tokenprivileges)(unsafe.Pointer(&buf[0])).AllPrivileges() {
		hasAssign = hasAssign || privilege.Luid == assign
		hasTCB = hasTCB || privilege.Luid == tcb
	}
	return hasAssign && hasTCB
}

// privNote is what the window says about the queried privileges.
func privNote(granted bool) string {
	if granted {
		return i18nT("Ready")
	}
	return i18nT("Missing")
}

// privScript is the script that grants them, beside the program.
func privScript() string {
	path := filepath.Join(win32.ExeDir(), "install-privileges.ps1")
	if _, err := os.Stat(path); err != nil {
		return ""
	}
	return path
}

// runElevated runs a script as administrator, which Windows asks to allow.
func runElevated(script string) int {
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	info := shellExecuteInfo{}
	info.cbSize = uint32(unsafe.Sizeof(info))
	info.fMask = 0x00000040 // SEE_MASK_NOCLOSEPROCESS
	info.lpVerb = win32.UTF16Ptr("runas")
	info.lpFile = win32.UTF16Ptr("pwsh.exe")
	info.lpParameters = win32.UTF16Ptr(`-NoLogo -NoProfile -ExecutionPolicy Bypass -File "` + script + `"`)
	info.nShow = 1
	r, _, err := shell32.NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&info)))
	if r == 0 {
		if errno, ok := err.(syscall.Errno); ok && errno == 1223 { // cancelled
			return 0
		}
		return 1
	}
	if info.hProcess == 0 {
		return 0
	}
	_, _ = syscall.WaitForSingleObject(syscall.Handle(info.hProcess), 0xFFFFFFFF)
	code := uint32(0)
	_ = syscall.GetExitCodeProcess(syscall.Handle(info.hProcess), &code)
	_ = syscall.CloseHandle(syscall.Handle(info.hProcess))
	return int(code)
}

// copyText puts s on the clipboard.
func copyText(s string) error {
	cmd := exec.Command("clip.exe")
	win32.HideWindow(cmd)
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

// stderrf writes a line to the standard error.
func stderrf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

var _ = syscall.SysProcAttr{}

// shellExecuteInfo is the SHELLEXECUTEINFOW ShellExecuteEx takes.
type shellExecuteInfo struct {
	cbSize       uint32
	fMask        uint32
	hwnd         uintptr
	lpVerb       *uint16
	lpFile       *uint16
	lpParameters *uint16
	lpDirectory  *uint16
	nShow        int32
	hInstApp     uintptr
	lpIDList     uintptr
	lpClass      *uint16
	hkeyClass    uintptr
	dwHotKey     uint32
	hIcon        uintptr
	hProcess     uintptr
}
