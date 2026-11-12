package rdp

import (
	"syscall"
	"unsafe"

	"localrdp/internal/fault"
	"localrdp/internal/win32"
)

var (
	procRegGetValueW           = advapi32.NewProc("RegGetValueW")
	procOpenSCManagerW         = advapi32.NewProc("OpenSCManagerW")
	procOpenServiceW           = advapi32.NewProc("OpenServiceW")
	procQueryServiceStatusEx   = advapi32.NewProc("QueryServiceStatusEx")
	procCloseServiceHandle     = advapi32.NewProc("CloseServiceHandle")
	procRegOpenKeyExW          = advapi32.NewProc("RegOpenKeyExW")
	procRegCloseKey            = advapi32.NewProc("RegCloseKey")
	procWTSIsChildSessions     = wtsapi32.NewProc("WTSIsChildSessionsEnabled")
	procWTSGetChildSessionId   = wtsapi32.NewProc("WTSGetChildSessionId")
	procWTSEnableChildSessions = wtsapi32.NewProc("WTSEnableChildSessions")
	procWTSLogoffSession       = wtsapi32.NewProc("WTSLogoffSession")
	procWTSQueryUserToken      = wtsapi32.NewProc("WTSQueryUserToken")
	procWTSFreeMemory          = wtsapi32.NewProc("WTSFreeMemory")

	procWTSEnumerateSessionsW = wtsapi32.NewProc("WTSEnumerateSessionsW")
)

// wtsStateDown is WTS_CONNECTSTATE_CLASS's value for a session that is down.
const wtsStateDown = 8

// wtsSessionInfoRaw is the WTS_SESSION_INFOW layout, used inside wtsSessions
// only: its WinStationName points into the buffer WTSFreeMemory hands back.
type wtsSessionInfoRaw struct {
	SessionID      uint32
	WinStationName *uint16
	State          int32
}

// wtsSessionInfo is a copy of one session, with the name already a Go string
// so it outlives the buffer it came from.
type wtsSessionInfo struct {
	SessionID      uint32
	WinStationName string
	State          int32
}

// wtsSessions 列出本机的 WTS 会话。
func wtsSessions() []wtsSessionInfo {
	var p *wtsSessionInfoRaw
	var count uint32
	r, _, _ := procWTSEnumerateSessionsW.Call(0, 0, 1,
		uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&count)))
	if r == 0 || p == nil {
		return nil
	}
	defer procWTSFreeMemory.Call(uintptr(unsafe.Pointer(p)))
	raw := unsafe.Slice(p, int(count))
	out := make([]wtsSessionInfo, len(raw))
	for i, it := range raw {
		// 名字必须在缓冲区释放之前拷出来。
		name := ""
		if it.WinStationName != nil {
			name = syscall.UTF16ToString(unsafe.Slice(it.WinStationName, 64))
		}
		out[i] = wtsSessionInfo{SessionID: it.SessionID, WinStationName: name, State: it.State}
	}
	return out
}

// sessionAlive 判断会话是否仍然真实存在。子会话注销之后
// WTSGetChildSessionId 还会返回旧 id，只有枚举结果算数。
func sessionAlive(id uint32) bool {
	for _, s := range wtsSessions() {
		if s.SessionID == id && s.State != wtsStateDown {
			return true
		}
	}
	return false
}

const (
	hkeyLocalMachine    = 0x80000002
	hkeyClassesRoot     = 0x80000000
	serviceQueryStatus  = 0x0004
	scManagerConnect    = 0x0001
	scStatusProcessInfo = 0
)

func readRegSz(root uintptr, sub, name string) string {
	buf := make([]uint16, 256)
	cb := uint32(len(buf) * 2)
	var typ uint32
	r, _, _ := procRegGetValueW.Call(root, uintptr(unsafe.Pointer(win32.UTF16Ptr(sub))),
		uintptr(unsafe.Pointer(win32.UTF16Ptr(name))), 0x00000002|0x00000004, /*SZ|EXPAND_SZ*/
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&cb)))
	if r != 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func readRegDword(root uintptr, sub, name string, def uint32) uint32 {
	v := def
	cb := uint32(4)
	var typ uint32
	r, _, _ := procRegGetValueW.Call(root, uintptr(unsafe.Pointer(win32.UTF16Ptr(sub))),
		uintptr(unsafe.Pointer(win32.UTF16Ptr(name))), 0x00000010, /*DWORD*/
		uintptr(unsafe.Pointer(&typ)), uintptr(unsafe.Pointer(&v)), uintptr(unsafe.Pointer(&cb)))
	if r != 0 {
		return def
	}
	return v
}

