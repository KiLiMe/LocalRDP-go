package rdp

import (
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
	"localrdp/internal/win32"
)

// dropFiles 是 DROPFILES：DWORD pFiles; POINT pt; BOOL fNC; BOOL fWide。
type dropFiles struct {
	PFiles uint32
	Pt     win.POINT
	FNC    int32
	FWide  int32
}

const (
	clipRoleHost  = 1
	clipRoleChild = 2

	clipPollTimer   = 1
	clipParentTimer = 2
	clipWMClose     = win.WM_CLOSE
)

type clipState struct {
	hwnd    win.HWND
	role    int
	parent  uint32
	inPath  string
	outPath string

	seq         uint64
	lastWritten uint64
	lastApplied uint64
	ignoreOnce  bool
	lastPayload string
}

var (
	gClip       *clipState
	clipStarted bool
)

func clipPaths(role int) (outPath, inPath string) {
	dir := localAppData() + "\\LocalRDP"
	ensureDir(dir)
	base := dir + "\\"
	if role == clipRoleHost {
		return base + "clip-to-child.txt", base + "clip-to-parent.txt"
	}
	return base + "clip-to-parent.txt", base + "clip-to-child.txt"
}

func (c *clipState) publish() {
	if c.ignoreOnce {
		c.ignoreOnce = false
		return
	}
	if !win.IsClipboardFormatAvailable(uint32(win.CF_HDROP)) {
		return
	}
	paths := readClipboardFiles()
	if len(paths) == 0 {
		return
	}
	payload := strings.Join(paths, "\n")
	if payload == c.lastPayload {
		return
	}
	c.seq++
	body := "SEQ=" + strconv.FormatUint(c.seq, 10) + "\r\n" + strings.Join(paths, "\r\n") + "\r\n"
	if !writeUTF8File(c.outPath, body) {
		logWarn("写剪贴板报文失败: " + c.outPath)
		return
	}
	c.lastWritten = c.seq
	c.lastPayload = payload
	logf("clipboard -> %s (%d 个文件)", c.outPath, len(paths))
}

func readClipboardFiles() []string {
	if !win.OpenClipboard(0) {
		return nil
	}
	defer win.CloseClipboard()
	h := win.GetClipboardData(uint32(win.CF_HDROP))
	if h == 0 {
		return nil
	}
	n := win.DragQueryFile(win.HDROP(h), 0xFFFFFFFF, nil, 0)
	var out []string
	for i := uint(0); i < n; i++ {
		need := win.DragQueryFile(win.HDROP(h), i, nil, 0)
		buf := make([]uint16, need+1)
		win.DragQueryFile(win.HDROP(h), i, &buf[0], uint(len(buf)))
		out = append(out, syscall.UTF16ToString(buf))
	}
	return out
}

// setClipboardFiles 写入 CF_HDROP（DROPFILES 头 + 双 NUL 结尾的宽字符串列表）。
func setClipboardFiles(paths []string) bool {
	var payload []uint16
	for _, p := range paths {
		payload = append(payload, syscall.StringToUTF16(p)...)
	}
	payload = append(payload, 0)
	total := int(unsafe.Sizeof(dropFiles{})) + len(payload)*2
	h := win.GlobalAlloc(win.GMEM_MOVEABLE|win.GMEM_ZEROINIT, uintptr(total))
	if h == 0 {
		return false
	}
	p := win.GlobalLock(h)
	if p == nil {
		win.GlobalFree(h)
		return false
	}
	df := (*dropFiles)(p)
	df.PFiles = uint32(unsafe.Sizeof(dropFiles{}))
	df.FWide = 1
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(uintptr(p)+uintptr(df.PFiles))), len(payload))
	copy(dst, payload)
	win.GlobalUnlock(h)

	if !win.OpenClipboard(0) {
		win.GlobalFree(h)
		return false
	}
	defer win.CloseClipboard()
	win.EmptyClipboard()
	if win.SetClipboardData(uint32(win.CF_HDROP), win.HANDLE(h)) == 0 {
		win.GlobalFree(h)
		return false
	}
	return true
}

