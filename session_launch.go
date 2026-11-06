package main

import (
	"syscall"
	"unsafe"
)

var (
	procCreateProcessW           = kernel32.NewProc("CreateProcessW")
	procOpenProcess              = kernel32.NewProc("OpenProcess")
	procTerminateProcess         = kernel32.NewProc("TerminateProcess")
	procCloseHandle              = kernel32.NewProc("CloseHandle")
	procGetExitCodeProcess       = kernel32.NewProc("GetExitCodeProcess")
	procWaitForSingleObject      = kernel32.NewProc("WaitForSingleObject")
	procCreateEnvironmentBlock   = userenv.NewProc("CreateEnvironmentBlock")
	procDestroyEnvironmentBlock  = userenv.NewProc("DestroyEnvironmentBlock")
	procDuplicateTokenEx         = advapi32.NewProc("DuplicateTokenEx")
	procOpenProcessTokenEx       = advapi32.NewProc("OpenProcessToken")
	procInitProcThreadAttrList   = kernel32.NewProc("InitializeProcThreadAttributeList")
	procUpdateProcThreadAttr     = kernel32.NewProc("UpdateProcThreadAttribute")
	procDeleteProcThreadAttrList = kernel32.NewProc("DeleteProcThreadAttributeList")
)

type startupInfo struct {
	Cb            uint32
	Reserved      *uint16
	Desktop       *uint16
	Title         *uint16
	X, Y          int32
	XSize, YSize  int32
	XCountChars   int32
	YCountChars   int32
	FillAttribute uint32
	Flags         uint32
	ShowWindow    uint16
	Reserved2     uint16
	Reserved3     *byte
	StdInput      syscall.Handle
	StdOutput     syscall.Handle
	StdErr        syscall.Handle
}

type startupInfoEx struct {
	StartupInfo   startupInfo
	AttributeList uintptr
}

type processInformation struct {
	Process   syscall.Handle
	Thread    syscall.Handle
	ProcessID uint32
	ThreadID  uint32
}

const (
	startfUseShowWindow         = 0x00000001
	createUnicodeEnvironment    = 0x00000400
	createNewConsole            = 0x00000010
	createBreakawayFromJob      = 0x01000000
	extendedStartupInfoPresent  = 0x00080000
	procThreadAttributeParent   = 0x00020000
	seDebugName                 = "SeDebugPrivilege"
	seTcbName                   = "SeTcbPrivilege"
	seAssignPrimaryTokenName    = "SeAssignPrimaryTokenPrivilege"
	seIncreaseQuotaName         = "SeIncreaseQuotaPrivilege"
)

