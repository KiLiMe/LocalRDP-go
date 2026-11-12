package rdp

import (
	"os"
	"runtime"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/internal/fault"
	"localrdp/internal/i18n"
	"localrdp/internal/store"
	"localrdp/internal/win32"
)

// pickLanguage speaks the language the window is set to: the host runs as a
// separate process, so it cannot inherit it.
func pickLanguage() {
	lang := store.LoadSettings().Lang
	if lang == "" {
		lang = i18n.Detect()
	}
	i18n.Set(lang)
}

// sessionAppID is the taskbar group of the child desktop window. A process
// that names no group is grouped by its exe path, and the form is that same
// exe — so without an ID of its own the child desktop would land on the form's
// button, with the form's icon and preview. Its own group gives it its own
// button, so the two windows can be found, switched to and closed apart.
const sessionAppID = "com.localrdp.desktop.session"

// RunRdpSession 对应 src/rdp_host.cpp 的 RunRdpSession()。
func RunRdpSession(opt StartOptions, jobPath string) int {
	if err := win32.SetAppUserModelID(sessionAppID); err != nil {
		// 没有自己的任务栏分组也能跑，只是窗口会并到主窗口那个按钮上。
		logWarn("设置任务栏分组失败: " + err.Error())
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	pickLanguage()

	icc := win.INITCOMMONCONTROLSEX{DwSize: uint32(unsafe.Sizeof(win.INITCOMMONCONTROLSEX{})), DwICC: win.ICC_WIN95_CLASSES}
	win.InitCommonControlsEx(&icc)

	hr := win.OleInitialize()
	if hrFailed(uintptr(uint32(hr))) && hr != 1 && uint32(hr) != 0x80010106 {
		printErr("OleInitialize failed: " + formatHr(uint32(hr)))
		return int(ExitError)
	}
	defer win.OleUninitialize()

	s := &hostState{opt: opt, jobPath: jobPath, host: newAxHost()}
	s.hostDPI = startDPI(&opt)
	s.m = newBarMetrics(s.hostDPI)
	s.sizeMode = store.NormalizeSizeMode(opt.SizeMode)
	if opt.Fullscreen {
		// 全屏就是让画面填满屏幕，和尺寸档冲突：全屏优先。
		// 只停用全屏期间的模式，退出全屏后照旧生效。
		s.suspendSizeModeForFullscreen()
	}
	logf("窗口尺寸模式 %s", s.sizeMode)
	gHost = s

	x, y, cw, ch, clamped := defaultWindowRect(&opt, s.m.dpi)
	if clamped {
		// 尺寸是用户要的，但屏幕放不下：说一声，免得以为是自己填错了。
		printOutf("The window does not fit the screen; using %dx%d.", cw, ch)
		logf("屏幕放不下，窗口改成 %dx%d", cw, ch)
	}
	style := uint32(win.WS_OVERLAPPEDWINDOW | win.WS_CLIPCHILDREN | win.WS_CLIPSIBLINGS)
	exStyle := uint32(win.WS_EX_APPWINDOW)
	show := int32(win.SW_SHOW)
	if opt.Hidden {
		style = win.WS_POPUP
		exStyle = win.WS_EX_TOOLWINDOW | win.WS_EX_NOACTIVATE
		show = int32(win.SW_HIDE)
		x, y = 0, 0
	} else if opt.Minimized {
		show = int32(win.SW_SHOWMINNOACTIVE)
	}
	if opt.Fullscreen {
		style = win.WS_POPUP | win.WS_VISIBLE
		show = int32(win.SW_SHOW)
	}
	if s.sizeMode == store.SizeFixed {
		// 固定大小：没有可拖的边框，也没有最大化按钮。
		style &^= win.WS_THICKFRAME | win.WS_MAXIMIZEBOX
	}

	// 无边框：窗口矩形就是客户区，不再往外加一圈看不见的边框。
	rc := win.RECT{Left: x, Top: y, Right: x + cw, Bottom: y + ch}
	s.fixedW, s.fixedH = cw, ch
	s.resizable = style&win.WS_THICKFRAME != 0

	inst := win.GetModuleHandle(nil)
	appIcon, appIconSm := hostIcons(inst, s.m.dpi)
	wc := win.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(win.WNDCLASSEX{})),
		Style:         win.CS_HREDRAW | win.CS_VREDRAW,
		LpfnWndProc:   wndProcCb,
		HInstance:     inst,
		HCursor:       win.LoadCursor(0, win.MAKEINTRESOURCE(uintptr(win.IDC_ARROW))),
		HbrBackground: win.HBRUSH(win.GetStockObject(win.BLACK_BRUSH)),
		HIcon:         appIcon,
		HIconSm:       appIconSm,
		LpszClassName: win32.UTF16Ptr(className),
	}
	if win.RegisterClassEx(&wc) == 0 {
		printErr("RegisterClassEx failed: " + formatWin32(uint32(win.GetLastError())))
		return int(ExitError)
	}
	s.hwnd = win.CreateWindowEx(exStyle, win32.UTF16Ptr(className), win32.UTF16Ptr(opt.Title), style,
		rc.Left, rc.Top, rc.Right-rc.Left, rc.Bottom-rc.Top, 0, 0, inst, nil)
	if s.hwnd == 0 {
		printErr("CreateWindowEx failed: " + formatWin32(uint32(win.GetLastError())))
		return int(ExitError)
	}
	// 窗口自己再挂一次图标：只写在窗口类里时，任务栏缩略图和 Alt+Tab 有时会画成空壳。
	win.SendMessage(s.hwnd, win.WM_SETICON, iconBig, uintptr(appIcon))
	win.SendMessage(s.hwnd, win.WM_SETICON, iconSmall, uintptr(appIconSm))
	if opt.Fullscreen {
		// 走同一条路，才能记住原样式/位置，Ctrl+Alt+F 才退得回来。
		s.enterFullscreen()
	} else {
		// 建窗口时系统只问了不带 wParam 的那一次，客户区还是按有标题栏算的；
		// 这一句让它按新的算法重算一次，标题栏才真的消失。
		win.SetWindowPos(s.hwnd, 0, 0, 0, 0, 0,
			win.SWP_FRAMECHANGED|win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOZORDER|win.SWP_NOACTIVATE)
	}
	win.ShowWindow(s.hwnd, show)
	if show != int32(win.SW_HIDE) && !win.IsWindowVisible(s.hwnd) {
		// A process started with STARTF_USESHOWWINDOW/SW_HIDE (see
		// win32.HideWindow) has its very first ShowWindow ignored, which left
		// the window built but never seen. A second call does show it.
		win.ShowWindow(s.hwnd, int32(win.SW_SHOW))
		win.SetForegroundWindow(s.hwnd)
	}
	win.UpdateWindow(s.hwnd)
	if !win32.RegisterHotKey(uintptr(s.hwnd), hotkeyFullScreen, win32.ModControl|win32.ModAlt|win32.ModNoRepeat, 'F') {
		logWarn("Ctrl+Alt+F 热键注册失败，只能用顶栏按钮切全屏")
	}
	// 先把自己登记进去：从这一步起，就算消息循环之后卡死，`localrdp
	// hosts`/`stop` 也能找到这个窗口（见 hosts.go）。
	s.registerSelf()

	// 抽干一次消息队列，避免尺寸消息早于控件创建到达。
	var msg win.MSG
	for win.PeekMessage(&msg, 0, 0, 0, win.PM_REMOVE) {
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}

	var created bool
	// 控件住在一个子窗口里：上面的栏是本进程画的，控件只管栏下面那块，
	// 四周再留一圈给鼠标拖拽改大小。
	bar := barHeightFor(&opt, s.m.dpi)
	grip := s.m.grip
	if !s.resizable || s.sizeMode == store.SizeFixed {
		grip = 0
	}
	s.view = win.CreateWindowEx(0, win32.UTF16Ptr("STATIC"), nil,
		win.WS_CHILD|win.WS_VISIBLE|win.WS_CLIPCHILDREN|win.WS_CLIPSIBLINGS,
		grip, bar, cw-2*grip, ch-bar-grip, s.hwnd, 0, inst, nil)
	if s.view == 0 {
		logWarn("创建控件容器失败: " + formatWin32(uint32(win.GetLastError())))
	}
	s.createOverlay(inst)
	for i := range clsidRdpClients {
		if err := s.host.create(&clsidRdpClients[i], s.view); err != nil {
			logWarn("创建 RDP 控件失败: " + err.Error())
			continue
		}
		created = true
		logf("RDP 控件已创建 clsid[%d]", i)
		break
	}
	if !created {
		s.fail(fault.ControlBroken, "创建控件失败")
		return int(ExitError)
	}
	if opt.ViewOnly {
		if w := s.host.controlWindow(); w != 0 {
			win.EnableWindow(w, false)
		}
	}
	s.host.onEvent = s.onEvent

	if !s.host.configure(&opt) {
		s.fail(fault.NoChildSupport, "")
		return int(ExitError)
	}
	if opt.Clipboard {
		startClipHost()
	}
	s.host.connect()

	if opt.TimeoutSec > 0 {
		win.SetTimer(s.hwnd, kTimerWatchdog, 1000, 0)
	}
	for {
		r := win.GetMessage(&msg, 0, 0, 0)
		if r == 0 || r == -1 {
			break
		}
		win.TranslateMessage(&msg)
		win.DispatchMessage(&msg)
	}
	go func() {
		sleepMS(6000)
		logError("清理超时，强制退出")
		// 清理卡住时也别留一个处于断开状态的子会话。
		s.forceCleanup("清理超时")
		os.Exit(int(ExitOk))
	}()
	logf("消息循环已退出，开始清理")
	// 还没断开就退出＝把半开的握手留给下一次连接，先干净地挂断。
	s.hangUpCleanly()
	s.unregisterSelf()
	stopClipHost()
	logf("剪辑板宿主已停止")
	if s.host != nil {
		s.host.release()
	}
	logf("控件已释放")
	gHost = nil

	sid := getChildSessionID()
	if sid != kNoSession && !opt.KeepSess {
		wtLogoffSession(sid, false)
		logf("Logged off child session %d.", sid)
	}
	if s.failed && !s.everConnected {
		// 具体原因已经由 fail() 说过一次了。
		return int(ExitConnectFailed)
	}
	if s.everConnected {
		printOut(describeDisconnect(s.discReason, s.extReason))
	}
	return int(ExitOk)
}
