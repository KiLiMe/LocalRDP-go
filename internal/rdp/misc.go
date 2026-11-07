package rdp

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

var (
	procEnumWindows             = user32.NewProc("EnumWindows")
	procGetWindowTextW          = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW    = user32.NewProc("GetWindowTextLengthW")
	procMonitorFromPoint        = user32.NewProc("MonitorFromPoint")
	procRemoveClipboardListener = user32.NewProc("RemoveClipboardFormatListener")
	procCreateProcessAsUserW    = advapi32.NewProc("CreateProcessAsUserW")
	userenv                     = syscall.NewLazyDLL("userenv.dll")
	dwmapi                      = syscall.NewLazyDLL("dwmapi.dll")
)

func sprintf(format string, a ...any) string { return fmt.Sprintf(format, a...) }

func sleepMS(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

func freeConsole() { procFreeConsole.Call() }

// currentUserSID returns the raw SID bytes of this process token, or nil.
func currentUserSID() []byte {
	var tok syscall.Handle
	if r, _, _ := procOpenProcessToken.Call(^uintptr(0), 0x0008, uintptr(unsafe.Pointer(&tok))); r == 0 {
		return nil
	}
	defer syscall.CloseHandle(tok)
	var need uint32
	procGetTokenInformation.Call(uintptr(tok), 1 /*TokenUser*/, 0, 0, uintptr(unsafe.Pointer(&need)))
	if need == 0 {
		return nil
	}
	buf := make([]byte, need)
	if r, _, _ := procGetTokenInformation.Call(uintptr(tok), 1,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(need), uintptr(unsafe.Pointer(&need))); r == 0 {
		return nil
	}
	// TOKEN_USER.Sid is the first field (a pointer).
	sid := *(*uintptr)(unsafe.Pointer(&buf[0]))
	if sid == 0 {
		return nil
	}
	// Copy enough bytes for any SID (max 68 bytes).
	return ptrAt[[68]byte](sid)[:subAuthorityEnd(sid):subAuthorityEnd(sid)]
}

// subAuthorityEnd computes the actual length of a SID in bytes.
func subAuthorityEnd(sid uintptr) int {
	b := ptrAt[[68]byte](sid)
	n := int(b[1])
	return 8 + n*4
}

func removeClipboardFormatListener(hwnd uintptr) {
	procRemoveClipboardListener.Call(hwnd)
}

func enumWindows(cb uintptr, lparam uintptr) {
	procEnumWindows.Call(cb, lparam)
}

// exe 里的图标资源：go-winres 生成的 rsrc_windows_amd64.syso，图标组号是 1。
// 没有它，任务栏缩略图、Alt+Tab 只能画一个通用的空壳图标。
const (
	appIconID = 1
	iconSmall = 0
	iconBig   = 1
)

// loadAppIcon 按需要的边长从 exe 的图标资源里取一张，取不到返回 0（窗口照旧没有图标）。
func loadAppIcon(inst win.HINSTANCE, size int32) win.HICON {
	if size <= 0 {
		size = 32
	}
	return win.HICON(win.LoadImage(inst, win.MAKEINTRESOURCE(appIconID), win.IMAGE_ICON, size, size, win.LR_DEFAULTCOLOR))
}

func windowText(hwnd uintptr) string {
	n, _, _ := procGetWindowTextLengthW.Call(hwnd)
	if int32(n) <= 0 {
		return ""
	}
	buf := make([]uint16, int(n)+1)
	procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf)
}
