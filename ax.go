package main

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/win"
)

const (
	oleIverbInPlaceActivate = -5
	oleIverbUIActivate      = -4
)

func iid(d1 uint32, d2, d3 uint16, rest ...byte) win.IID {
	g := win.IID{Data1: d1, Data2: d2, Data3: d3}
	copy(g.Data4[:], rest)
	return g
}

var (
	iidIOleInPlaceFrame        = iid(0x00000116, 0, 0, 0xC0, 0, 0, 0, 0, 0, 0, 0x46)
	iidIOleInPlaceUIWindow     = iid(0x00000115, 0, 0, 0xC0, 0, 0, 0, 0, 0, 0, 0x46)
	iidIOleInPlaceActiveObject = iid(0x00000117, 0, 0, 0xC0, 0, 0, 0, 0, 0, 0, 0x46)
	iidIOleControlSite         = iid(0xB196B289, 0xBAB4, 0x101A, 0xB6, 0x9C, 0x00, 0xAA, 0x00, 0x34, 0x1D, 0x07)
	iidIOleInPlaceSiteEx       = iid(0x9C2CAD80, 0x3424, 0x11CF, 0xB6, 0x70, 0x00, 0xAA, 0x00, 0x4C, 0xD6, 0xD8)
	iidIDocHostUIHandler       = iid(0xBD3F23C0, 0xD43E, 0x11CF, 0x89, 0x3B, 0x00, 0xAA, 0x00, 0xBD, 0xCE, 0x1A)
	iidIServiceProvider        = iid(0x6D5140C1, 0x7436, 0x11CE, 0x80, 0x34, 0x00, 0xAA, 0x00, 0x60, 0x09, 0xFA)
	iidIStorage                = iid(0x0000000B, 0, 0, 0xC0, 0, 0, 0, 0, 0, 0, 0x46)
	iidIPersistStreamInit      = iid(0x7FD52380, 0x4E07, 0x101B, 0xAE, 0x2D, 0x08, 0x00, 0x2B, 0x2E, 0xC7, 0x13)
	diidMsTscAxEvents          = iid(0x336D5562, 0xEFA8, 0x482E, 0x8C, 0xB3, 0xC5, 0xC0, 0xFC, 0x7A, 0x7D, 0xB6)
)

func guidString(g *win.IID) string {
	return fmt.Sprintf("%08X-%04X-%04X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		g.Data1, g.Data2, g.Data3,
		g.Data4[0], g.Data4[1], g.Data4[2], g.Data4[3],
		g.Data4[4], g.Data4[5], g.Data4[6], g.Data4[7])
}

// ---------------------------------------------------------------- 子对象

type clientSite struct {
	lp *win.IOleClientSiteVtbl
	h  *axHost
}

type inPlaceSite struct {
	lp *win.IOleInPlaceSiteVtbl
	h  *axHost
}

type inPlaceFrame struct {
	lp *win.IOleInPlaceFrameVtbl
	h  *axHost
}

type evSink struct {
	lp *win.IDispatchVtbl
	h  *axHost
}

type inPlaceSiteEx struct {
	lp *inPlaceSiteExVtbl
	h  *axHost
}

type controlSite struct {
	lp *controlSiteVtbl
	h  *axHost
}

// IOleInPlaceSiteEx = IOleInPlaceSite(15) + 3
type inPlaceSiteExVtbl struct {
	QueryInterface, AddRef, Release       uintptr
	GetWindow, ContextSensitiveHelp       uintptr
	CanInPlaceActivate, OnInPlaceActivate uintptr
	OnUIActivate, GetWindowContext        uintptr
	Scroll, OnUIDeactivate                uintptr
	OnInPlaceDeactivate, DiscardUndoState uintptr
	DeactivateAndUndo, OnPosRectChange    uintptr
	OnInPlaceActivateEx, OnInPlaceDeactivateEx, RequestUIActivate uintptr
}

// IOleControlSite
 type controlSiteVtbl struct {
	QueryInterface, AddRef, Release             uintptr
	OnControlInfoChanged, LockInPlaceActive     uintptr
	GetExtendedControl                          uintptr
	TransformCoords, TranslateAccelerator       uintptr
	OnFocus, ShowPropertyFrame                  uintptr
}

