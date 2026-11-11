package rdp

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"

	"localrdp/internal/store"
	"localrdp/internal/win32"
)

var (
	clsidRdpClients = []win.CLSID{
		{Data1: 0xa0c63c30, Data2: 0xf08d, Data3: 0x4ab4, Data4: [8]byte{0x90, 0x7c, 0x34, 0x90, 0x5d, 0x77, 0x0c, 0x7d}},
		{Data1: 0x8b918b82, Data2: 0x7985, Data3: 0x4c24, Data4: [8]byte{0x89, 0xdf, 0xc3, 0x3a, 0xd2, 0xbb, 0xfb, 0xcd}},
		{Data1: 0xa3bc03a0, Data2: 0x041d, Data3: 0x42e3, Data4: [8]byte{0xad, 0x22, 0x88, 0x2b, 0x78, 0x65, 0xc9, 0xc5}},
	}
	iidIMsRdpExtendedSettings = iid(0x302D8188, 0x0052, 0x4807, 0x80, 0x6A, 0x36, 0x2B, 0x62, 0x8F, 0x9A, 0xC5)
	iidNs3                    = iid(0xB3378D90, 0x0728, 0x45C7, 0x8E, 0xD7, 0xB6, 0x15, 0x9F, 0xB9, 0x22, 0x19)
	iidNs4                    = iid(0xF50FA8AA, 0x1C7D, 0x4F59, 0xB1, 0x5C, 0xA9, 0x0C, 0xAC, 0xAE, 0x1F, 0xCB)
	iidNs5                    = iid(0x4F6996D5, 0xD7B1, 0x412C, 0xB0, 0xFF, 0x06, 0x37, 0x18, 0x56, 0x69, 0x07)
	iidTscNonScriptable       = iid(0xC1E6743A, 0x41C1, 0x4A74, 0x83, 0x2A, 0x0D, 0xD0, 0x6C, 0x1C, 0x7A, 0x0E)
)

const (
	nsPutClearTextPassword = 3 // IMsTscNonScriptable::put_ClearTextPassword
)

// IMsRdpClientNonScriptable3 的 vtable 下标（IUnknown 3 + IMsTscNonScriptable 10 +
// IMsRdpClientNonScriptable 2 + ...2 2 = 17 起）。
const (
	nsPutShowRedirectionWarningDialog  = 17
	nsPutPromptForCredentials          = 19
	nsPutRedirectDynamicDrives         = 25
	nsGetDriveCollection               = 30
	nsPutWarnAboutSendingCredentials   = 31
	nsPutWarnAboutClipboardRedirection = 33
	nsPutConnectionBarText             = 35
)

// IMsRdpClientNonScriptable4 从 37 起。
const (
	ns4PutWarnAboutPrinterRedirection     = 43
	ns4PutLaunchedViaClientShellInterface = 49
	ns4PutTrustedZoneSite                 = 51
)

// IMsRdpClientNonScriptable5 从 53 起，AllowPromptingForCredentials 是它的第 11 个方法。
const (
	ns5PutAllowPromptingForCredentials = 63
)

// IMsRdpDriveCollection：RescanDrives=3, get_DriveByIndex=4, get_DriveCount=5
const (
	dcRescanDrives   = 3
	dcDriveByIndex   = 4
	dcDriveCount     = 5
	driveName        = 3
	drivePutRedirect = 4
	driveGetRedirect = 5
)

func vtCall(p unsafe.Pointer, index int, args ...uintptr) uintptr {
	if p == nil {
		return hrPointer
	}
	vtbl := *(*unsafe.Pointer)(p)
	fn := *(*uintptr)(unsafe.Add(vtbl, uintptr(index)*unsafe.Sizeof(uintptr(0))))
	all := make([]uintptr, 0, len(args)+1)
	all = append(all, uintptr(p))
	all = append(all, args...)
	r, _, _ := syscall.SyscallN(fn, all...)
	return r
}

func boolArg(b bool) uintptr {
	if b {
		return 1
	}
	return 0
}

// ---------------------------------------------------------------- 扩展属性

type extSettingsVtbl struct {
	QueryInterface, AddRef, Release, PutProperty, GetProperty uintptr
}

func (h *axHost) ext() (unsafe.Pointer, *extSettingsVtbl) {
	p := qi(h.ctrl, &iidIMsRdpExtendedSettings)
	if p == nil {
		return nil, nil
	}
	return p, *(**extSettingsVtbl)(p)
}

