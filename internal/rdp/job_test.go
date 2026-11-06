package rdp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestJobFileRoundTrip(t *testing.T) {
	want := StartOptions{
		WorkDir:  `C:\work dir`,
		Command:  `notepad.exe "a b.txt"`,
		KeepSess: true,
		Desktop:  true,
	}
	path := writeJobFile(want)
	if path == "" {
		t.Fatal("writeJobFile returned no path")
	}
	defer deleteJobFile(path)

	var got StartOptions
	if !readJobFile(path, &got) {
		t.Fatalf("readJobFile rejected %s", path)
	}
	if got.WorkDir != want.WorkDir || got.Command != want.Command || !got.KeepSess || !got.Desktop {
		t.Fatalf("round trip changed the job: %+v", got)
	}
}

func TestJobFileRejectsForeignFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "foreign.job")
	if err := os.WriteFile(path, []byte("NOT-A-JOB\ncommand=calc.exe\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var got StartOptions
	if readJobFile(path, &got) {
		t.Fatal("a file without the header was accepted")
	}
	if got.Command != "" {
		t.Fatalf("a foreign file set the command: %q", got.Command)
	}
}