// InitNew 在 SetClientSite 之前调用会让 mstscax 崩溃（0xc0000005@0x8），默认跳过。
var noInitNew = true

type axHost struct {
	cs     clientSite
	ips    inPlaceSite
	ipf    inPlaceFrame
	ev     evSink
	ipsEx  inPlaceSiteEx
	ctl    controlSite
	hwnd   win.HWND
	ctrl   unsafe.Pointer // 控件自身 IUnknown，对应 C++ 的 st->control
	ole    *win.IOleObject
	ipo    *win.IOleInPlaceObject
	cp     *win.IConnectionPoint
	cookie uint32
	disp   unsafe.Pointer
	onEvent func(id int32, args []variant)

	appliedW, appliedH, appliedScale int
}

// ---------------------------------------------------------------- vtable

var (
	clientSiteVtbl = &win.IOleClientSiteVtbl{
		syscall.NewCallback(csQueryInterface),
		syscall.NewCallback(csAddRef),
		syscall.NewCallback(csRelease),
		syscall.NewCallback(csSaveObject),
		syscall.NewCallback(csGetMoniker),
		syscall.NewCallback(csGetContainer),
		syscall.NewCallback(csShowObject),
		syscall.NewCallback(csOnShowWindow),
		syscall.NewCallback(csRequestNewObjectLayout),
	}
	inPlaceSiteVtbl = &win.IOleInPlaceSiteVtbl{
		syscall.NewCallback(ipsQueryInterface),
		syscall.NewCallback(ipsAddRef),
		syscall.NewCallback(ipsRelease),
		syscall.NewCallback(ipsGetWindow),
		syscall.NewCallback(ipsContextSensitiveHelp),
		syscall.NewCallback(ipsCanInPlaceActivate),
		syscall.NewCallback(ipsOnInPlaceActivate),
		syscall.NewCallback(ipsOnUIActivate),
		syscall.NewCallback(ipsGetWindowContext),
		syscall.NewCallback(ipsScroll),
		syscall.NewCallback(ipsOnUIDeactivate),
		syscall.NewCallback(ipsOnInPlaceDeactivate),
		syscall.NewCallback(ipsDiscardUndoState),
		syscall.NewCallback(ipsDeactivateAndUndo),
		syscall.NewCallback(ipsOnPosRectChange),
	}
	inPlaceFrameVtbl = &win.IOleInPlaceFrameVtbl{
		syscall.NewCallback(ipfQueryInterface),
		syscall.NewCallback(ipfAddRef),
		syscall.NewCallback(ipfRelease),
		syscall.NewCallback(ipfGetWindow),
		syscall.NewCallback(ipfContextSensitiveHelp),
		syscall.NewCallback(ipfGetBorder),
		syscall.NewCallback(ipfRequestBorderSpace),
		syscall.NewCallback(ipfSetBorderSpace),
		syscall.NewCallback(ipfSetActiveObject),
		syscall.NewCallback(ipfInsertMenus),
		syscall.NewCallback(ipfSetMenu),
		syscall.NewCallback(ipfRemoveMenus),
		syscall.NewCallback(ipfSetStatusText),
		syscall.NewCallback(ipfEnableModeless),
		syscall.NewCallback(ipfTranslateAccelerator),
	}
	evSinkVtbl = &win.IDispatchVtbl{
		syscall.NewCallback(evQueryInterface),
		syscall.NewCallback(evAddRef),
		syscall.NewCallback(evRelease),
		syscall.NewCallback(evGetTypeInfoCount),
		syscall.NewCallback(evGetTypeInfo),
		syscall.NewCallback(evGetIDsOfNames),
		syscall.NewCallback(evInvoke),
	}
	inPlaceSiteExVtblVar = &inPlaceSiteExVtbl{
		syscall.NewCallback(ipsExQueryInterface),
		syscall.NewCallback(ipsAddRef2),
		syscall.NewCallback(ipsRelease2),
		syscall.NewCallback(ipsGetWindow2),
		syscall.NewCallback(ipsContextSensitiveHelp2),
		syscall.NewCallback(ipsCanInPlaceActivate2),
		syscall.NewCallback(ipsOnInPlaceActivate2),
		syscall.NewCallback(ipsOnUIActivate2),
		syscall.NewCallback(ipsGetWindowContext2),
		syscall.NewCallback(ipsScroll2),
		syscall.NewCallback(ipsOnUIDeactivate2),
		syscall.NewCallback(ipsOnInPlaceDeactivate2),
		syscall.NewCallback(ipsDiscardUndoState2),
		syscall.NewCallback(ipsDeactivateAndUndo2),
		syscall.NewCallback(ipsOnPosRectChange2),
		syscall.NewCallback(ipsExOnInPlaceActivateEx),
		syscall.NewCallback(ipsExOnInPlaceDeactivateEx),
		syscall.NewCallback(ipsExRequestUIActivate),
	}
	controlSiteVtblVar = &controlSiteVtbl{
		syscall.NewCallback(ctlQueryInterface),
		syscall.NewCallback(ipsAddRef2),
		syscall.NewCallback(ipsRelease2),
		syscall.NewCallback(ctlOnControlInfoChanged),
		syscall.NewCallback(ctlLockInPlaceActive),
		syscall.NewCallback(ctlGetExtendedControl),
		syscall.NewCallback(ctlTransformCoords),
		syscall.NewCallback(ctlTranslateAccelerator),
		syscall.NewCallback(ctlOnFocus),
		syscall.NewCallback(ctlShowPropertyFrame),
	}
)