type serviceStatus struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
}

// serviceStatusProcess is SERVICE_STATUS_PROCESS: SERVICE_STATUS plus the two
// fields QueryServiceStatusEx adds.
type serviceStatusProcess struct {
	serviceStatus
	ProcessID    uint32
	ServiceFlags uint32
}

func serviceRunning(name string) bool {
	scm, _, _ := procOpenSCManagerW.Call(0, 0, scManagerConnect)
	if scm == 0 {
		return false
	}
	defer procCloseServiceHandle.Call(scm)
	svc, _, _ := procOpenServiceW.Call(scm, uintptr(unsafe.Pointer(win32.UTF16Ptr(name))),
		serviceQueryStatus)
	if svc == 0 {
		return false
	}
	defer procCloseServiceHandle.Call(svc)
	var ssp serviceStatusProcess
	var needed uint32
	r, _, _ := procQueryServiceStatusEx.Call(svc, scStatusProcessInfo,
		uintptr(unsafe.Pointer(&ssp)), unsafe.Sizeof(ssp), uintptr(unsafe.Pointer(&needed)))
	return r != 0 && ssp.CurrentState == 4 // SERVICE_RUNNING
}

func clsidRegistered(clsid string) bool {
	var h uintptr
	r, _, _ := procRegOpenKeyExW.Call(hkeyClassesRoot,
		uintptr(unsafe.Pointer(win32.UTF16Ptr("CLSID\\"+clsid))), 0, 0x00020019, /*KEY_READ*/
		uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return false
	}
	procRegCloseKey.Call(h)
	return true
}

func childSessionsEnabled() bool {
	var enabled int32
	r, _, _ := procWTSIsChildSessions.Call(uintptr(unsafe.Pointer(&enabled)))
	return r != 0 && enabled != 0
}

func getChildSessionID() uint32 {
	var id uint32 = kNoSession
	r, _, _ := procWTSGetChildSessionId.Call(uintptr(unsafe.Pointer(&id)))
	if r == 0 {
		return kNoSession
	}
	return id
}

func insideChildSession() bool {
	mine := currentSessionID()
	child := getChildSessionID()
	return child != kNoSession && child != 0 && child == mine
}

