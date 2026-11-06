package rdp

import (
	"localrdp/internal/win32"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	gdi32                    = syscall.NewLazyDLL("gdi32.dll")
	procGetModuleFileNameW   = kernel32.NewProc("GetModuleFileNameW")
	procGetCurrentProcessId  = kernel32.NewProc("GetCurrentProcessId")
	procProcessIdToSessionId = kernel32.NewProc("ProcessIdToSessionId")
	procGetEnvironmentVarW   = kernel32.NewProc("GetEnvironmentVariableW")
	procGetTempPathW         = kernel32.NewProc("GetTempPathW")
	procCreateDirectoryW     = kernel32.NewProc("CreateDirectoryW")
	procOpenProcessToken     = advapi32.NewProc("OpenProcessToken")
	procGetTokenInformation  = advapi32.NewProc("GetTokenInformation")
	procLookupPrivilegeValue = advapi32.NewProc("LookupPrivilegeValueW")
	procAdjustTokenPrivs     = advapi32.NewProc("AdjustTokenPrivileges")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
	procGetDC                = user32.NewProc("GetDC")
	procReleaseDC            = user32.NewProc("ReleaseDC")
	procGetDeviceCaps        = gdi32.NewProc("GetDeviceCaps")
	procShellExecuteExW      = shell32.NewProc("ShellExecuteExW")
	procGetConsoleWindow     = kernel32.NewProc("GetConsoleWindow")
	procFreeConsole          = kernel32.NewProc("FreeConsole")
	procSetConsoleCtrlHandle = kernel32.NewProc("SetConsoleCtrlHandler")
)

type tokenElevation struct {
	TokenIsElevated uint32
}

type luid struct {
	LowPart  uint32
	HighPart int32
}

type luidAndAttributes struct {
	Luid       luid
	Attributes uint32
}

type tokenPrivileges struct {
	PrivilegeCount uint32
	Privileges     [1]luidAndAttributes
}

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

func currentProcessID() uint32 {
	r, _, _ := procGetCurrentProcessId.Call()
	return uint32(r)
}

func currentSessionID() uint32 {
	var id uint32
	r, _, _ := procProcessIdToSessionId.Call(uintptr(currentProcessID()), uintptr(unsafe.Pointer(&id)))
	if r == 0 {
		return kNoSession
	}
	return id
}

func processSessionID(pid uint32) (uint32, bool) {
	var id uint32
	r, _, _ := procProcessIdToSessionId.Call(uintptr(pid), uintptr(unsafe.Pointer(&id)))
	return id, r != 0
}

func isElevated() bool {
	var tok syscall.Token
	r, _, _ := procOpenProcessToken.Call(^uintptr(0) /*GetCurrentProcess*/, 0x0008, /*TOKEN_QUERY*/
		uintptr(unsafe.Pointer(&tok)))
	if r == 0 {
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(tok))
	var elev tokenElevation
	var n uint32
	r, _, _ = procGetTokenInformation.Call(uintptr(tok), 20, /*TokenElevation*/
		uintptr(unsafe.Pointer(&elev)), unsafe.Sizeof(elev), uintptr(unsafe.Pointer(&n)))
	return r != 0 && elev.TokenIsElevated != 0
}

func enablePrivilege(name string) bool {
	var tok syscall.Handle
	if r, _, _ := procOpenProcessToken.Call(^uintptr(0), 0x0020|0x0008, // ADJUST_PRIVILEGES|QUERY
		uintptr(unsafe.Pointer(&tok))); r == 0 {
		return false
	}
	defer syscall.CloseHandle(tok)
	var l luid
	if r, _, _ := procLookupPrivilegeValue.Call(0, uintptr(unsafe.Pointer(win32.UTF16Ptr(name))),
		uintptr(unsafe.Pointer(&l))); r == 0 {
		return false
	}
	tp := tokenPrivileges{PrivilegeCount: 1}
	tp.Privileges[0].Luid = l
	tp.Privileges[0].Attributes = 0x00000002 // SE_PRIVILEGE_ENABLED
	r, _, err := procAdjustTokenPrivs.Call(uintptr(tok), 0, uintptr(unsafe.Pointer(&tp)),
		unsafe.Sizeof(tp), 0, 0)
	if r == 0 {
		return false
	}
	if errno, ok := err.(syscall.Errno); ok && errno != 0 {
		return false
	}
	return true
}

// commandLineTail returns the whole command line of this process (used to re-launch elevated).
func commandLineTail() string {
	p, _, _ := procGetCommandLine.Call()
	full := u2s(ptrAt[uint16](p))
	if full == "" {
		return ""
	}
	rest := full
	if strings.HasPrefix(rest, "\"") {
		if i := strings.IndexByte(rest[1:], '"'); i >= 0 {
			rest = rest[i+2:]
		}
	} else if i := strings.IndexAny(rest, " \t"); i >= 0 {
		rest = rest[i:]
	} else {
		rest = ""
	}
	return strings.TrimLeft(rest, " \t")
}

