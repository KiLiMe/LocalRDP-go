package rdp

import (
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

const (
	vtI2   = 2
	vtI4   = 3
	vtBstr = 8
	vtDisp = 9
	vtBool = 11
	vtUi4  = 19
	vtI8   = 20
	vtUi8  = 21

	dispatchMethod    = 1
	dispatchPropGet   = 2
	dispatchPropPut   = 4
	dispidPropertyPut = -3
	localeUserDefault = 0x0400

	hrOK          = uintptr(0)
	hrNotImpl     = uintptr(0x80004001)
	hrNoInterface = uintptr(0x80004002)
	hrPointer     = uintptr(0x80004003)
)

// variant 与 amd64 VARIANT 同布局：8 字节头 + 8 字节联合 + 8 字节保留 = 24。
type variant struct {
	VT  uint16
	r1  uint16
	r2  uint16
	r3  uint16
	val uint64
	_   [8]byte
}

var procVariantClear = syscall.NewLazyDLL("oleaut32.dll").NewProc("VariantClear")

// boolI4 is a flag the control takes as a LONG.
func boolI4(b bool) int32 {
	if b {
		return 1
	}
	return 0
}

func varI4(v int32) variant { return variant{VT: vtI4, val: uint64(uint32(v))} }

func varBool(b bool) variant {
	x := variant{VT: vtBool}
	if b {
		x.val = 0xffff
	}
	return x
}

func varBstr(s string) variant {
	return variant{VT: vtBstr, val: uint64(uintptr(unsafe.Pointer(win.SysAllocString(s))))}
}

func variantClear(v *variant) {
	procVariantClear.Call(uintptr(unsafe.Pointer(v)))
}

// disp 是裸 IDispatch 指针的薄封装，对应 src/dispatch.cpp。
type disp struct{ p unsafe.Pointer }

func (d disp) vtbl() *win.IDispatchVtbl { return (*win.IDispatch)(d.p).LpVtbl }

func (d disp) idsOfNames(name string) (int32, uintptr) {
	nm := win.SysAllocString(name)
	defer win.SysFreeString(nm)
	var id int32
	names := nm
	hr, _, _ := syscall.SyscallN(d.vtbl().GetIDsOfNames,
		uintptr(d.p), 0,
		uintptr(unsafe.Pointer(&names)), 1,
		uintptr(localeUserDefault),
		uintptr(unsafe.Pointer(&id)))
	return id, hr
}

func (d disp) invoke(name string, flags uint16, params *win.DISPPARAMS, result *variant) uintptr {
	id, hr := d.idsOfNames(name)
	if hr != hrOK {
		return hr
	}
	var argErr uint32
	hr, _, _ = syscall.SyscallN(d.vtbl().Invoke,
		uintptr(d.p),
		uintptr(int64(id)),
		0,
		uintptr(localeUserDefault),
		uintptr(flags),
		uintptr(unsafe.Pointer(params)),
		uintptr(unsafe.Pointer(result)),
		0,
		uintptr(unsafe.Pointer(&argErr)))
	return hr
}

// put 对应 DispPut：单个具名参数 + DISPATCH_PROPERTYPUT。
func (d disp) put(name string, v variant) uintptr {
	named := int32(dispidPropertyPut)
	p := win.DISPPARAMS{
		Rgvarg:            (*win.VARIANTARG)(unsafe.Pointer(&v)),
		RgdispidNamedArgs: (*win.DISPID)(unsafe.Pointer(&named)),
		CArgs:             1,
		CNamedArgs:        1,
	}
	hr := d.invoke(name, dispatchPropPut, &p, nil)
	variantClear(&v)
	return hr
}

func (d disp) get(name string) (variant, uintptr) {
	var out variant
	p := win.DISPPARAMS{}
	hr := d.invoke(name, dispatchPropGet, &p, &out)
	return out, hr
}

func u2p(u uint64) unsafe.Pointer {
	var p unsafe.Pointer
	*(*uint64)(unsafe.Pointer(&p)) = u
	return p
}

func (d disp) getDispatch(name string) (unsafe.Pointer, uintptr) {
	v, hr := d.get(name)
	if hr != hrOK {
		return nil, hr
	}
	if v.VT == vtDisp && v.val != 0 {
		return u2p(v.val), hrOK
	}
	variantClear(&v)
	return nil, hrNoInterface
}

func (d disp) call(name string) uintptr {
	p := win.DISPPARAMS{}
	return d.invoke(name, dispatchMethod, &p, nil)
}

func variantString(v *variant) string {
	switch v.VT {
	case vtBool:
		if v.val != 0 {
			return "true"
		}
		return "false"
	case vtI4:
		return itoa64(int64(int32(uint32(v.val))))
	case vtI2:
		return itoa64(int64(int16(uint32(v.val))))
	case vtBstr:
		if v.val == 0 {
			return "\"\""
		}
		return "\"" + win.BSTRToString((*uint16)(u2p(v.val))) + "\""
	case vtUi4:
		return itoa64(int64(uint32(v.val)))
	case vtI8:
		return itoa64(int64(v.val))
	case vtUi8:
		return itoa64(int64(v.val))
	default:
		return "vt=" + itoa64(int64(v.VT))
	}
}

func itoa64(v int64) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	var buf [24]byte
	i := len(buf)
	u := uint64(v)
	if neg {
		u = uint64(-v)
	}
	for u > 0 {
		i--
		buf[i] = byte('0' + u%10)
		u /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// callN 对应 DispCallN：参数在 DISPPARAMS 里逆序存放。
func (d disp) callN(name string, args []variant, result *variant) uintptr {
	n := len(args)
	arr := make([]variant, n)
	for i := 0; i < n; i++ {
		arr[i] = args[n-1-i]
	}
	var p win.DISPPARAMS
	if n > 0 {
		p.Rgvarg = (*win.VARIANTARG)(unsafe.Pointer(&arr[0]))
		p.CArgs = int32(n)
	}
	hr := d.invoke(name, dispatchMethod, &p, result)
	return hr
}
