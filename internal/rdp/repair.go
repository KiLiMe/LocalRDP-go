package rdp

import (
	"syscall"
	"time"
	"unsafe"

	"localrdp/internal/win32"
)

// repair.go is `localrdp repair`: restart the local RDP transport, which is
// the one thing that clears a half-open handshake for good (E202). It is the
// only place the program touches a service, it asks for the approval it needs,
// and it does nothing else — no firewall, no listener, no credentials.

var (
	procControlService        = advapi32.NewProc("ControlService")
	procStartServiceW         = advapi32.NewProc("StartServiceW")
	procEnumDependentServices = advapi32.NewProc("EnumDependentServicesW")
)

const (
	// termService is the service the child session's transport lives in.
	termService = "TermService"

	scManagerAllAccess = 0x000F003F

	svcQueryStatus    = 0x0004
	svcStart          = 0x0010
	svcStop           = 0x0020
	svcEnumDependents = 0x0008

	svcControlStop = 0x00000001

	svcStateStopped = 0x0001
	svcStateRunning = 0x0004
	svcAllStates    = 0x0003
)

var (
	// kServiceWait bounds one stop or start: seconds, not minutes.
	kServiceWait = 25 * time.Second
	// kTransportWait is how long the transport gets to come back once the
	// service says it is running.
	kTransportWait = 15 * time.Second
	kServicePoll   = 300 * time.Millisecond
)

// cmdRepair restarts the transport, asking for the approval it needs first.
func cmdRepair() int {
	if !isElevated() {
		printOut("Restarting " + termService + " needs a one-time administrator approval.")
		return relaunchElevated("repair")
	}
	return repairTransport()
}

// repairTransport stops the service and starts it again, then waits for the
// child-session transport to answer.
func repairTransport() int {
	printOut("Restarting " + termService + " (the local RDP transport)…")
	logInfo("repair：重启 " + termService)
	scm, _, err := procOpenSCManagerW.Call(0, 0, scManagerAllAccess)
	if scm == 0 {
		var errno uint32
		if e, ok := err.(syscall.Errno); ok {
			errno = uint32(e)
		}
		printErr("OpenSCManager failed: " + formatWin32(errno))
		return int(ExitError)
	}
	defer procCloseServiceHandle.Call(scm)
	// 依赖得先停，否则 TermService 拒绝停止；起来之后 Windows 由触发启动
	// 再把它们带起来，这里不去抢。
	for _, name := range serviceDependents(scm, termService) {
		if !stopService(scm, name) {
			printErr("Could not stop " + name + ".")
			return int(ExitError)
		}
	}
	if !stopService(scm, termService) {
		return int(ExitError)
	}
	if !startService(scm, termService) {
		return int(ExitError)
	}
	// 服务说自己在跑不等于传输建好了：等管道真能建出来。
	deadline := time.Now().Add(kTransportWait)
	for {
		if status, pipe := probeTransport(); status == 0 {
			msg := termService + " is running and the transport is ready"
			if pipe != "" {
				msg += " (" + pipe + ")"
			}
			printOut(msg + ".")
			logInfo("repair：传输已就绪")
			return int(ExitOk)
		}
		if !time.Now().Before(deadline) {
			break
		}
		time.Sleep(kServicePoll)
	}
	status, _ := probeTransport()
	printErr("Transport is still not ready after the restart: " + describeTransport(status))
	return int(ExitError)
}

// openService opens one service with the rights asked for; 0 when it cannot.
func openService(scm uintptr, name string, access uint32) uintptr {
	h, _, _ := procOpenServiceW.Call(scm, uintptr(unsafe.Pointer(win32.UTF16Ptr(name))), uintptr(access))
	return h
}

// serviceState is the service's current state, 0 when it cannot be read.
func serviceState(scm uintptr, name string) uint32 {
	h := openService(scm, name, svcQueryStatus)
	if h == 0 {
		return 0
	}
	defer procCloseServiceHandle.Call(h)
	var ssp serviceStatusProcess
	var needed uint32
	r, _, _ := procQueryServiceStatusEx.Call(h, scStatusProcessInfo,
		uintptr(unsafe.Pointer(&ssp)), unsafe.Sizeof(ssp), uintptr(unsafe.Pointer(&needed)))
	if r == 0 {
		return 0
	}
	return ssp.CurrentState
}

// stopService stops one service and waits until it says it stopped.
func stopService(scm uintptr, name string) bool {
	switch serviceState(scm, name) {
	case 0:
		printErr("Cannot read the state of " + name + ".")
		return false
	case svcStateStopped:
		return true
	}
	h := openService(scm, name, svcStop|svcQueryStatus)
	if h == 0 {
		printErr("OpenService(" + name + ") failed.")
		return false
	}
	defer procCloseServiceHandle.Call(h)
	printOut("Stopping " + name + "…")
	var ss serviceStatus
	if r, _, _ := procControlService.Call(h, svcControlStop, uintptr(unsafe.Pointer(&ss))); r == 0 {
		// 已经停了也算成功：状态是准的，返回码不是。
		if serviceState(scm, name) == svcStateStopped {
			return true
		}
		printErr("ControlService(" + name + ") failed.")
		return false
	}
	if !waitServiceState(scm, name, svcStateStopped) {
		printErr(name + " did not stop.")
		return false
	}
	return true
}

// startService starts one service and waits until it says it runs.
func startService(scm uintptr, name string) bool {
	if serviceState(scm, name) == svcStateRunning {
		return true
	}
	h := openService(scm, name, svcStart|svcQueryStatus)
	if h == 0 {
		printErr("OpenService(" + name + ") failed.")
		return false
	}
	defer procCloseServiceHandle.Call(h)
	printOut("Starting " + name + "…")
	// 返回码只说明这次调用，不说明服务状态（已经在跑也算失败），状态说话。
	procStartServiceW.Call(h, 0, 0)
	if !waitServiceState(scm, name, svcStateRunning) {
		printErr(name + " did not start.")
		return false
	}
	return true
}

// waitServiceState polls one service until it is in the state asked for.
func waitServiceState(scm uintptr, name string, want uint32) bool {
	deadline := time.Now().Add(kServiceWait)
	for {
		if serviceState(scm, name) == want {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		time.Sleep(kServicePoll)
	}
}

// enumServiceStatus is ENUM_SERVICE_STATUSW: a dependent service and its state.
// SERVICE_STATUS is spelled out rather than taken from serviceStatusProcess,
// which is longer and would misplace every entry after the first.
type enumServiceStatus struct {
	ServiceName  *uint16
	DisplayName  *uint16
	StatusBuffer serviceStatus
}

// serviceDependents lists the services that have to stop before this one may.
func serviceDependents(scm uintptr, name string) []string {
	h := openService(scm, name, svcEnumDependents|svcQueryStatus)
	if h == 0 {
		return nil
	}
	defer procCloseServiceHandle.Call(h)
	var needed, count uint32
	procEnumDependentServices.Call(h, svcAllStates, 0, 0,
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if needed == 0 || count == 0 {
		return nil
	}
	buf := make([]byte, needed)
	r, _, _ := procEnumDependentServices.Call(h, svcAllStates,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(needed),
		uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&count)))
	if r == 0 || count == 0 {
		return nil
	}
	items := unsafe.Slice((*enumServiceStatus)(unsafe.Pointer(&buf[0])), int(count))
	out := make([]string, 0, len(items))
	for _, it := range items {
		// 名字必须在缓冲区丢掉之前拷出来。
		if it.ServiceName != nil {
			out = append(out, syscall.UTF16ToString(unsafe.Slice(it.ServiceName, 128)))
		}
	}
	return out
}
