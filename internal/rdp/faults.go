package rdp

import "localrdp/internal/fault"

// faultForDisconnect says which code a disconnect carries, and whether it is
// a failure at all: the ends that are not — the window was closed, the session
// was logged off — get no code, so a normal end stays quiet.
func faultForDisconnect(reason, extended int32, everConnected bool) (fault.Code, string, bool) {
	detail := sprintf("0x%X", uint32(reason))
	if extended == 5 {
		return fault.TakenOver, detail, true
	}
	switch uint32(reason) {
	case 0x1: // 本地断开：窗口关了
		return "", "", false
	case 0x2: // 子会话里注销
		return "", "", false
	case 0x3: // 系统结束了会话
		if everConnected {
			return "", "", false
		}
		return fault.Disconnected, detail, true
	case 0x108:
		return fault.ConnectTimeout, detail, true
	case 0x807:
		return fault.LogonRejected, detail, true
	case 0xD07: // 3335：本机传输在握手阶段被中断
		return fault.TLSHandshake, detail, true
	}
	return fault.Disconnected, detail, true
}

// fail records the failure the window ends with and says it once: on the
// stream the launcher reads, and in the log. The first one wins — what comes
// after is a consequence of it.
func (s *hostState) fail(c fault.Code, detail string) {
	if s.code != "" {
		return
	}
	s.code, s.detail = c, detail
	s.failed = true
	line := fault.Line(c, detail)
	printErr(line)
	logError(line)
}

// warn says what went wrong without turning the session into a failure.
func (s *hostState) warn(c fault.Code, detail string) {
	line := fault.Line(c, detail)
	printErr(line)
	logWarn(line)
}