func (c *clipState) applyIncoming() {
	text, ok := readUTF8File(c.inPath)
	if !ok || text == "" {
		return
	}
	lines := strings.Split(strings.ReplaceAll(text, "\r", ""), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "SEQ=") {
		return
	}
	seq, err := strconv.ParseUint(strings.TrimSpace(lines[0][4:]), 10, 64)
	if err != nil || seq == 0 || seq == c.lastApplied || seq == c.lastWritten {
		return
	}
	var paths []string
	for _, l := range lines[1:] {
		if s := strings.TrimSpace(l); s != "" {
			paths = append(paths, s)
		}
	}
	if len(paths) == 0 {
		return
	}
	if !setClipboardFiles(paths) {
		logWarn("设置剪贴板失败")
		return
	}
	c.lastApplied = seq
	c.ignoreOnce = true
	logf("clipboard <- %s (%d 个文件)", c.inPath, len(paths))
}

func clipWndProc(hwnd win.HWND, msg uint32, wParam, lParam uintptr) uintptr {
	c := gClip
	if c == nil {
		return win.DefWindowProc(hwnd, msg, wParam, lParam)
	}
	switch msg {
	case win.WM_CLIPBOARDUPDATE:
		c.publish()
		return 0
	case win.WM_TIMER:
		switch wParam {
		case clipPollTimer:
			c.applyIncoming()
		case clipParentTimer:
			if c.parent != 0 && !processAlive(c.parent) {
				logInfo("父进程已退出，剪贴板桥结束")
				win.DestroyWindow(hwnd)
			}
		}
		return 0
	case clipWMClose:
		win.DestroyWindow(hwnd)
		return 0
	case win.WM_DESTROY:
		removeClipboardFormatListener(uintptr(hwnd))
		win.PostQuitMessage(0)
		return 0
	}
	return win.DefWindowProc(hwnd, msg, wParam, lParam)
}

var clipWndProcCb = syscall.NewCallback(clipWndProc)

// clipBridgeMain 在当前线程跑一个仅消息窗口 + 消息循环，直到窗口销毁。
func clipBridgeMain(role int, parent uint32) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	outPath, inPath := clipPaths(role)
	c := &clipState{role: role, parent: parent, outPath: outPath, inPath: inPath}
	gClip = c

	inst := win.GetModuleHandle(nil)
	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		LpfnWndProc:   clipWndProcCb,
		HInstance:     inst,
		LpszClassName: win32.UTF16Ptr("LocalRDP.ClipBridge"),
	}
	win.RegisterClassEx(&wc)

	hwnd := win.CreateWindowEx(0, win32.UTF16Ptr("LocalRDP.ClipBridge"), nil, 0, 0, 0, 0, 0,
		win.HWND_MESSAGE, 0, inst, nil)
	if hwnd == 0 {
		logWarn("剪贴板桥创建消息窗失败")
		gClip = nil
		return
	}
	c.hwnd = hwnd
	win.AddClipboardFormatListener(hwnd)
	win.SetTimer(hwnd, clipPollTimer, 400, 0)
	if role == clipRoleChild {
		win.SetTimer(hwnd, clipParentTimer, 1000, 0)
	}
	logf("剪贴板桥已就绪 role=%d out=%s in=%s", role, outPath, inPath)

	var msg win.MSG
	for {
		r := win.GetMessage(&msg, 0, 0, 0)
		if r == 0 || r == -1 {
			break
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	win.KillTimer(hwnd, clipPollTimer)
	win.KillTimer(hwnd, clipParentTimer)
	gClip = nil
}

func startClipHost() {
	if clipStarted {
		return
	}
	clipStarted = true
	go clipBridgeMain(clipRoleHost, 0)
}

func stopClipHost() {
	if gClip != nil && gClip.role == clipRoleHost && gClip.hwnd != 0 {
		win.PostMessage(gClip.hwnd, clipWMClose, 0, 0)
	}
}
