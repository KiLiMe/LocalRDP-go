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
	procRemoveClipboardListener = user32.NewProc("RemoveClipboardFormatListener")
	procCreateProcessAsUserW    = advapi32.NewProc("CreateProcessAsUserW")
	userenv                     = syscall.NewLazyDLL("userenv.dll")
)

func sprintf(format string, a ...any) string { return fmt.Sprintf(format, a...) }

func sleepMS(ms int) { time.Sleep(time.Duration(ms) * time.Millisecond) }

func freeConsole() { procFreeConsole.Call() }

func removeClipboardFormatListener(hwnd uintptr) {
	procRemoveClipboardListener.Call(hwnd)
}

func enumWindows(cb uintptr, lparam uintptr) {
	procEnumWindows.Call(cb, lparam)
}

// exe 里的图标资源：mygo 打包时写进 exe 的图标组，组号是 1（它自己分组的顺序）。
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