func scoreProcess(name string) int {
	switch lower(name) {
	case "explorer.exe":
		return 100
	case "userinit.exe":
		return 80
	case "rdpshell.exe":
		return 70
	case "sihost.exe":
		return 60
	case "runtimebroker.exe":
		return 40
	case "winlogon.exe", "csrss.exe", "smss.exe", "services.exe", "lsass.exe":
		return 0
	}
	return 10
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

func runProcess(cmdLine, workDir string, flags uint32, env uintptr, si *startupInfoEx, token syscall.Token) (uint32, uint32) {
	var pi processInformation
	cmd := utf16Slice(cmdLine)
	var wd *uint16
	if workDir != "" {
		wd = utf16Ptr(workDir)
	}
	var r uintptr
	var err error
	if token != 0 {
		r, _, err = procCreateProcessAsUserW.Call(uintptr(token), 0,
			uintptr(unsafe.Pointer(&cmd[0])), 0, 0, 0, uintptr(flags), env,
			uintptr(unsafe.Pointer(wd)), uintptr(unsafe.Pointer(si)), uintptr(unsafe.Pointer(&pi)))
	} else {
		r, _, err = procCreateProcessW.Call(0, uintptr(unsafe.Pointer(&cmd[0])), 0, 0, 0,
			uintptr(flags), env, uintptr(unsafe.Pointer(wd)),
			uintptr(unsafe.Pointer(si)), uintptr(unsafe.Pointer(&pi)))
	}
	if r == 0 {
		errno := uint32(0)
		if e, ok := err.(syscall.Errno); ok {
			errno = uint32(e)
		}
		return 0, errno
	}
	procCloseHandle.Call(uintptr(pi.Thread))
	procCloseHandle.Call(uintptr(pi.Process))
	return pi.ProcessID, 0
}

var (
	procCreateProcessWithTokenW = advapi32.NewProc("CreateProcessWithTokenW")
)

// runProcessWithToken 走 advapi32!CreateProcessWithTokenW（只需 SeImpersonatePrivilege）。
func runProcessWithToken(token syscall.Token, cmdLine, workDir string, flags uint32, env uintptr, si *startupInfoEx) (uint32, uint32) {
	var pi processInformation
	cmd := utf16Slice(cmdLine)
	var wd *uint16
	if workDir != "" {
		wd = utf16Ptr(workDir)
	}
	r, _, err := procCreateProcessWithTokenW.Call(uintptr(token), 1 /*LOGON_WITH_PROFILE*/, 0,
		uintptr(unsafe.Pointer(&cmd[0])), uintptr(flags), env,
		uintptr(unsafe.Pointer(wd)), uintptr(unsafe.Pointer(&si.StartupInfo)), uintptr(unsafe.Pointer(&pi)))
	if r == 0 {
		errno := uint32(0)
		if e, ok := err.(syscall.Errno); ok {
			errno = uint32(e)
		}
		return 0, errno
	}
	procCloseHandle.Call(uintptr(pi.Thread))
	procCloseHandle.Call(uintptr(pi.Process))
	return pi.ProcessID, 0
}

func launchWithPrimaryToken(token syscall.Token, cmdLine, workDir string, pid *uint32) bool {
	var env uintptr
	procCreateEnvironmentBlock.Call(uintptr(unsafe.Pointer(&env)), uintptr(token), 0)
	si := startupInfoEx{}
	si.StartupInfo.Cb = uint32(unsafe.Sizeof(si))
	si.StartupInfo.Desktop = utf16Ptr("winsta0\\default")
	si.StartupInfo.Flags = startfUseShowWindow
	si.StartupInfo.ShowWindow = 1 // SW_SHOWNORMAL
	id, errno := runProcess(cmdLine, workDir,
		createUnicodeEnvironment|createNewConsole|createBreakawayFromJob, env, &si, token)
	if id == 0 {
		// CreateProcessAsUser 需要 SeAssignPrimaryTokenPrivilege（本机管理员令牌里没有），
		// 退回 CreateProcessWithTokenW —— 它只要 SeImpersonatePrivilege。
		var id2, errno2 uint32
		if env == 0 {
			procCreateEnvironmentBlock.Call(uintptr(unsafe.Pointer(&env)), uintptr(token), 0)
		}
		id2, errno2 = runProcessWithToken(token, cmdLine, workDir,
			createUnicodeEnvironment|createNewConsole|createBreakawayFromJob, env, &si)
		if id2 != 0 {
			id, errno = id2, 0
		} else {
			logWarn("CreateProcessAsUser failed: " + formatWin32(errno) + " / WithToken failed: " + formatWin32(errno2))
		}
	}
	if env != 0 {
		procDestroyEnvironmentBlock.Call(env)
	}
	if id == 0 {
		return false
	}
	if pid != nil {
		*pid = id
	}
	return true
}

func launchWithWtsToken(sessionID uint32, cmdLine, workDir string, pid *uint32) bool {
	enablePrivilege(seTcbName)
	enablePrivilege(seAssignPrimaryTokenName)
	enablePrivilege(seIncreaseQuotaName)
	var token syscall.Token
	r, _, err := procWTSQueryUserToken.Call(uintptr(sessionID), uintptr(unsafe.Pointer(&token)))
	if r == 0 {
		errno := uint32(0)
		if e, ok := err.(syscall.Errno); ok {
			errno = uint32(e)
		}
		logInfo("WTSQueryUserToken unavailable: " + formatWin32(errno))
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(token))
	return launchWithPrimaryToken(token, cmdLine, workDir, pid)
}

func launchWithProcessToken(proc syscall.Handle, cmdLine, workDir string, pid *uint32) bool {
	var ptok syscall.Handle
	if r, _, err := procOpenProcessTokenEx.Call(uintptr(proc), 0x0002|0x0008|0x0001,
		uintptr(unsafe.Pointer(&ptok))); r == 0 {
		var errno uint32
		if e, ok := err.(syscall.Errno); ok {
			errno = uint32(e)
		}
		logWarn("OpenProcessToken failed: " + formatWin32(errno))
		return false
	}
	defer syscall.CloseHandle(ptok)
	var primary syscall.Token
	r, _, err := procDuplicateTokenEx.Call(uintptr(ptok), 0x02000000 /*TOKEN_ALL_ACCESS*/, 0,
		2 /*SecurityImpersonation*/, 1 /*TokenPrimary*/, uintptr(unsafe.Pointer(&primary)))
	if r == 0 {
		var errno uint32
		if e, ok := err.(syscall.Errno); ok {
			errno = uint32(e)
		}
		logWarn("DuplicateTokenEx failed: " + formatWin32(errno))
		return false
	}
	defer syscall.CloseHandle(syscall.Handle(primary))
	return launchWithPrimaryToken(primary, cmdLine, workDir, pid)
}

func launchAsChild(parent syscall.Handle, cmdLine, workDir string, pid *uint32) bool {
	var size uintptr
	procInitProcThreadAttrList.Call(0, 1, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		return false
	}
	buf := make([]byte, size)
	list := uintptr(unsafe.Pointer(&buf[0]))
	if r, _, _ := procInitProcThreadAttrList.Call(list, 1, 0, uintptr(unsafe.Pointer(&size))); r == 0 {
		return false
	}
	defer procDeleteProcThreadAttrList.Call(list)
	parentCopy := parent
	if r, _, err := procUpdateProcThreadAttr.Call(list, 0, procThreadAttributeParent,
		uintptr(unsafe.Pointer(&parentCopy)), unsafe.Sizeof(parentCopy), 0, 0); r == 0 {
		var errno uint32
		if e, ok := err.(syscall.Errno); ok {
			errno = uint32(e)
		}
		logWarn("UpdateProcThreadAttribute failed: " + formatWin32(errno))
		return false
	}
	si := startupInfoEx{}
	si.StartupInfo.Cb = uint32(unsafe.Sizeof(si))
	si.StartupInfo.Flags = startfUseShowWindow
	si.StartupInfo.ShowWindow = 1
	si.AttributeList = list
	id, errno := runProcess(cmdLine, workDir,
		createUnicodeEnvironment|extendedStartupInfoPresent|createNewConsole|createBreakawayFromJob,
		0, &si, 0)
	if id == 0 {
		logWarn("CreateProcess (parent attribute) failed: " + formatWin32(errno))
		return false
	}
	if pid != nil {
		*pid = id
	}
	return true
}

func killPID(pid uint32) {
	h, _, _ := procOpenProcess.Call(0x0001 /*PROCESS_TERMINATE*/, 0, uintptr(pid))
	if h == 0 {
		return
	}
	procTerminateProcess.Call(h, 1)
	procCloseHandle.Call(h)
}

func acceptLaunch(sessionID, pid uint32) bool {
	if pid == 0 {
		return false
	}
	if id, ok := processSessionID(pid); ok && id == sessionID {
		logInfo(sprintf("Launched pid=%d in session %d", pid, sessionID))
		return true
	}
	got, _ := processSessionID(pid)
	logWarn(sprintf("New process pid=%d is in session %d, expected %d; terminating it", pid, got, sessionID))
	killPID(pid)
	return false
}

// processEntry32W 对应 PROCESSENTRY32W。
type processEntry32W struct {
	Size            uint32
	CntUsage        uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	CntThreads      uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

var (
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = kernel32.NewProc("Process32FirstW")
	procProcess32NextW           = kernel32.NewProc("Process32NextW")
)

// enumerateSessionProcesses 用 Toolhelp 快照枚举进程。
// 不用 WTSEnumerateProcessesW：实测本机该 API 返回的 ProcessId 恒为 0/1（C# 侧同样如此）。
func enumerateSessionProcesses(sessionID uint32) []processEntry32W {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(0x00000002 /*TH32CS_SNAPPROCESS*/, 0)
	if snap == 0 || snap == uintptr(0xFFFFFFFFFFFFFFFF) {
		logWarn("CreateToolhelp32Snapshot failed")
		return nil
	}
	defer syscall.CloseHandle(syscall.Handle(snap))
	var e processEntry32W
	e.Size = uint32(unsafe.Sizeof(e))
	var out []processEntry32W
	r, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e)))
	for r != 0 {
		if sid, ok := processSessionID(e.ProcessID); ok && sid == sessionID {
			out = append(out, e)
		}
		e.Size = uint32(unsafe.Sizeof(e))
		r, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e)))
	}
	return out
}