// ---------------------------------------------------------------- client site

func csQueryInterface(self *clientSite, riid win.REFIID, ppv *unsafe.Pointer) uintptr {
	h := self.h
	switch {
	case win.EqualREFIID(riid, &win.IID_IUnknown), win.EqualREFIID(riid, &win.IID_IOleClientSite):
		*ppv = unsafe.Pointer(self)
	case win.EqualREFIID(riid, &win.IID_IOleInPlaceSite):
		*ppv = unsafe.Pointer(&h.ips)
	case win.EqualREFIID(riid, &iidIOleInPlaceFrame):
		*ppv = unsafe.Pointer(&h.ipf)
	case win.EqualREFIID(riid, &diidMsTscAxEvents):
		*ppv = unsafe.Pointer(&h.ev)
	case win.EqualREFIID(riid, &iidIOleInPlaceSiteEx):
		*ppv = unsafe.Pointer(&h.ipsEx)
	case win.EqualREFIID(riid, &iidIOleControlSite):
		*ppv = unsafe.Pointer(&h.ctl)
	default:
		logf("clientSite QI 未支持 %s", guidString(riid))
		*ppv = nil
		return hrNoInterface
	}
	return hrOK
}

func csAddRef(self *clientSite) uintptr   { return 1 }
func csRelease(self *clientSite) uintptr  { return 1 }
func csSaveObject(self *clientSite) uintptr { return hrNotImpl }

func csGetMoniker(self *clientSite, assign, which uint32, ppmk *unsafe.Pointer) uintptr {
	return hrNotImpl
}

func csGetContainer(self *clientSite, pp *unsafe.Pointer) uintptr {
	*pp = nil
	return hrNoInterface
}

func csShowObject(self *clientSite) uintptr { return hrOK }

func csOnShowWindow(self *clientSite, show win.BOOL) uintptr { return hrNotImpl }

func csRequestNewObjectLayout(self *clientSite) uintptr { return hrNotImpl }

// ---------------------------------------------------------------- in place site

func ipsQueryInterface(self *inPlaceSite, riid win.REFIID, ppv *unsafe.Pointer) uintptr {
	return csQueryInterface(&self.h.cs, riid, ppv)
}

func ipsAddRef(self *inPlaceSite) uintptr  { return 1 }
func ipsRelease(self *inPlaceSite) uintptr { return 1 }

func ipsGetWindow(self *inPlaceSite, lphwnd *win.HWND) uintptr {
	*lphwnd = self.h.hwnd
	return hrOK
}

func ipsContextSensitiveHelp(self *inPlaceSite, enter win.BOOL) uintptr { return hrNotImpl }
func ipsCanInPlaceActivate(self *inPlaceSite) uintptr                   { return hrOK }
func ipsOnInPlaceActivate(self *inPlaceSite) uintptr                    { return hrOK }
func ipsOnUIActivate(self *inPlaceSite) uintptr                         { return hrOK }

