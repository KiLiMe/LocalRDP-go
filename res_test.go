package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

// The icon reaches the exe in two hand-run steps: tools/genicon draws
// winres/localrdp.ico, then go-winres bakes it into rsrc_windows_amd64.syso,
// which go build links on its own. Forget the second step and the build keeps
// shipping the old icon without a word, so hold the two files to each other.
func TestIconReachesTheExe(t *testing.T) {
	ico, err := os.ReadFile("winres/localrdp.ico")
	if err != nil {
		t.Fatalf("read winres/localrdp.ico: %v", err)
	}
	syso, err := os.ReadFile("rsrc_windows_amd64.syso")
	if err != nil {
		t.Fatalf("read rsrc_windows_amd64.syso: %v", err)
	}

	images := icoImages(t, ico)
	if len(images) == 0 {
		t.Fatal("winres/localrdp.ico holds no images")
	}
	for i, img := range images {
		if !bytes.Contains(syso, img) {
			t.Errorf("image %d of winres/localrdp.ico is missing from rsrc_windows_amd64.syso: run pwsh ./tools/resync.ps1 after changing the icon", i)
		}
	}

	// And the group icon, which LoadImage(MAKEINTRESOURCE(1)) walks, has to
	// list every one of them.
	if !bytes.Contains(syso, groupIconHeader(len(images))) {
		t.Errorf("rsrc_windows_amd64.syso has no icon group of %d images", len(images))
	}
}

// icoImages returns the payload of every image in a .ico, in directory order.
func icoImages(t *testing.T, ico []byte) [][]byte {
	t.Helper()
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:]) != 1 {
		t.Fatal("winres/localrdp.ico is not an icon file")
	}
	n := int(binary.LittleEndian.Uint16(ico[4:]))
	images := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		entry := 6 + 16*i
		if entry+16 > len(ico) {
			t.Fatalf("winres/localrdp.ico: entry %d of %d is cut short", i, n)
		}
		size := binary.LittleEndian.Uint32(ico[entry+8:])
		offset := binary.LittleEndian.Uint32(ico[entry+12:])
		if int(offset)+int(size) > len(ico) {
			t.Fatalf("winres/localrdp.ico: entry %d points past the end of the file", i)
		}
		images = append(images, ico[offset:offset+size])
	}
	return images
}

// groupIconHeader is the first 6 bytes of a GRPICONDIR: reserved, type 1
// (icon), then how many images follow.
func groupIconHeader(n int) []byte {
	return []byte{0, 0, 1, 0, byte(n), byte(n >> 8)}
}