// openSessionProcess picks a foreground-ish process in the child session and opens it.
func openSessionProcess(sessionID uint32) syscall.Handle {
	enablePrivilege(seDebugName)
	procs := enumerateSessionProcesses(sessionID)
	bestPID, bestScore := uint32(0), 0
	for _, p := range procs {
		if p.ProcessID == currentProcessID() {
			continue
		}
		score := scoreProcess(u2s(&p.ExeFile[0]))
		if score > bestScore {
			bestScore, bestPID = score, p.ProcessID
		}
	}
	if bestPID == 0 {
		logWarn(sprintf("No suitable process in child session %d (%d procs)", sessionID, len(procs)))
		return 0
	}
	accesses := []uintptr{0x0400 | 0x0040 | 0x0080, 0x1000 | 0x0040 | 0x0080, 0x1F0FFF}
	for _, acc := range accesses {
		h, _, _ := procOpenProcess.Call(acc, 0, uintptr(bestPID))
		if h != 0 {
			logInfo(sprintf("Using session process pid=%d score=%d", bestPID, bestScore))
			return syscall.Handle(h)
		}
	}
	logWarn(sprintf("OpenProcess pid=%d failed", bestPID))
	return 0
}
// launchInSession mirrors session_launch.cpp: WTS token → process token → parent attribute.
func launchInSession(sessionID uint32, command, workDir string, pidOut *uint32) bool {
	if command == "" || sessionID == kNoSession {
		return false
	}
	for attempt := 0; attempt < 8; attempt++ {
		if attempt > 0 {
			sleepMS(400)
		}
		var pid uint32
		if launchWithWtsToken(sessionID, command, workDir, &pid) && acceptLaunch(sessionID, pid) {
			if pidOut != nil {
				*pidOut = pid
			}
			return true
		}
		proc := openSessionProcess(sessionID)
		if proc == 0 {
			continue
		}
		pid = 0
		if launchWithProcessToken(proc, command, workDir, &pid) && acceptLaunch(sessionID, pid) {
			syscall.CloseHandle(proc)
			if pidOut != nil {
				*pidOut = pid
			}
			return true
		}
		pid = 0
		if launchAsChild(proc, command, workDir, &pid) && acceptLaunch(sessionID, pid) {
			syscall.CloseHandle(proc)
			if pidOut != nil {
				*pidOut = pid
			}
			return true
		}
		syscall.CloseHandle(proc)
	}
	return false
}