func ipsGetWindowContext(self *inPlaceSite, frame **inPlaceFrame, doc *unsafe.Pointer,
	pos, clip *win.RECT, info *win.OLEINPLACEFRAMEINFO) uintptr {
	*frame = &self.h.ipf
	*doc = nil
	if info != nil {
		info.Cb = uint32(unsafe.Sizeof(win.OLEINPLACEFRAMEINFO{}))
		info.FMDIApp = win.FALSE
		info.HwndFrame = self.h.hwnd
		info.Haccel = 0
		info.CAccelEntries = 0
	}
	return hrOK
}

func ipsScroll(self *inPlaceSite, dx, dy int32) uintptr { return hrNotImpl }
func ipsOnUIDeactivate(self *inPlaceSite, undoable win.BOOL) uintptr { return hrOK }
func ipsOnInPlaceDeactivate(self *inPlaceSite) uintptr               { return hrOK }
func ipsDiscardUndoState(self *inPlaceSite) uintptr                  { return hrNotImpl }
func ipsDeactivateAndUndo(self *inPlaceSite) uintptr                 { return hrNotImpl }

func ipsOnPosRectChange(self *inPlaceSite, rc *win.RECT) uintptr {
	ipo := self.h.ipo
	if ipo == nil {
		return hrOK
	}
	return uintptr(ipo.SetObjectRects(rc, rc))
}

// ---------------------------------------------------------------- in place frame

func ipfQueryInterface(self *inPlaceFrame, riid win.REFIID, ppv *unsafe.Pointer) uintptr {
	return csQueryInterface(&self.h.cs, riid, ppv)
}

func ipfAddRef(self *inPlaceFrame) uintptr  { return 1 }
func ipfRelease(self *inPlaceFrame) uintptr { return 1 }

func ipfGetWindow(self *inPlaceFrame, lphwnd *win.HWND) uintptr {
	*lphwnd = self.h.hwnd
	return hrOK
}

func ipfContextSensitiveHelp(self *inPlaceFrame, enter win.BOOL) uintptr { return hrNotImpl }
func ipfGetBorder(self *inPlaceFrame, rc *win.RECT) uintptr              { return hrNotImpl }
func ipfRequestBorderSpace(self *inPlaceFrame, bs unsafe.Pointer) uintptr { return hrNotImpl }
func ipfSetBorderSpace(self *inPlaceFrame, bs unsafe.Pointer) uintptr     { return hrNotImpl }

func ipfSetActiveObject(self *inPlaceFrame, obj unsafe.Pointer, name *uint16) uintptr {
	return hrOK
}

func ipfInsertMenus(self *inPlaceFrame, menu win.HMENU, widths unsafe.Pointer) uintptr {
	return hrNotImpl
}

func ipfSetMenu(self *inPlaceFrame, menu win.HMENU, holemenu uintptr, active win.HWND) uintptr {
	return hrNotImpl
}

func ipfRemoveMenus(self *inPlaceFrame, menu win.HMENU) uintptr { return hrNotImpl }

func ipfSetStatusText(self *inPlaceFrame, text *uint16) uintptr { return hrOK }

func ipfEnableModeless(self *inPlaceFrame, enable win.BOOL) uintptr { return hrOK }

func ipfTranslateAccelerator(self *inPlaceFrame, msg *win.MSG, id uint16) uintptr {
	return 1 // S_FALSE
}

// ---------------------------------------------------------------- event sink

func evQueryInterface(self *evSink, riid win.REFIID, ppv *unsafe.Pointer) uintptr {
	switch {
	case win.EqualREFIID(riid, &win.IID_IUnknown), win.EqualREFIID(riid, &win.IID_IDispatch),
		win.EqualREFIID(riid, &diidMsTscAxEvents):
		*ppv = unsafe.Pointer(self)
		return hrOK
	}
	logf("eventSink QI 未支持 %s", guidString(riid))
	*ppv = nil
	return hrNoInterface
}

func evAddRef(self *evSink) uintptr  { return 1 }
func evRelease(self *evSink) uintptr { return 1 }

func evGetTypeInfoCount(self *evSink, n *uint32) uintptr {
	if n != nil {
		*n = 0
	}
	return hrOK
}

func evGetTypeInfo(self *evSink, i uint32, lcid uint32, pp unsafe.Pointer) uintptr { return hrNotImpl }
func evGetIDsOfNames(self *evSink, riid win.REFIID, names *uint16, n uint32, lcid uint32, ids *int32) uintptr {
	return hrNotImpl
}

