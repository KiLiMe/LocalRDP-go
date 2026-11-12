package rdp

import (
	"os"
	"testing"
)

// TestHostBookKeepsOnlyLiveProcesses pins the two things the book must get
// right: a record of a live process is read back, and a record whose process
// is gone is dropped instead of being reported as a host.
func TestHostBookKeepsOnlyLiveProcesses(t *testing.T) {
	t.Setenv("LOCALAPPDATA", t.TempDir())

	path := registerHost(hostRecord{
		PID:     uint32(os.Getpid()),
		HWND:    0,
		Mode:    "desktop",
		Title:   "LocalRDP",
		Started: "2026-11-12T22:26:55+08:00",
	})
	if path == "" {
		t.Fatal("registerHost wrote nothing")
	}
	list := listHosts()
	if len(list) != 1 || list[0].PID != uint32(os.Getpid()) {
		t.Fatalf("listHosts = %+v, want this process", list)
	}

	// 进程没了，记录也得没：否则 hosts/stop 会一直数幽灵。
	deadPath := hostRecordPath(0x7FFFFFF0)
	if err := writeHostRecord(deadPath, hostRecord{PID: 0x7FFFFFF0, Mode: "desktop"}); err != nil {
		t.Fatalf("writeHostRecord: %v", err)
	}
	if got := listHosts(); len(got) != 1 {
		t.Fatalf("listHosts after a dead record = %+v, want only the live one", got)
	}
	if _, err := os.Stat(deadPath); !os.IsNotExist(err) {
		t.Fatalf("dead record %s was not removed (%v)", deadPath, err)
	}

	unregisterHost(path)
	if got := listHosts(); len(got) != 0 {
		t.Fatalf("listHosts after unregister = %+v, want none", got)
	}
	if got := describeHosts(); got != "0" {
		t.Fatalf("describeHosts = %q, want %q", got, "0")
	}
}

// TestDescribeHostsCountsWindowless makes sure the status line tells a live
// window from a leftover: only the second one is worth a stop.
func TestDescribeHostsCountsWindowless(t *testing.T) {
	me := uint32(1000)
	list := []hostRecord{
		{PID: me, Mode: "desktop"},   // 调用者自己，不算残留
		{PID: 1001, Mode: "desktop"}, // 有窗口
		{PID: 1002, Mode: "start"},   // 没有窗口：残留
	}
	windowLive := func(rec hostRecord) bool { return rec.PID == 1001 }
	if got, want := describeHostsOf(list, me, windowLive), "3 registered, 1 without a window"; got != want {
		t.Fatalf("describeHostsOf = %q, want %q", got, want)
	}
	if got, want := describeHostsOf(nil, me, windowLive), "0"; got != want {
		t.Fatalf("describeHostsOf(nil) = %q, want %q", got, want)
	}
}
