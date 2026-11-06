package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

var (
	procEnumWindows              = user32.NewProc("EnumWindows")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW     = user32.NewProc("GetWindowTextLengthW")
	procAdjustWindowRectEx       = user32.NewProc("AdjustWindowRectEx")
	procMonitorFromPoint         = user32.NewProc("MonitorFromPoint")
	procRemoveClipboardListener  = user32.NewProc("RemoveClipboardFormatListener")
	procIsWindow                 = user32.NewProc("IsWindow")
	procEqualSid                 = advapi32.NewProc("EqualSid")
	procCreateProcessAsUserW     = advapi32.NewProc("CreateProcessAsUserW")
	userenv                      = syscall.NewLazyDLL("userenv.dll")
	dwmapi                       = syscall.NewLazyDLL("dwmapi.dll")
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
	return (*[68]byte)(unsafe.Pointer(sid))[:subAuthorityEnd(sid):subAuthorityEnd(sid)]
}

// subAuthorityEnd computes the actual length of a SID in bytes.
func subAuthorityEnd(sid uintptr) int {
	b := (*[68]byte)(unsafe.Pointer(sid))
	n := int(b[1])
	return 8 + n*4
}

func equalSID(a []byte, b uintptr) bool {
	if len(a) == 0 || b == 0 {
		return false
	}
	r, _, _ := procEqualSid.Call(uintptr(unsafe.Pointer(&a[0])), b)
	return r != 0
}

func adjustWindowRectEx(r *win_Rect, style uint32, menu bool, ex uint32) bool {
	m := 0
	if menu {
		m = 1
	}
	ok, _, _ := procAdjustWindowRectEx.Call(uintptr(unsafe.Pointer(r)), uintptr(style),
		uintptr(m), uintptr(ex))
	return ok != 0
}

func isWindow(h uintptr) bool {
	r, _, _ := procIsWindow.Call(h)
	return r != 0
}

func removeClipboardFormatListener(hwnd uintptr) {
	procRemoveClipboardListener.Call(hwnd)
}

func enumWindows(cb uintptr, lparam uintptr) {
	procEnumWindows.Call(cb, lparam)
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

// win_Rect aliases lxn/win RECT so helpers above stay readable.
type win_Rect = rect
