package rdp

import (
	"github.com/lxn/win"
)

// host_recover.go is what the session window does about a connect that failed
// before it was up. E202 (0xD07) is the local transport cutting the TLS
// handshake: tssecsrv keeps a half-open handshake behind, usually from a
// connection that did not end cleanly, and the next connect is the one that
// pays for it. It clears on its own after a while, so the first failure is not
// a verdict — the window waits and connects again. E202 never happens once a
// session is up, so nothing here touches a working connection.
//
// See docs/故障代码.md §E202.

const (
	// tlsRetryMax is how many times a rejected handshake is retried before
	// the window reports E202 and closes.
	tlsRetryMax = 2
	// tlsRetryStep is the wait before the first retry; a further retry waits
	// one step longer, so a busy machine gets it over with.
	tlsRetryStep = 2000
	// hangUpWait is how long a disconnect gets to reach the other side before
	// the process is gone.
	hangUpWait = 300
)

// retryTLSHandshake sets up another connect after a rejected handshake and
// says whether it did: false means this was the last attempt, the session had
// been up, the window is closing, or the failure was something else.
func (s *hostState) retryTLSHandshake() bool {
	if s.everConnected || s.closing || s.host == nil || s.tlsRetries >= tlsRetryMax {
		return false
	}
	s.tlsRetries++
	wait := tlsRetryStep * s.tlsRetries
	logf("E202：%d 毫秒后自动重试 %d/%d", wait, s.tlsRetries, tlsRetryMax)
	printOutf("TLS handshake interrupted; connecting again in %d s (%d/%d).",
		wait/1000, s.tlsRetries, tlsRetryMax)
	// retrying keeps the OnDisconnected that ends this attempt from closing
	// the window, and kTimerReconnect connects again with the same options.
	s.retrying = true
	win.SetTimer(s.hwnd, kTimerReconnect, uint32(wait), 0)
	return true
}

// hangUpCleanly disconnects a connection that is still up when the window is
// otherwise done: a connection the client walks away from is what leaves the
// transport half-open for the next one to trip over.
func (s *hostState) hangUpCleanly() {
	if s.host == nil || s.host.disp == nil || !s.everConnected || s.discDone {
		return
	}
	s.discDone = true
	logf("退出前补一次 Disconnect()")
	// 已经断干净的连接再 Disconnect 会抛 DISP_E_EXCEPTION（0x80020009），
	// 那是“早就断了”的意思，不是故障。
	if hr := (disp{s.host.disp}).call("Disconnect"); hrFailed(hr) && uint32(hr) != 0x80020009 {
		logf("Disconnect() hr=%s", formatHr(uint32(hr)))
	}
	sleepMS(hangUpWait)
}

// forceCleanup ends what the window still holds when its cleanup itself hangs:
// the child session, so the next connect does not find one left in the
// disconnected state. It only uses APIs that are safe off the UI thread.
func (s *hostState) forceCleanup(why string) {
	// 登记表里也得把自己抹掉：os.Exit 不会跑 defer，留一行就有个查不到的鬼。
	s.unregisterSelf()
	if s.opt.KeepSess {
		return
	}
	sid := getChildSessionID()
	if sid == kNoSession {
		return
	}
	if wtLogoffSession(sid, false) {
		logf("%s：注销子会话 %d", why, sid)
	}
}
