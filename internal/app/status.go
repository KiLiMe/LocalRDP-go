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
	"golang.org/x/sys/windows/registry"

	"localrdp/internal/store"
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
	if windows.LookupPrivilegeValue(nil, utf16Ptr("SeAssignPrimaryTokenPrivilege"), &assign) != nil {
		return false
	}
	if windows.LookupPrivilegeValue(nil, utf16Ptr("SeTcbPrivilege"), &tcb) != nil {
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
	path := filepath.Join(exeDir(), "install-privileges.ps1")
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
	info.lpVerb = utf16Ptr("runas")
	info.lpFile = utf16Ptr("pwsh.exe")
	info.lpParameters = utf16Ptr(`-NoLogo -NoProfile -ExecutionPolicy Bypass -File "` + script + `"`)
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

// makeShortcut writes the shortcut on the desktop that opens this window.
func makeShortcut() error {
	desktop, err := desktopDir()
	if err != nil {
		return err
	}
	path := filepath.Join(desktop, "LocalRDP 控制台.lnk")
	body := `Set s = CreateObject("WScript.Shell")` + "\r\n" +
		`Set l = s.CreateShortcut("` + path + `")` + "\r\n" +
		`l.TargetPath = "` + exePath() + `"` + "\r\n" +
		`l.Arguments = "-gui"` + "\r\n" +
		`l.WorkingDirectory = "` + exeDir() + `"` + "\r\n" +
		`l.IconLocation = "` + exePath() + `,0"` + "\r\n" +
		`l.Save` + "\r\n"
	_ = os.MkdirAll(store.Dir(), 0o755)
	tmp := filepath.Join(store.Dir(), "shortcut.vbs")
	if err := os.WriteFile(tmp, []byte(body), 0o644); err != nil {
		return err
	}
	cmd := exec.Command("wscript.exe", "//nologo", tmp)
	hideWindow(cmd)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// desktopDir is the desktop of this account.
func desktopDir() (string, error) {
	key, err := registry.OpenKey(registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Explorer\User Shell Folders`, registry.QUERY_VALUE)
	if err == nil {
		defer key.Close()
		if value, _, err := key.GetStringValue("Desktop"); err == nil {
			return os.ExpandEnv(value), nil
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Desktop"), nil
}

// copyText puts s on the clipboard.
func copyText(s string) error {
	cmd := exec.Command("clip.exe")
	hideWindow(cmd)
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

// stderrf writes a line to the standard error.
func stderrf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

// itoa is a number as text, or nothing for zero, so the field starts
// empty and the session's own default shows.
func itoa(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
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
