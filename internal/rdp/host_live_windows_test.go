//go:build windows

package rdp

// 真的开窗口的测试：起一个会话进程，量它的矩形。默认跳过，设 LRDP_LIVE=1 才跑：
//
//	$env:LRDP_LIVE = '1'; go test ./internal/rdp -run Live -v
//
// LRDP_LIVE_EXE 可以指到现成的 build\windows-amd64\localrdp.exe，省掉每次编译。

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lxn/win"

	"localrdp/internal/win32"
)

var (
	liveBuildOnce sync.Once
	liveExePath   string
	liveBuildErr  error
	dpiAwareOnce  sync.Once
)

// liveExe 返回被测的 exe：这些测试驱动的是真程序，所以二进制得由当前代码编出来。
func liveExe(t *testing.T) string {
	t.Helper()
	if os.Getenv("LRDP_LIVE") == "" || testing.Short() {
		t.Skip("set LRDP_LIVE=1 to run the tests that open real session windows")
	}
	liveBuildOnce.Do(func() {
		if p := os.Getenv("LRDP_LIVE_EXE"); p != "" {
			liveExePath = p
			return
		}
		// 固定的临时路径：反复跑只覆盖同一个文件，不在临时目录里积垃圾。
		out := filepath.Join(os.TempDir(), "localrdp-live-test.exe")
		cmd := exec.Command("go", "build", "-o", out, "../..")
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			liveBuildErr = err
			return
		}
		liveExePath = out
	})
	if liveBuildErr != nil {
		t.Fatalf("building the exe under test: %v", liveBuildErr)
	}
	return liveExePath
}

// sessionWindow 是会话窗口，按我们自己的窗口类找：一个登录会话里最多一个。
func sessionWindow() win.HWND {
	return win.FindWindow(win32.UTF16Ptr(className), nil)
}

// dpiAwareTestProcess 把测试进程设成 dpi 感知：不设的话 Windows 交给它的是
// 除以 dpi 的坐标，1536 像素的窗口会报成 1024，断言就成了白过。
//
// 必须在测试进程碰过 user32 之前调：awareness 是进程级的，一旦系统已经按某个值的
// 赢了口，再改就是无声失败（SetProcessDpiAwarenessContext 返回 false），所以这里
// 由 TestMain 在最前面调一次，startSession 里的调用只是兑地方。
func dpiAwareTestProcess() {
	dpiAwareOnce.Do(func() {
		procDpiAware := user32.NewProc("SetProcessDpiAwarenessContext")
		// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 是 -4，其余几条也一样。
		if ok, _, err := procDpiAware.Call(uintptr(^uintptr(3))); ok == 0 {
			fmt.Printf("warning: SetProcessDpiAwarenessContext failed (%v); window coordinates will be virtualized\n", err)
		}
	})
}

// TestMain 先把进程的 dpi 感知设好，再跑其余测试。
func TestMain(m *testing.M) {
	dpiAwareTestProcess()
	os.Exit(m.Run())
}

// stopSessions 让 `localrdp stop` 注销子会话并等窗口消失，下一次 FindWindow 才不会抓到
// 上一个。注销本身可能要几十秒（会话在半连接状态下被杀时实测 30 秒），所以给它一个上限。
func stopSessions(t *testing.T, exe string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	if err := exec.CommandContext(ctx, exe, "stop").Run(); err != nil && ctx.Err() == nil {
		t.Logf("stop: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for sessionWindow() != 0 && time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)
	}
}

// startSession 起一个 desktop 会话并等它的窗口出现，返回窗口句柄和收尾函数。
func startSession(t *testing.T, exe string, args ...string) (win.HWND, func()) {
	t.Helper()
	dpiAwareTestProcess()
	if sessionWindow() != 0 {
		t.Skip("a session window is already open; close it before running the live tests")
	}
	// 上一轮的子会话注销得比窗口慢，先把它等干净，否则这次启动会直接撞上
	// "A child session already exists" 并在重试里耗掉半分钟。
	stopSessions(t, exe)
	cmd := exec.Command(exe, append([]string{"desktop"}, args...)...)
	if err := cmd.Start(); err != nil {
		t.Fatalf("starting the session: %v", err)
	}
	cleanup := func() {
		// 先让它自己收：注销先关窗口、进程随之退出；杀掉是最后的兼预。
		// 反过来（先杀再 stop）会让注销卡在半个连接状态的会话上，实测白白等了 30 秒。
		stopSessions(t, exe)
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		if hwnd := sessionWindow(); hwnd != 0 {
			// 窗口一出现还是创建时的尺寸，等连接把大小定下来再量。
			time.Sleep(time.Second)
			return hwnd, cleanup
		}
		if time.Now().After(deadline) {
			cleanup()
			t.Fatalf("no session window after 30s: desktop %v", args)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// 屏幕放不下要的尺寸时，窗口要整块落在工作区里：顶栏是唯一的拖动条，
// 它跑到屏幕外就再也拖不回来了。
func TestLiveOversizedSessionStaysOnScreen(t *testing.T) {
	exe := liveExe(t)
	wa := primaryWorkArea()
	for _, mode := range []string{"fixed", "fit", "scale"} {
		t.Run(mode, func(t *testing.T) {
			hwnd, done := startSession(t, exe, "--size", mode, "--width", "2560", "--height", "1440", "--timeout", "120")
			defer done()
			var w, c win.RECT
			if !win.GetWindowRect(hwnd, &w) || !win.GetClientRect(hwnd, &c) {
				t.Fatal("cannot measure the session window")
			}
			if w.Left < wa.Left || w.Top < wa.Top || w.Right > wa.Right || w.Bottom > wa.Bottom {
				t.Fatalf("window (%d,%d)-(%d,%d) leaves the work area (%d,%d)-(%d,%d)",
					w.Left, w.Top, w.Right, w.Bottom, wa.Left, wa.Top, wa.Right, wa.Bottom)
			}
			if want := (wa.Right - wa.Left) &^ 1; c.Right != want {
				t.Fatalf("client width = %d, want %d (the work area, even)", c.Right, want)
			}
			if want := wa.Bottom - wa.Top; c.Bottom != want {
				t.Fatalf("client height = %d, want %d (the work area)", c.Bottom, want)
			}
		})
	}
}

// 放得下的尺寸要原样用上，别被截。
func TestLiveFittingSessionIsUsedAsIs(t *testing.T) {
	exe := liveExe(t)
	for _, mode := range []string{"fixed", "fit", "scale"} {
		t.Run(mode, func(t *testing.T) {
			// dpi 得在开会话之前量：子会话一接管控制台，本会话的桌面就不在任何显示器
			// 上，这时候 GetDpiForWindow 回落成 96，而后台进程开窗时量到的是 144。
			dpi := startDPI(nil)
			hwnd, done := startSession(t, exe, "--size", mode, "--width", "1200", "--height", "800", "--timeout", "120")
			defer done()
			var c win.RECT
			if !win.GetClientRect(hwnd, &c) {
				t.Fatal("cannot measure the session window")
			}
			wantW := int32(dpiPx(1200, dpi))
			wantB := int32(dpiPx(800, dpi)) + newBarMetrics(dpi).h
			if c.Right != wantW || c.Bottom != wantB {
				t.Fatalf("client = %dx%d, want %dx%d (800 plus the bar, at dpi %d)", c.Right, c.Bottom, wantW, wantB, dpi)
			}
		})
	}
}
