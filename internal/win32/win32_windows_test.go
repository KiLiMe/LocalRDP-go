package win32

import (
	"syscall"
	"testing"
	"unicode/utf16"
	"unsafe"
)

// getCurrentAppUserModelID 是 SetCurrentProcessExplicitAppUserModelID 的配套
// 读接口：shell32 里有，但没写进文档，所以取不到就跳过测试。
func getCurrentAppUserModelID(t *testing.T) func() (string, bool) {
	t.Helper()
	get := shell32.NewProc("GetCurrentProcessExplicitAppUserModelID")
	if err := get.Find(); err != nil {
		t.Skip("GetCurrentProcessExplicitAppUserModelID 不可用: " + err.Error())
	}
	free := syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree")
	return func() (string, bool) {
		var p *uint16
		hr, _, _ := get.Call(uintptr(unsafe.Pointer(&p)))
		if hr != 0 {
			// 没有显式分组时它就报 E_FAIL，这正是“没设置”。
			if uint32(hr) == 0x80004005 {
				return "", false
			}
			t.Fatalf("GetCurrentProcessExplicitAppUserModelID: HRESULT 0x%08x", uint32(hr))
		}
		defer free.Call(uintptr(unsafe.Pointer(p)))
		var units []uint16
		for i := 0; i < 128; i++ {
			c := *(*uint16)(unsafe.Add(unsafe.Pointer(p), uintptr(i)*2))
			if c == 0 {
				break
			}
			units = append(units, c)
		}
		return string(utf16.Decode(units)), true
	}
}

// TestSetAppUserModelID 检查设置之后能读回同一个分组：任务栏按它分按钮，
// 报错了没人会发现，只会看到窗口又粘到一起。
func TestSetAppUserModelID(t *testing.T) {
	read := getCurrentAppUserModelID(t)

	id := "localrdp.test.group"
	if err := SetAppUserModelID(id); err != nil {
		t.Fatal(err)
	}
	got, ok := read()
	if !ok || got != id {
		t.Fatalf("读回的分组是 %q（ok=%v），想要 %q", got, ok, id)
	}
}