func (h *axHost) putExtended(name string, v variant) uintptr {
	p, vtb := h.ext()
	if p == nil {
		logf("IMsRdpExtendedSettings 缺失，无法设置 %s", name)
		return hrNoInterface
	}
	b := win.SysAllocString(name)
	r, _, _ := syscall.SyscallN(vtb.PutProperty, uintptr(p),
		uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(&v)))
	win.SysFreeString(b)
	variantClear(&v)
	syscall.SyscallN(vtb.Release, uintptr(p))
	return r
}

func (h *axHost) putExtendedBool(name string, b bool) uintptr { return h.putExtended(name, varBool(b)) }
func (h *axHost) putExtendedI4(name string, v int32) uintptr  { return h.putExtended(name, varI4(v)) }

func (h *axHost) getExtended(name string) (variant, uintptr) {
	var out variant
	p, vtb := h.ext()
	if p == nil {
		return out, hrNoInterface
	}
	b := win.SysAllocString(name)
	r, _, _ := syscall.SyscallN(vtb.GetProperty, uintptr(p),
		uintptr(unsafe.Pointer(b)), uintptr(unsafe.Pointer(&out)))
	win.SysFreeString(b)
	syscall.SyscallN(vtb.Release, uintptr(p))
	return out, r
}

// putClearTextPassword 通过 IMsTscNonScriptable 的 vtable 把密码交给控件。
// 这是唯一能让子会话自动登录的路径：该属性既不在 AdvancedSettings 上，
// 也不在主 IDispatch 的名字表里，晚期绑定只会拿到 0x80020006。
func (h *axHost) putClearTextPassword(pw string) uintptr {
	ns := qi(h.ctrl, &iidTscNonScriptable)
	if ns == nil {
		return hrNoInterface
	}
	defer vtCall(ns, 2)
	b := win.SysAllocString(pw)
	defer win.SysFreeString(b)
	return vtCall(ns, nsPutClearTextPassword, uintptr(unsafe.Pointer(b)))
}

// ---------------------------------------------------------------- 驱动重定向

func (h *axHost) enableRedirectedDrives() {
	ns3 := qi(h.ctrl, &iidNs3)
	if ns3 == nil {
		logWarn("QI IMsRdpClientNonScriptable3 失败，无法启用驱动器重定向")
		return
	}
	defer vtCall(ns3, 2)
	vtCall(ns3, nsPutShowRedirectionWarningDialog, boolArg(false))
	vtCall(ns3, nsPutWarnAboutClipboardRedirection, boolArg(false))
	vtCall(ns3, nsPutPromptForCredentials, boolArg(false))
	vtCall(ns3, nsPutWarnAboutSendingCredentials, boolArg(false))
	dyn := vtCall(ns3, nsPutRedirectDynamicDrives, boolArg(true))
	logf("RedirectDynamicDrives hr=%s", formatHr(uint32(dyn)))

	var col unsafe.Pointer
	hr := vtCall(ns3, nsGetDriveCollection, uintptr(unsafe.Pointer(&col)))
	if hrFailed(hr) || col == nil {
		logWarn("get_DriveCollection 失败 " + formatHr(uint32(hr)))
	} else {
		vtCall(col, dcRescanDrives, boolArg(false))
		var n uint32
		vtCall(col, dcDriveCount, uintptr(unsafe.Pointer(&n)))
		logf("DriveCount=%d", n)
		if n == 0 {
			vtCall(col, dcRescanDrives, boolArg(true))
			vtCall(col, dcDriveCount, uintptr(unsafe.Pointer(&n)))
			logf("DriveCount after dynamic rescan=%d", n)
		}
		for i := uint32(0); i < n; i++ {
			var d unsafe.Pointer
			if hr := vtCall(col, dcDriveByIndex, uintptr(i), uintptr(unsafe.Pointer(&d))); hrFailed(hr) || d == nil {
				logWarn("DriveByIndex " + win32.Itoa(int(i)) + " 失败 " + formatHr(uint32(hr)))
				continue
			}
			var nm uint16Ptr
			vtCall(d, driveName, uintptr(unsafe.Pointer(&nm)))
			putHr := vtCall(d, drivePutRedirect, boolArg(true))
			var st uint16
			vtCall(d, driveGetRedirect, uintptr(unsafe.Pointer(&st)))
			name := "?"
			if nm != 0 {
				name = u2s(ptrAt[uint16](nm))
				win.SysFreeString(ptrAt[uint16](nm))
			}
			state := "NOT redirected"
			if st != 0 {
				state = "redirected"
			}
			logf("Drive[%d] %s put=%s %s", i, name, formatHr(uint32(putHr)), state)
			vtCall(d, 2)
		}
		vtCall(col, 2)
	}

	if ns4 := qi(h.ctrl, &iidNs4); ns4 != nil {
		vtCall(ns4, ns4PutLaunchedViaClientShellInterface, boolArg(true))
		vtCall(ns4, ns4PutTrustedZoneSite, boolArg(true))
		vtCall(ns4, ns4PutWarnAboutPrinterRedirection, boolArg(false))
		vtCall(ns4, 2)
	}
}