func relaunchElevated(args string) int {
	sei := shellExecuteInfo{}
	sei.cbSize = uint32(unsafe.Sizeof(sei))
	sei.fMask = 0x00000040 // SEE_MASK_NOCLOSEPROCESS
	sei.lpVerb = win32.UTF16Ptr("runas")
	sei.lpFile = win32.UTF16Ptr(win32.ExePath())
	sei.lpParameters = win32.UTF16Ptr(args)
	sei.nShow = 1
	if r, _, err := procShellExecuteExW.Call(uintptr(unsafe.Pointer(&sei))); r == 0 {
		errno := uint32(err.(syscall.Errno))
		if errno == 1223 { // ERROR_CANCELLED
			return int(ExitCancelled)
		}
		printErr("Failed to elevate: " + formatWin32(errno))
		return int(ExitError)
	}
	code := uint32(1)
	if sei.hProcess != 0 {
		syscall.WaitForSingleObject(syscall.Handle(sei.hProcess), 0xFFFFFFFF)
		syscall.GetExitCodeProcess(syscall.Handle(sei.hProcess), &code)
		syscall.CloseHandle(syscall.Handle(sei.hProcess))
	}
	return int(code)
}

func getEnvVar(name string) string {
	n, _, _ := procGetEnvironmentVarW.Call(uintptr(unsafe.Pointer(win32.UTF16Ptr(name))), 0, 0)
	if n == 0 {
		return ""
	}
	buf := make([]uint16, n)
	n, _, _ = procGetEnvironmentVarW.Call(uintptr(unsafe.Pointer(win32.UTF16Ptr(name))),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	if n == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf[:n])
}

func tempDir() string {
	buf := make([]uint16, syscall.MAX_PATH)
	n, _, _ := procGetTempPathW.Call(uintptr(len(buf)), uintptr(unsafe.Pointer(&buf[0])))
	if n == 0 || int(n) > len(buf) {
		return "."
	}
	return strings.TrimRight(syscall.UTF16ToString(buf[:n]), "\\/")
}

func localAppData() string {
	if v := getEnvVar("LOCALAPPDATA"); v != "" {
		return v
	}
	return tempDir()
}

func ensureDir(path string) bool {
	if path == "" {
		return false
	}
	if st, err := os.Stat(path); err == nil && st.IsDir() {
		return true
	}
	if parent := filepath.Dir(path); parent != path && parent != "." {
		if !ensureDir(parent) {
			return false
		}
	}
	r, _, err := procCreateDirectoryW.Call(uintptr(unsafe.Pointer(win32.UTF16Ptr(path))), 0)
	if r != 0 {
		return true
	}
	if errno, ok := err.(syscall.Errno); ok && errno == 183 { // ERROR_ALREADY_EXISTS
		return true
	}
	return false
}

func writeUTF8File(path, body string) bool {
	return os.WriteFile(path, []byte(body), 0o644) == nil
}

func readUTF8File(path string) (string, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	s := string(b)
	s = strings.TrimPrefix(s, "\ufeff")
	return s, true
}

func quoteArg(arg string) string {
	need := arg == ""
	for _, c := range arg {
		if c == ' ' || c == '\t' || c == '"' || c == '\n' {
			need = true
			break
		}
	}
	if !need {
		return arg
	}
	var b strings.Builder
	b.WriteByte('"')
	slashes := 0
	for _, c := range arg {
		switch c {
		case '\\':
			slashes++
			continue
		case '"':
			b.WriteString(strings.Repeat("\\", slashes*2+1))
			b.WriteByte('"')
			slashes = 0
			continue
		}
		if slashes > 0 {
			b.WriteString(strings.Repeat("\\", slashes))
			slashes = 0
		}
		b.WriteRune(c)
	}
	if slashes > 0 {
		b.WriteString(strings.Repeat("\\", slashes*2))
	}
	b.WriteByte('"')
	return b.String()
}

func buildCommandLine(args []string) string {
	parts := make([]string, len(args))
	for i, a := range args {
		parts[i] = quoteArg(a)
	}
	return strings.Join(parts, " ")
}

type rect struct{ Left, Top, Right, Bottom int32 }

func primaryWorkArea() rect {
	r := rect{0, 0, 1280, 720}
	procSystemParametersInfo.Call(0x0030 /*SPI_GETWORKAREA*/, 0, uintptr(unsafe.Pointer(&r)), 0)
	return r
}

// windowDPI mirrors WindowDpi(): GetDpiForWindow when available, else GetDeviceCaps.
func windowDPI(hwnd uintptr) uint32 {
	if p := user32.NewProc("GetDpiForWindow"); p.Find() == nil && hwnd != 0 {
		if dpi, _, _ := p.Call(hwnd); dpi != 0 {
			return uint32(dpi)
		}
	}
	dc, _, _ := procGetDC.Call(hwnd)
	if dc == 0 {
		return 96
	}
	dpi, _, _ := procGetDeviceCaps.Call(dc, 88 /*LOGPIXELSX*/)
	procReleaseDC.Call(hwnd, dc)
	if dpi == 0 {
		return 96
	}
	return uint32(dpi)
}