// probeTransport calls windbg's WinStationCreateChildSessionTransport when present.
func probeTransport() (uint32, string) {
	m, err := syscall.LoadLibrary("winsta.dll")
	if err != nil {
		return 0x80070000 | uint32(err.(syscall.Errno)), ""
	}
	defer syscall.FreeLibrary(m)
	addr, err := syscall.GetProcAddress(m, "WinStationCreateChildSessionTransport")
	if err != nil {
		return 0x80070000 | 127 /*ERROR_PROC_NOT_FOUND*/, ""
	}
	buf := make([]uint16, 0x80)
	status, _, _ := syscall.SyscallN(addr, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	pipe := ""
	if int32(status) == 0 {
		pipe = syscall.UTF16ToString(buf)
	}
	return uint32(status), pipe
}

func describeTransport(status uint32) string {
	switch status {
	case 0:
		return "ready"
	case 0x800706BA:
		return "RPC server unavailable — child-session transport is not up"
	case 0x80070005:
		return "access denied"
	}
	return "error " + formatHr(status)
}

func queryEnvironment() EnvReport {
	r := EnvReport{}
	r.EditionID = readRegSz(hkeyLocalMachine, "SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion", "EditionID")
	r.BuildNumber = readRegSz(hkeyLocalMachine, "SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion", "CurrentBuild")
	r.HomeEdition = len(r.EditionID) >= 4 && equalFoldPrefix(r.EditionID, "Core")
	r.ChildSessionsOn = childSessionsEnabled()
	r.TermServiceRunning = serviceRunning("TermService")
	r.RdpListenerEnabled = readRegDword(hkeyLocalMachine,
		"SYSTEM\\CurrentControlSet\\Control\\Terminal Server", "fDenyTSConnections", 1) == 0
	r.RdpControlOK = clsidRegistered("{A0C63C30-F08D-4AB4-907C-34905D770C7D}") ||
		clsidRegistered("{8B918B82-7985-4C24-89DF-C33AD2BBFBCD}")
	r.InsideChildSession = insideChildSession()
	r.Elevated = isElevated()
	r.ChildSessionID = getChildSessionID()
	r.CurrentSessionID = currentSessionID()
	r.TransportStatus, r.TransportPipe = probeTransport()

	switch {
	case r.HomeEdition:
		r.Fault = fault.Edition
		r.NextAction = "This Windows edition has no Remote Desktop server component. Use Pro, Enterprise, Education, or Server."
	case r.InsideChildSession:
		r.Fault = fault.InsideChild
		r.NextAction = "Run this tool from the parent session, not from inside the child session."
	case !r.RdpControlOK:
		r.Fault = fault.RdpControl
		r.NextAction = "The Remote Desktop ActiveX control is not registered (mstscax.dll)."
	case !r.ChildSessionsOn:
		r.Fault = fault.ChildSessionsOff
		r.NextAction = "Child sessions are off. `localrdp start` will enable them (one administrator prompt if needed)."
	case !r.TermServiceRunning:
		r.Fault = fault.TermService
		r.NextAction = "TermService is not running: start it (one administrator prompt), then open the session again."
	case hrFailed(uintptr(r.TransportStatus)):
		r.Fault = fault.Transport
		r.NextAction = "Child-session transport is not available. Try `localrdp start` anyway."
	}
	return r
}

func equalFoldPrefix(s, prefix string) bool {
	if len(s) < len(prefix) {
		return false
	}
	for i := 0; i < len(prefix); i++ {
		a, b := s[i], prefix[i]
		if a >= 'A' && a <= 'Z' {
			a += 32
		}
		if b >= 'A' && b <= 'Z' {
			b += 32
		}
		if a != b {
			return false
		}
	}
	return true
}

func yn(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func printStatus(r EnvReport) {
	line := func(k, v string) { printOutf("%-21s= %s", k, v) }
	line("Version", "LocalRDP "+versionString)
	line("EditionID", r.EditionID+" (build "+r.BuildNumber+")")
	line("HomeEdition", yn(r.HomeEdition))
	line("Elevated", yn(r.Elevated))
	line("ChildSessionsEnabled", yn(r.ChildSessionsOn))
	line("TermServiceRunning", yn(r.TermServiceRunning))
	line("RdpListenerEnabled", yn(r.RdpListenerEnabled))
	line("RdpControlRegistered", yn(r.RdpControlOK))
	line("InsideChildSession", yn(r.InsideChildSession))
	line("CurrentSessionId", win32.Itoa(int(r.CurrentSessionID)))
	if r.ChildSessionID == kNoSession {
		line("ChildSessionId", "(none)")
	} else {
		line("ChildSessionId", win32.Itoa(int(r.ChildSessionID)))
	}
	line("Transport", sprintf("0x%08X", r.TransportStatus)+" ("+describeTransport(r.TransportStatus)+")")
	if r.TransportPipe != "" {
		line("TransportPipe", r.TransportPipe)
	}
	line("Hosts", describeHosts())
	if r.Fault != "" {
		line("Fault", string(r.Fault)+" "+r.Fault.Short())
	}
	line("Ready", yn(r.ready()))
	if r.NextAction != "" {
		line("NextAction", r.NextAction)
	}
}

// ensureChildSessionsForStart mirrors host_prep.cpp.
func ensureChildSessionsForStart() int {
	if childSessionsEnabled() {
		return 0
	}
	r, _, err := procWTSEnableChildSessions.Call(1)
	if r == 0 {
		if !isElevated() {
			printOut("Enabling Windows child sessions needs a one-time administrator approval.")
			return relaunchElevated(commandLineTail())
		}
		var errno uint32
		if e, ok := err.(syscall.Errno); ok {
			errno = uint32(e)
		}
		printErr("WTSEnableChildSessions failed: " + formatWin32(errno))
		return int(ExitError)
	}
	if !childSessionsEnabled() {
		printErr("Child sessions are still disabled after WTSEnableChildSessions.")
		return int(ExitError)
	}
	logInfo("Child sessions enabled")
	printOut("Windows child sessions enabled.")
	return 0
}

func wtLogoffSession(id uint32, wait bool) bool {
	var w uintptr
	if wait {
		w = 1
	}
	r, _, _ := procWTSLogoffSession.Call(0 /*WTS_CURRENT_SERVER_HANDLE*/, uintptr(id), w)
	return r != 0
}
