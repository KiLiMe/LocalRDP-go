package rdp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"localrdp/internal/store"
	"localrdp/internal/win32"
)

// hosts.go is the book the session windows keep about themselves: one small
// file per host process under %LOCALAPPDATA%\LocalRDP\hosts. A host that loses
// its message loop is otherwise invisible — nothing on disk says it exists,
// and the only trace is the file lock on localrdp.exe — so the window writes
// its own record, `localrdp hosts` reads them, and `localrdp stop` closes what
// it finds. The file is not the truth: a record whose process is gone is
// removed on sight, and a live process whose window is gone is a leftover the
// stop path ends.

var (
	procGetExitCodeProcess = kernel32.NewProc("GetExitCodeProcess")
	// kStillActive is what GetExitCodeProcess says about a running process.
	kStillActive = 259
	// kHostCloseWait is how long a host gets to act on WM_CLOSE before it is
	// ended: a window whose loop is gone never acts on it.
	kHostCloseWait = 3 * time.Second
)

// hostRecord is what one session window says about itself on disk.
type hostRecord struct {
	PID       uint32  `json:"pid"`
	HWND      uintptr `json:"hwnd"`
	SessionID uint32  `json:"session_id,omitempty"`
	Mode      string  `json:"mode"`
	Title     string  `json:"title"`
	Started   string  `json:"started"`
}

// windowLive reports whether the process still has its host window. It says
// nothing about the message loop: the leftover hosts keep the process and lose
// the window, which is exactly what this distinguishes.
func (r hostRecord) windowLive() bool {
	return win32.HasHostWindow(int(r.PID))
}

// hostsDir is where the records live.
func hostsDir() string { return filepath.Join(store.Dir(), "hosts") }

// hostRecordPath is the file one process writes about itself.
func hostRecordPath(pid uint32) string {
	return filepath.Join(hostsDir(), win32.Itoa(int(pid))+".json")
}

// registerHost writes this process into the book and returns the path the
// update and unregister calls take ("" when the write failed — a host that
// cannot be listed is a host nothing can stop, so it is only a warning).
func registerHost(rec hostRecord) string {
	if err := os.MkdirAll(hostsDir(), 0o700); err != nil {
		logWarn("宿主登记目录建不出来: " + err.Error())
		return ""
	}
	path := hostRecordPath(rec.PID)
	if err := writeHostRecord(path, rec); err != nil {
		logWarn("宿主登记写入失败: " + err.Error())
		return ""
	}
	return path
}

// writeHostRecord replaces the record in one step, so a reader never sees half.
func writeHostRecord(path string, rec hostRecord) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// updateHostRecord rewrites a record in place (the child session id, once it is
// known). A missing or unwritable record is not worth a log line.
func updateHostRecord(path string, rec hostRecord) {
	if path == "" {
		return
	}
	_ = writeHostRecord(path, rec)
}

