// single.go is the lock the window takes, so a second start reaches the
// window of the first one instead of opening a second one, and the way to
// find that window.
package win32

import (
	"syscall"
	"unsafe"
)

// errorAlreadyExists is what CreateMutex answers when another program of
// this session holds the name.
const errorAlreadyExists = 183

// GuiClassName is the window class of the mygo window.
const GuiClassName = "MyGoWindow"

// singleLock keeps the mutex alive for as long as the program runs: a
// mutex no handle refers to is gone.
var singleLock uintptr

// ClaimSingleInstance takes the named lock of the window and reports
// whether this program is the first to take it. A second program of the
// same session gets false and should bring the first one up instead. When
// the mutex cannot be created at all the program goes on: one window too
// many is better than none.
func ClaimSingleInstance(name string) bool {
	if singleLock != 0 {
		return true
	}
	p, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return true
	}
	h, _, e := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(p)))
	if h == 0 {
		return true
	}
	if errno, ok := e.(syscall.Errno); ok && errno == errorAlreadyExists {
		procCloseHandle.Call(h)
		return false
	}
	singleLock = h
	return true
}

// ActivateWindow shows and focuses the first window of a class and title.
// The window of the app is named by both, because a second program of the
// same class may belong to something else.
func ActivateWindow(class, title string) bool {
	cls, err := syscall.UTF16PtrFromString(class)
	if err != nil {
		return false
	}
	cap, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return false
	}
	hwnd, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(cap)))
	if hwnd == 0 {
		return false
	}
	procShowWindow.Call(hwnd, swRestore)
	procSetForegroundWindow.Call(hwnd)
	// Windows often refuses SetForegroundWindow to a process that is not in
	// front already; SwitchToThisWindow still brings the window up.
	procSwitchToThisWindow.Call(hwnd, 1)
	return true
}