type uint16Ptr = uintptr

// ---------------------------------------------------------------- 配置

func getFirstDispatch(d disp, names []string) (unsafe.Pointer, string) {
	for _, n := range names {
		if p, hr := d.getDispatch(n); hr == hrOK && p != nil {
			return p, n
		}
	}
	return nil, ""
}

func clientDesktopSize(hwnd win.HWND) (int32, int32) {
	var rc win.RECT
	if hwnd != 0 {
		win.GetClientRect(hwnd, &rc)
	}
	w := rc.Right - rc.Left
	h := rc.Bottom - rc.Top
	if w < 200 {
		w = 200
	}
	w &^= 1
	if h < 200 {
		h = 200
	}
	return w, h
}

// viewSize is the desktop the control should use: the client area of the
// window the control lives in, which already leaves the bar out, or what the
// caller asked for while the window is minimized or not laid out yet (a
// window started with --minimized reports 0x0, and the session would open at
// 200x200).
func viewSize(hwnd win.HWND, opt *StartOptions) (int32, int32) {
	w, h := clientDesktopSize(hwnd)
	if w >= kMinDesktopW && h >= kMinDesktopH {
		return w, h
	}
	if opt != nil && opt.Width >= kMinDesktopW && opt.Height >= kMinDesktopH {
		return int32(opt.Width), int32(opt.Height)
	}
	return 1280, 800
}

// resolveScale is the desktop scale the session starts with: --scale when
// it is given, 100 otherwise.
//
// The window is already DPI aware (the process is per-monitor v2), so a
// screen at 150% gives a window with 1.5 times the pixels, and the session
// takes the window's pixels as its desktop. Handing the session the DPI
// factor too made the guest lay out at 150% in that same pixel grid and then
// SmartSizing stretched the result, which is the picture growing twice.
// A machine that wants bigger text in the child session says so: --scale.
func resolveScale(_ win.HWND, opt *StartOptions) (int32, int32) {
	pct := int32(opt.Scale)
	if pct <= 0 {
		pct = 100
	}
	if pct < 100 {
		pct = 100
	}
	if pct > 500 {
		pct = 500
	}
	return pct, deviceScaleFor(pct)
}

// deviceScaleFor maps a desktop scale factor onto the device scale factor the
// session reports back to the guest.
func deviceScaleFor(pct int32) int32 {
	switch {
	case pct >= 200:
		return 180
	case pct >= 125:
		return 140
	}
	return 100
}