func evInvoke(self *evSink, id int32, riid win.REFIID, lcid uint32, flags uint16,
	params *win.DISPPARAMS, result *variant, excep unsafe.Pointer, argErr *uint32) uintptr {

	var args []variant
	if params != nil && params.CArgs > 0 {
		n := int(params.CArgs)
		raw := (*[16]variant)(unsafe.Pointer(params.Rgvarg))
		if n > 16 {
			n = 16
		}
		for i := 0; i < n; i++ {
			args = append(args, raw[n-1-i])
		}
	}

	switch id {
	case 15: // OnRequestContainerMinimize 的应答
		if result != nil {
			*result = varBool(true)
		}
	}

	if self.h.onEvent != nil {
		self.h.onEvent(id, args)
	} else {
		logf("事件 dispid=%d args=%d", id, len(args))
	}
	return hrOK
}

// ---------------------------------------------------------------- 宿主

func newAxHost() *axHost {
	h := &axHost{}
	h.cs = clientSite{lp: clientSiteVtbl, h: h}
	h.ips = inPlaceSite{lp: inPlaceSiteVtbl, h: h}
	h.ipf = inPlaceFrame{lp: inPlaceFrameVtbl, h: h}
	h.ev = evSink{lp: evSinkVtbl, h: h}
	h.ipsEx = inPlaceSiteEx{lp: inPlaceSiteExVtblVar, h: h}
	h.ctl = controlSite{lp: controlSiteVtblVar, h: h}
	return h
}

type persistStreamInit struct{ lp *persistStreamInitVtbl }

type persistStreamInitVtbl struct {
	QueryInterface uintptr
	AddRef         uintptr
	Release        uintptr
	GetClassID     uintptr
	IsDirty        uintptr
	InitNew        uintptr
	Load           uintptr
	Save           uintptr
	GetSizeMax     uintptr
}

func (h *axHost) create(clsid *win.CLSID, hwnd win.HWND) error {
	var cfPtr unsafe.Pointer
	hr := win.CoGetClassObject((*win.CLSID)(clsid),
		win.CLSCTX_INPROC_SERVER|win.CLSCTX_INPROC_HANDLER,
		nil, &win.IID_IClassFactory, &cfPtr)
	if hr < 0 {
		return fmt.Errorf("CoGetClassObject 失败 hr=0x%08X", uint32(hr))
	}
	cf := (*win.IClassFactory)(cfPtr)
	var objPtr unsafe.Pointer
	hr = cf.CreateInstance(nil, &win.IID_IOleObject, &objPtr)
	cf.Release()
	if hr < 0 || objPtr == nil {
		return fmt.Errorf("CreateInstance 失败 hr=0x%08X", uint32(hr))
	}

	h.hwnd = hwnd
	h.ctrl = objPtr
	h.ole = (*win.IOleObject)(objPtr)
	cs := (*win.IOleClientSite)(unsafe.Pointer(&h.cs))

	logf("步骤: CreateInstance ok obj=%p", objPtr)

	// IPersistStreamInit::InitNew（ATL 的 CAxWindow 也这么做）
	var psiPtr unsafe.Pointer
	if hr := h.ole.QueryInterface(&iidIPersistStreamInit, &psiPtr); hr >= 0 && psiPtr != nil {
		psi := (*persistStreamInit)(psiPtr)
		logf("步骤: QI IPersistStreamInit ok obj=%p vtbl=%p InitNew=%p", psiPtr, psi.lp, unsafe.Pointer(psi.lp.InitNew))
		if !noInitNew {
			r, _, _ := syscall.SyscallN(psi.lp.InitNew, uintptr(psiPtr))
			logf("步骤: InitNew ok hr=0x%08X", uint32(r))
		}
		syscall.SyscallN(psi.lp.Release, uintptr(psiPtr))
	} else {
		logf("步骤: QI IPersistStreamInit hr=0x%08X", uint32(hr))
	}

	logf("步骤: SetClientSite 前 cs=%p", unsafe.Pointer(cs))
	if hr := h.ole.SetClientSite(cs); hr < 0 {
		return fmt.Errorf("SetClientSite 失败 hr=0x%08X", uint32(hr))
	}
	if hr := win.OleSetContainedObject((*win.IUnknown)(objPtr), true); hr < 0 {
		logf("OleSetContainedObject hr=0x%08X", uint32(hr))
	}

	var rc win.RECT
	win.GetClientRect(hwnd, &rc)
	for _, verb := range []int32{oleIverbInPlaceActivate, oleIverbUIActivate} {
		hr = h.ole.DoVerb(verb, nil, cs, 0, hwnd, &rc)
		logf("DoVerb(%d) hr=0x%08X", verb, uint32(hr))
	}

	var p unsafe.Pointer
	if hr := h.ole.QueryInterface(&win.IID_IOleInPlaceObject, &p); hr >= 0 {
		h.ipo = (*win.IOleInPlaceObject)(p)
		// 控件建窗口后必须立刻告诉它正确尺寸，否则它会用未初始化的矩形（实测高度 0、x 为负）
		h.resize(rc.Right-rc.Left, rc.Bottom-rc.Top)
	} else {
		logf("QI IOleInPlaceObject hr=0x%08X", uint32(hr))
	}
	if hr := h.ole.QueryInterface(&win.IID_IDispatch, &p); hr >= 0 {
		h.disp = p
	} else {
		return fmt.Errorf("控制无 IDispatch hr=0x%08X", uint32(hr))
	}

	var cpcPtr unsafe.Pointer
	if hr := h.ole.QueryInterface(&win.IID_IConnectionPointContainer, &cpcPtr); hr >= 0 {
		cpc := (*win.IConnectionPointContainer)(cpcPtr)
		var cp *win.IConnectionPoint
		if hr := cpc.FindConnectionPoint(&diidMsTscAxEvents, &cp); hr >= 0 {
			h.cp = cp
			if hr := cp.Advise(unsafe.Pointer(&h.ev), &h.cookie); hr >= 0 {
				logf("事件已连接 cookie=%d", h.cookie)
			} else {
				logf("Advise hr=0x%08X", uint32(hr))
			}
		} else {
			logf("FindConnectionPoint hr=0x%08X", uint32(hr))
		}
		cpc.Release()
	} else {
		logf("QI IConnectionPointContainer hr=0x%08X", uint32(hr))
	}
	return nil
}