// unregisterHost removes this process from the book.
func unregisterHost(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

// listHosts returns the hosts whose process is still there, dropping the
// records of the ones that are not: a crash leaves the file behind.
func listHosts() []hostRecord {
	entries, err := os.ReadDir(hostsDir())
	if err != nil {
		return nil
	}
	out := make([]hostRecord, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(hostsDir(), e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var rec hostRecord
		if err := json.Unmarshal(b, &rec); err != nil || rec.PID == 0 {
			// 坏文件留着只会误导人。
			_ = os.Remove(path)
			continue
		}
		if !pidAlive(rec.PID) {
			_ = os.Remove(path)
			continue
		}
		out = append(out, rec)
	}
	return out
}

// pidAlive reports whether a process id still names a running process.
func pidAlive(pid uint32) bool {
	h, _, _ := procOpenProcess.Call(0x1000 /*PROCESS_QUERY_LIMITED_INFORMATION*/, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer procCloseHandle.Call(h)
	var code uint32
	if r, _, _ := procGetExitCodeProcess.Call(h, uintptr(unsafe.Pointer(&code))); r == 0 {
		return false
	}
	return code == uint32(kStillActive)
}

// terminatePID ends one process, no questions: only leftovers and hosts that
// ignored WM_CLOSE get here.
func terminatePID(pid uint32) bool {
	h, _, _ := procOpenProcess.Call(0x0001 /*PROCESS_TERMINATE*/, 0, uintptr(pid))
	if h == 0 {
		return false
	}
	defer procCloseHandle.Call(h)
	r, _, _ := procTerminateProcess.Call(h, 1)
	return r != 0
}

// focusHost brings a host's window up and to the front.
func focusHost(rec hostRecord) bool {
	return win32.BringToFront(int(rec.PID))
}

// closeHost asks the window to close, and ends the process when it has not gone
// after the wait — the leftover state that started this whole file.
func closeHost(rec hostRecord, wait time.Duration) bool {
	win32.CloseWindow(int(rec.PID))
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if !pidAlive(rec.PID) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !pidAlive(rec.PID) {
		return true
	}
	logWarn("宿主 pid=" + win32.Itoa(int(rec.PID)) + " 不响应关闭，结束进程")
	return terminatePID(rec.PID)
}

// stopAllHosts closes every host but this process, and returns how many went
// away and how many refused to.
func stopAllHosts() (int, int) {
	me := uint32(os.Getpid())
	closed, failed := 0, 0
	for _, rec := range listHosts() {
		if rec.PID == me {
			continue
		}
		if closeHost(rec, kHostCloseWait) {
			closed++
		} else {
			failed++
		}
		unregisterHost(hostRecordPath(rec.PID))
	}
	return closed, failed
}

// reusableHost is the window a new `desktop`/`start` hands over to instead of
// opening a second one: same mode, still has a window. Nothing is created and
// nothing is logged off — the window that is already there is the session.
func reusableHost(mode string) (hostRecord, bool) {
	me := uint32(os.Getpid())
	for _, rec := range listHosts() {
		if rec.PID == me || rec.Mode != mode {
			continue
		}
		if rec.windowLive() {
			return rec, true
		}
	}
	return hostRecord{}, false
}

// reapWindowlessHosts ends the hosts that have no window left: they cannot be
// brought to the front, cannot act on WM_CLOSE, and hold localrdp.exe open.
func reapWindowlessHosts() int {
	me := uint32(os.Getpid())
	n := 0
	for _, rec := range listHosts() {
		if rec.PID == me || rec.windowLive() {
			continue
		}
		if terminatePID(rec.PID) {
			n++
			logWarn("收掉没有窗口的宿主 pid=" + win32.Itoa(int(rec.PID)))
		}
		unregisterHost(hostRecordPath(rec.PID))
	}
	return n
}

// describeHosts is the one status line: how many hosts, how many without a
// window (those are the leftovers a stop has to clear).
func describeHosts() string {
	return describeHostsOf(listHosts(), uint32(os.Getpid()), hostRecord.windowLive)
}

// describeHostsOf is describeHosts with its inputs handed in, so the counting
// can be tested without three processes on the machine.
func describeHostsOf(list []hostRecord, me uint32, windowLive func(hostRecord) bool) string {
	if len(list) == 0 {
		return "0"
	}
	windowless := 0
	for _, rec := range list {
		if rec.PID != me && !windowLive(rec) {
			windowless++
		}
	}
	s := win32.Itoa(len(list)) + " registered"
	if windowless > 0 {
		s += ", " + win32.Itoa(windowless) + " without a window"
	}
	return s
}

// registerSelf writes this window's record, once the window exists. From this
// point on the window can be found even if its own loop stops answering.
func (s *hostState) registerSelf() {
	mode := "desktop"
	if !s.opt.Desktop {
		mode = "start"
	}
	s.registryRec = hostRecord{
		PID:     uint32(os.Getpid()),
		HWND:    uintptr(s.hwnd),
		Mode:    mode,
		Title:   s.opt.Title,
		Started: time.Now().Format(time.RFC3339),
	}
	s.registryPath = registerHost(s.registryRec)
}

// refreshSelf keeps the record true when the child session is known: the id is
// what tells a live session from a leftover.
func (s *hostState) refreshSelf() {
	if s.registryPath == "" {
		return
	}
	if sid := getChildSessionID(); sid != kNoSession {
		s.registryRec.SessionID = sid
	}
	s.registryRec.HWND = uintptr(s.hwnd)
	updateHostRecord(s.registryPath, s.registryRec)
}

// forgetSessionInBook drops the child session id the record carries: after a
// disconnect that id no longer belongs to this window.
func (s *hostState) forgetSessionInBook() {
	if s.registryPath == "" || s.registryRec.SessionID == 0 {
		return
	}
	s.registryRec.SessionID = 0
	updateHostRecord(s.registryPath, s.registryRec)
}

// unregisterSelf removes this window from the book on the way out.
func (s *hostState) unregisterSelf() {
	unregisterHost(s.registryPath)
	s.registryPath = ""
}

// cmdHosts is `localrdp hosts`: every session window that says it is alive,
// with the two things that say whether it can still be used (window, session).
func cmdHosts() int {
	list := listHosts()
	if len(list) == 0 {
		printOut("No host processes are registered.")
		return int(ExitOk)
	}
	for _, rec := range list {
		session := "(none)"
		if rec.SessionID != kNoSession && rec.SessionID != 0 {
			session = win32.Itoa(int(rec.SessionID))
		}
		state := "no-window"
		if rec.windowLive() {
			state = "ok"
		}
		printOutf("%-6s %-8s %-9s session=%-7s %-6s since %s", win32.Itoa(int(rec.PID)),
			"hwnd="+win32.Itoa(int(rec.HWND)), state, session, rec.Mode, shortClock(rec.Started))
	}
	return int(ExitOk)
}

// shortClock turns a record's RFC3339 stamp into hh:mm:ss for the listing; a
// stamp that cannot be parsed is shown as it is.
func shortClock(stamp string) string {
	t, err := time.Parse(time.RFC3339, stamp)
	if err != nil {
		return stamp
	}
	return t.Format("15:04:05")
}