// configure 对应 src/rdp_host.cpp 的 ConfigureControl()。
func (h *axHost) configure(opt *StartOptions) bool {
	d := disp{h.disp}
	desk_w, desk_h := viewSize(h.hwnd, opt)
	put := func(name string, v variant) uintptr {
		hr := d.put(name, v)
		if hr != hrOK {
			logf("属性 %s 设置失败 %s", name, formatHr(uint32(hr)))
		}
		return hr
	}

	logf("Fullscreen shortcut: native Ctrl+Alt+F; host initial state=%v", opt.Fullscreen)
	put("Server", varBstr("localhost"))
	put("DesktopWidth", varI4(desk_w))
	put("DesktopHeight", varI4(desk_h))
	put("ColorDepth", varI4(32))

	adv, advName := getFirstDispatch(d, []string{"AdvancedSettings9", "AdvancedSettings8",
		"AdvancedSettings7", "AdvancedSettings6", "AdvancedSettings2", "AdvancedSettings"})
	sec, _ := getFirstDispatch(d, []string{"SecuredSettings3", "SecuredSettings2", "SecuredSettings"})
	if adv == nil {
		logWarn("找不到 AdvancedSettings 对象")
	} else {
		logf("使用 %s", advName)
	}
	advD := disp{adv}
	advPut := func(name string, v variant) uintptr {
		if adv == nil {
			variantClear(&v)
			return hrNoInterface
		}
		hr := advD.put(name, v)
		if hr != hrOK {
			logf("高级属性 %s 设置失败 %s", name, formatHr(uint32(hr)))
		}
		return hr
	}
	advGet := func(name string) {
		if adv == nil {
			return
		}
		if v, hr := advD.get(name); hr == hrOK {
			logf("%s = %s", name, variantString(&v))
			variantClear(&v)
		}
	}

	advPut("EnableCredSspSupport", varBool(true))
	advPut("AuthenticationLevel", varI4(0))
	hrFull := advPut("ContainerHandledFullScreen", varI4(1))
	logf("ContainerHandledFullScreen set hr=%s", formatHr(uint32(hrFull)))
	advGet("ContainerHandledFullScreen")
	hrHot := advPut("HotKeyFullScreen", varI4('F'))
	logf("HotKeyFullScreen set hr=%s", formatHr(uint32(hrHot)))
	advGet("HotKeyFullScreen")

	// 连接栏标题走 IMsRdpClientNonScriptable3。
	barHr := uintptr(hrNoInterface)
	if ns3 := qi(h.ctrl, &iidNs3); ns3 != nil {
		t := win.SysAllocString(opt.Title)
		barHr = vtCall(ns3, nsPutConnectionBarText, uintptr(unsafe.Pointer(t)))
		win.SysFreeString(t)
		vtCall(ns3, 2)
	}
	logf("ConnectionBarText set hr=%s", formatHr(uint32(barHr)))
	advPut("DisplayConnectionBar", varBool(true))
	advPut("ConnectionBarShowPinButton", varBool(true))
	advGet("DisplayConnectionBar")
	advGet("ConnectionBarShowPinButton")

	// RDPDR 必须保持开启（驱动器映射与 CLIPRDR 文件流都依赖它）。
	advPut("DisableRdpdr", varBool(false))
	advPut("RedirectDrives", varBool(opt.Drives))
	advPut("RedirectClipboard", varBool(opt.Clipboard))
	advPut("RedirectPrinters", varBool(false))
	advPut("RedirectPorts", varBool(false))
	advPut("RedirectSmartCards", varBool(false))
	advPut("RedirectDevices", varBool(false))
	advPut("RedirectPOSDevices", varBool(false))
	// SmartSizing fits the session picture into the control, whatever the
	// session desktop is — downwards only: a smaller window shrinks the whole
	// picture, a bigger one leaves it at 1:1 in the middle. The control has no
	// zoom property at all, so scale mode magnifies nothing by itself (raising
	// --scale is what makes text bigger). Follow-window and scale both want it,
	// fixed keeps the window at the session size, so it has nothing to fit.
	//
	// It goes in as a LONG: the property is a LONG, and a BOOL from the
	// script interface does not set it.
	stretches := store.NormalizeSizeMode(opt.SizeMode) != store.SizeFixed
	advPut("SmartSizing", varI4(boolI4(stretches)))
	advPut("EnableAutoReconnect", varBool(false))
	advPut("MaxReconnectAttempts", varI4(0))
	advPut("PerformanceFlags", varI4(0x00000190))
	advPut("GrabFocusOnConnect", varBool(!opt.ViewOnly && !opt.Hidden))
	advPut("allowBackgroundInput", varI4(0))
	advPut("NegotiateSecurityLayer", varBool(true))
	// 凭据本身与“要不要弹凭据框”都只能走非脚本接口的 vtable。
	if ns3 := qi(h.ctrl, &iidNs3); ns3 != nil {
		hr := vtCall(ns3, nsPutPromptForCredentials, boolArg(false))
		logf("PromptForCredentials=false hr=%s", formatHr(uint32(hr)))
		vtCall(ns3, 2)
	} else {
		logWarn("QI IMsRdpClientNonScriptable3 失败：关不掉凭据提示")
	}

	if ns5 := qi(h.ctrl, &iidNs5); ns5 != nil {
		hr := vtCall(ns5, ns5PutAllowPromptingForCredentials, boolArg(false))
		logf("AllowPromptingForCredentials=false hr=%s", formatHr(uint32(hr)))
		vtCall(ns5, 2)
	}

	if opt.User != "" {
		put("UserName", varBstr(opt.User))
	}
	if opt.Password != "" {
		hr := h.putClearTextPassword(opt.Password)
		logf("ClearTextPassword（长度 %d）hr=%s", len(opt.Password), formatHr(uint32(hr)))
		if hrFailed(hr) {
			logWarn("密码没能交给 RDP 控件，子会话会停在登录页")
		}
	} else if opt.User != "" {
		logWarn("没有密码：子会话会停在登录页（用 --password-env，或在界面里保存密码）")
	}
	if sec != nil {
		if hr := (disp{sec}).put("KeyboardHookMode", varI4(1)); hr != hrOK {
			logf("KeyboardHookMode 设置失败 %s", formatHr(uint32(hr)))
		}
	}

	advGet("DisableRdpdr")
	advGet("RedirectDrives")
	advGet("RedirectClipboard")

	if opt.Drives {
		h.enableRedirectedDrives()
		logInfo("Drive redirection requested (RDPDR + DriveCollection)")
	}
	if opt.Clipboard {
		logInfo("CLIPRDR 已启用；文件复制走环回 CF_HDROP 桥")
	}
	if adv != nil {
		vtCall(adv, 2)
	}
	if sec != nil {
		vtCall(sec, 2)
	}

	if hr := h.putExtendedBool("ConnectToChildSession", true); hr != hrOK {
		printErr("Failed to set ConnectToChildSession. Use the NotSafeForScripting RDP control.")
		logError("ConnectToChildSession 设置失败 " + formatHr(uint32(hr)))
		return false
	}
	if v, hr := h.getExtended("ConnectToChildSession"); true {
		logf("ConnectToChildSession get hr=%s vt=%d value=%s",
			formatHr(uint32(hr)), v.VT, variantString(&v))
		variantClear(&v)
	}
	h.putExtendedBool("EnableHardwareMode", true)
	h.putExtendedBool("EnableFrameBufferRedirection", true)

	ds, device := resolveScale(h.hwnd, opt)
	if store.NormalizeSizeMode(opt.SizeMode) != store.SizeScale {
		// Only the scale mode carries a --scale: fit works in the window's
		// own pixels and fixed keeps what it connected with, so a scale on
		// either of them would magnify the picture a second time.
		ds, device = 100, deviceScaleFor(100)
	}
	h.putExtendedI4("DesktopScaleFactor", ds)
	h.putExtendedI4("DeviceScaleFactor", device)

	h.appliedW, h.appliedH, h.appliedScale = int(desk_w), int(desk_h), int(ds)
	logf("Initial desktop %dx%d @%d%% size=%s", desk_w, desk_h, ds, store.NormalizeSizeMode(opt.SizeMode))
	return true
}

