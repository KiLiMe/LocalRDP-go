package rdp

import (
	"fmt"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/internal/win32"
	"localrdp/locales"
)

var (
	procEnumWindows             = user32.NewProc("EnumWindows")
	procGetWindowTextW          = user32.NewProc("GetWindowTextW")
	procGetWindowTextLengthW    = user32.NewProc("GetWindowTextLengthW")
	procSetWindowTextW          = user32.NewProc("SetWindowTextW")
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

// hostIcons 返回会话窗口要挂的两个图标：大图标与小图标。会话版（右下角带点）
// 取到就用它，取不到（PNG 解不开、DIB 建不成）退回 exe 资源里的那张。
//
// 边长按窗口所在屏的 dpi 放大：150% 时任务栏要 24、Alt+Tab 要 48，给 16/32 让系统
// 自己拉大就糊了，而我们手上那源图是 256 的，缩得出来。
func hostIcons(inst win.HINSTANCE, dpi uint32) (big, small win.HICON) {
	bigSize := int32(dpiPx(int(win.GetSystemMetrics(win.SM_CXICON)), dpi))
	smallSize := int32(dpiPx(int(win.GetSystemMetrics(win.SM_CXSMICON)), dpi))
	big = sessionIcon(bigSize)
	small = sessionIcon(smallSize)
	if big == 0 {
		big = loadAppIcon(inst, bigSize)
	}
	if small == 0 {
		small = loadAppIcon(inst, smallSize)
	}
	return big, small
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

// setWindowText 换窗口标题；任务栏标签与 tooltip 跟着它变。
func setWindowText(hwnd uintptr, title string) {
	p, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	procSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(p)))
}

// sessionTitle 是会话窗口的标题：用户给的 title 后面挂上子会话号，任务栏上
// 两个按钮才分得出哪个是主窗口、哪个是会话窗口。
func sessionTitle(base string, sid uint32) string {
	base = strings.TrimSpace(base)
	if base == "" {
		base = "LocalRDP"
	}
	if sid == 0 {
		return base
	}
	return base + " · " + locales.ChildSession() + " " + win32.Itoa(int(sid))
}
