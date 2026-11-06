package rdp

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	"localrdp/internal/win32"
)

// processAlive 判断进程是否仍在运行。
func processAlive(pid uint32) bool {
	if pid == 0 {
		return false
	}
	h, err := syscall.OpenProcess(0x1000 /*PROCESS_QUERY_LIMITED_INFORMATION*/, false, pid)
	if err != nil {
		return false
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == 259 // STILL_ACTIVE
}

// runSessionEntryProcess 在子会话里启动 job 里的命令行并等待它结束。
func runSessionEntryProcess(job *StartOptions) int32 {
	cmdLine := job.Command
	if cmdLine == "" {
		return 0
	}
	var si startupInfo
	si.Cb = uint32(unsafe.Sizeof(si))
	si.Desktop = win32.UTF16Ptr("winsta0\\default")
	var pi processInformation
	var workDir *uint16
	if job.WorkDir != "" {
		workDir = win32.UTF16Ptr(job.WorkDir)
	}
	r, _, err := procCreateProcessW.Call(0,
		uintptr(unsafe.Pointer(win32.UTF16Ptr(cmdLine))), 0, 0, 0,
		uintptr(createNewConsole|createUnicodeEnvironment),
		0, uintptr(unsafe.Pointer(workDir)), uintptr(unsafe.Pointer(&si)), uintptr(unsafe.Pointer(&pi)))
	if r == 0 {
		logError("CreateProcess failed: " + err.Error())
		printErr("CreateProcess failed: " + err.Error())
		return -1
	}
	logf("[session] started pid=%d", pi.ProcessID)
	procWaitForSingleObject.Call(uintptr(pi.Process), 0xFFFFFFFF)
	var code uint32
	syscall.GetExitCodeProcess(syscall.Handle(pi.Process), &code)
	logf("[session] process exited %d", int32(code))
	syscall.CloseHandle(syscall.Handle(pi.Thread))
	syscall.CloseHandle(syscall.Handle(pi.Process))
	return int32(code)
}

var procPostMessage = user32.NewProc("PostMessageW")

// startDialogGuard 对应 src/session_entry.cpp 的 guard 线程：关掉 CrossDeviceResume 之类的系统对话框。
func startDialogGuard(stop chan struct{}) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cb := syscall.NewCallback(func(hwnd uintptr, lparam uintptr) uintptr {
		if !win.IsWindowVisible(win.HWND(hwnd)) {
			return 1
		}
		var buf [256]uint16
		if n, _ := win.GetClassName(win.HWND(hwnd), &buf[0], len(buf)); n <= 0 {
			return 1
		}
		if syscall.UTF16ToString(buf[:]) != "#32770" {
			return 1
		}
		title := windowText(hwnd)
		if !hasSuffixFold(title, "\\CrossDeviceResume.exe") {
			return 1
		}
		logf("[session] dismissed system dialog: %s", title)
		procPostMessage.Call(hwnd, uintptr(win.WM_CLOSE), 0, 0)
		return 1
	})
	for {
		select {
		case <-stop:
			return
		default:
		}
		enumWindows(cb, 0)
		sleepMS(750)
	}
}

func hasSuffixFold(s, suffix string) bool {
	return len(s) >= len(suffix) && equalFoldASCII(s[len(s)-len(suffix):], suffix)
}

func equalFoldASCII(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		ca, cb := a[i], b[i]
		if 'A' <= ca && ca <= 'Z' {
			ca += 32
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// runSessionEntry 对应 src/session_entry.cpp 的 main：在子会话里自举。
func runSessionEntry(expectSession uint32, jobPath string) int {
	freeConsole()
	logf("session-entry started expect=%d cur=%d", expectSession, currentSessionID())
	cur := currentSessionID()
	if cur == 0 || (expectSession != kNoSession && cur != expectSession) {
		logError("session-entry 在错误的会话中运行")
		return int(ExitError)
	}
	var job StartOptions
	if !readJobFile(jobPath, &job) {
		logError("session-entry 读取 job 失败: " + jobPath)
		return int(ExitError)
	}
	deleteJobFile(jobPath)

	stop := make(chan struct{})
	go startDialogGuard(stop)

	code := runSessionEntryProcess(&job)
	close(stop)

	// 干净退出：子会话里跑完程序就注销，除非明确要求保留。
	if !job.Desktop && !job.KeepSess {
		wtLogoffSession(cur, false)
	}
	if code == -1 {
		return int(ExitError)
	}
	return int(ExitOk)
}