// qi 对任意 IUnknown 指针做 QueryInterface，失败返回 nil。
func qi(p unsafe.Pointer, iid *win.IID) unsafe.Pointer {
	if p == nil {
		return nil
	}
	var out unsafe.Pointer
	r := vtCall(p, 0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&out)))
	if hrFailed(r) {
		return nil
	}
	return out
}

// controlWindow 返回控件自建窗口的 HWND（IOleWindow::GetWindow）。
func (h *axHost) controlWindow() win.HWND {
	if h.ipo == nil {
		return 0
	}
	var w win.HWND
	vtCall(unsafe.Pointer(h.ipo), 3, uintptr(unsafe.Pointer(&w)))
	return w
}

// release 释放控件对象（对应 C++ 里 ipao/disp/control 的 Release）。
func (h *axHost) release() {
	if h.ipo != nil {
		h.ipo.Release()
		h.ipo = nil
	}
	if h.disp != nil {
		vtCall(h.disp, 2)
		h.disp = nil
	}
	if h.ole != nil {
		h.ole.Release()
		h.ole = nil
	}
	h.ctrl = nil
}

func (h *axHost) resize(width, height int32) {
	if h.ipo == nil {
		return
	}
	rc := win.RECT{Right: width, Bottom: height}
	h.ipo.SetObjectRects(&rc, &rc)
}

func (h *axHost) disconnect() {
	if h.disp != nil {
		if hr := (disp{h.disp}).call("Disconnect"); hr != 0 {
			logf("Disconnect() hr=0x%08X", uint32(hr))
		}
		logf("已发送 Disconnect，等待控件收尾")
		for i := 0; i < 30; i++ {
			time.Sleep(100 * time.Millisecond)
		}
	}
	if h.cp == nil {
		return
	}
	if h.cookie != 0 {
		syscall.SyscallN(h.cp.LpVtbl.Unadvise, uintptr(unsafe.Pointer(h.cp)), uintptr(h.cookie))
		h.cookie = 0
	}
	h.cp.Release()
	h.cp = nil
}


// ---------------------------------------------------------------- IOleInPlaceSiteEx / IOleControlSite

