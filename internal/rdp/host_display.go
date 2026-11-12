package rdp

import (
	"localrdp/internal/store"
	"localrdp/internal/win32"
)

// applyDisplay says the size the session desktop should have now. Fit is the
// only mode that changes it: fixed and scale keep the resolution the session
// connected with, and the control fits that picture into the window (scale;
// SmartSizing shrinks but never magnifies), or the window never moves away
// from it (fixed).
func applyDisplay(s *hostState) bool {
	if s.host == nil || s.host.disp == nil || !s.loggedIn {
		return false
	}
	w, h := s.viewSize()
	ds := s.layoutScale()
	if s.sizeMode != store.SizeFit {
		if s.baseW <= 0 || s.baseH <= 0 {
			// 还没连上，基准尺寸未知，先不动。
			return true
		}
		// 固定与缩放都不换分辨率：会话一直是连接时那个桌面，
		// 只有窗口在动（缩放档由 SmartSizing 把画面缩到放得下，放大不了）。
		w, h, ds = s.baseW, s.baseH, s.baseScale
	}
	dev := deviceScaleFor(ds)
	if int(w) == s.appliedW && int(h) == s.appliedH && int(ds) == s.appliedScale {
		return true
	}
	// The physical size says how big the desktop is in inches, which is what
	// tells the guest its dpi: the one the scale factor stands for, so the
	// guest cannot read one scaling from this and another from the factor.
	dpi := layoutDPI(ds)
	physW := clampI32(w*254/(10*dpi), 10, 10000)
	physH := clampI32(h*254/(10*dpi), 10, 10000)

	d := disp{s.host.disp}
	callUpdate := func(pw, ph int32) uintptr {
		args := []variant{varI4(w), varI4(h), varI4(pw), varI4(ph), varI4(0), varI4(ds), varI4(dev)}
		return d.callN("UpdateSessionDisplaySettings", args, nil)
	}
	hr := callUpdate(physW, physH)
	if hrFailed(hr) {
		hr = callUpdate(0, 0)
	}
	if hrFailed(hr) {
		logWarn("UpdateSessionDisplaySettings 失败: " + formatHr(uint32(hr)))
		var ret variant
		hr = d.callN("Reconnect", []variant{varI4(w), varI4(h)}, &ret)
		status := int32(-1)
		if ret.VT == vtI4 {
			status = int32(uint32(ret.val))
		}
		variantClear(&ret)
		if hrFailed(hr) || status != 0 {
			logWarn("Reconnect 尺寸回退失败 hr=" + formatHr(uint32(hr)) + " status=" + win32.Itoa(int(status)))
			return false
		}
		logf("Display updated via Reconnect %dx%d", w, h)
	} else {
		logf("Display updated %dx%d @%d%% dpi=%d", w, h, ds, dpi)
	}
	s.appliedW, s.appliedH, s.appliedScale = int(w), int(h), int(ds)
	return true
}

func clampI32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// layoutScale is the desktop scale this session lays out at; desktopScale
// says what each size mode asks for.
func (s *hostState) layoutScale() int32 {
	return desktopScale(s.opt, s.sizeMode, s.hostDPI)
}