func (h *axHost) connect() uintptr {
	hr := (disp{h.disp}).call("Connect")
	if hr != hrOK {
		logf("Connect() 失败 %s", formatHr(uint32(hr)))
	} else {
		logInfo("Connect() 已发出")
	}
	return hr
}

func (h *axHost) setFullScreen(on bool) {
	if h.disp == nil {
		return
	}
	if hr := (disp{h.disp}).put("FullScreen", varBool(on)); hr != hrOK {
		logf("FullScreen=%v 设置失败 %s", on, formatHr(uint32(hr)))
	}
}

func describeDisconnect(reason, extended int32) string {
	switch extended {
	case 5:
		return "The child session was taken over by another connection (only one is allowed)."
	case 2, 12:
		return "The child session was logged off."
	}
	switch uint32(reason) {
	case 0x1:
		return "Disconnected locally."
	case 0x2:
		return "Logged off inside the child session."
	case 0x3:
		return "Session ended by the system."
	case 0x108:
		return "Connection timed out."
	case 0x204:
		return "Cannot connect: the child-session transport is not available."
	case 0x406:
		return "Invalid security data."
	case 0x706:
		return "Loopback certificate check failed."
	case 0x807:
		return "Logon failed: credentials rejected."
	case 0x904:
		return "Connection closed."
	case 0x1607, 0x1707:
		return "Credential delegation is blocked by policy; cannot auto-logon."
	}
	return sprintf("disconnect reason=0x%X extended=%d", uint32(reason), extended)
}

var eventNames = map[int32]string{
	1:  "OnConnecting",
	2:  "OnConnected",
	3:  "OnLogin",
	4:  "OnDisconnected",
	8:  "OnEnterFullScreenMode",
	9:  "OnLeaveFullScreenMode",
	10: "OnFatalError",
	12: "OnRemoteDesktopSizeChange",
	14: "OnRequestContainerMinimize",
	15: "OnConfirmClose",
}

func describeEvent(id int32, args []variant) string {
	name := eventNames[id]
	if name == "" {
		name = "dispid" + win32.Itoa(int(id))
	}
	s := name
	for i := range args {
		s += " " + variantString(&args[i])
	}
	return s
}