func ipsExQueryInterface(self unsafe.Pointer, riid win.REFIID, ppv *unsafe.Pointer) uintptr {
	s := (*inPlaceSite)(self)
	return csQueryInterface(&s.h.cs, riid, ppv)
}

func ipsAddRef2(self unsafe.Pointer) uintptr  { return 1 }
func ipsRelease2(self unsafe.Pointer) uintptr { return 1 }

func ipsGetWindow2(self unsafe.Pointer, h *win.HWND) uintptr {
	return ipsGetWindow((*inPlaceSite)(self), h)
}

func ipsContextSensitiveHelp2(self unsafe.Pointer, enter win.BOOL) uintptr { return hrNotImpl }
func ipsCanInPlaceActivate2(self unsafe.Pointer) uintptr                   { return hrOK }
func ipsOnInPlaceActivate2(self unsafe.Pointer) uintptr                    { return hrOK }
func ipsOnUIActivate2(self unsafe.Pointer) uintptr                         { return hrOK }

func ipsGetWindowContext2(self unsafe.Pointer, frame **inPlaceFrame, doc *unsafe.Pointer,
	pos, clip *win.RECT, info *win.OLEINPLACEFRAMEINFO) uintptr {
	return ipsGetWindowContext((*inPlaceSite)(self), frame, doc, pos, clip, info)
}

func ipsScroll2(self unsafe.Pointer, dx, dy int32) uintptr             { return hrNotImpl }
func ipsOnUIDeactivate2(self unsafe.Pointer, undoable win.BOOL) uintptr { return hrOK }
func ipsOnInPlaceDeactivate2(self unsafe.Pointer) uintptr              { return hrOK }
func ipsDiscardUndoState2(self unsafe.Pointer) uintptr                 { return hrNotImpl }
func ipsDeactivateAndUndo2(self unsafe.Pointer) uintptr                { return hrNotImpl }

func ipsOnPosRectChange2(self unsafe.Pointer, rc *win.RECT) uintptr {
	return ipsOnPosRectChange((*inPlaceSite)(self), rc)
}

// OnInPlaceActivateEx(this, BOOL* pfNoRedraw, DWORD dwFlags)
// 关键：把 dwFlags 置 0 表示“不走 windowless”，让控件自己建子窗口。
func ipsExOnInPlaceActivateEx(self unsafe.Pointer, pfNoRedraw *win.BOOL, dwFlags uint32) uintptr {
	if pfNoRedraw != nil {
		*pfNoRedraw = win.FALSE
	}
	logf("OnInPlaceActivateEx 请求 dwFlags=0x%X", dwFlags)
	return hrOK
}

func ipsExOnInPlaceDeactivateEx(self unsafe.Pointer, fNoRedraw win.BOOL) uintptr { return hrOK }

func ipsExRequestUIActivate(self unsafe.Pointer) uintptr { return hrOK }

func ctlQueryInterface(self unsafe.Pointer, riid win.REFIID, ppv *unsafe.Pointer) uintptr {
	s := (*controlSite)(self)
	return csQueryInterface(&s.h.cs, riid, ppv)
}

func ctlOnControlInfoChanged(self unsafe.Pointer) uintptr                { return hrOK }
func ctlLockInPlaceActive(self unsafe.Pointer, lock win.BOOL) uintptr    { return hrOK }
func ctlTransformCoords(self unsafe.Pointer, a, b unsafe.Pointer, f uint32) uintptr { return hrNotImpl }
func ctlTranslateAccelerator(self unsafe.Pointer, msg unsafe.Pointer, grf uint32) uintptr { return 1 }
func ctlOnFocus(self unsafe.Pointer, gotFocus win.BOOL) uintptr         { return hrOK }
func ctlShowPropertyFrame(self unsafe.Pointer) uintptr                  { return hrNotImpl }

func ctlGetExtendedControl(self unsafe.Pointer, ppDisp *unsafe.Pointer) uintptr {
	s := (*controlSite)(self)
	if s.h.disp == nil {
		*ppDisp = nil
		return hrNoInterface
	}
	t := (*win.IDispatchVtbl)(disp{s.h.disp}.vtbl())
	syscall.SyscallN(t.AddRef, uintptr(s.h.disp))
	*ppDisp = s.h.disp
	return hrOK
}