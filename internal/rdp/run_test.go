package rdp

import (
	"os"
	"strings"
	"testing"
)

func TestParseU32(t *testing.T) {
	cases := map[string]uint32{
		"0":          0,
		"7":          7,
		"4294967295": 4294967295,
		" 12 ":       12,
		"":           0,
		"-1":         0,
		"1x":         0,
	}
	for in, want := range cases {
		if got := parseU32(in); got != want {
			t.Errorf("parseU32(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSplitCommandJoinsAndQuotes(t *testing.T) {
	if got := splitCommand(nil); got != "" {
		t.Errorf("splitCommand(nil) = %q, want empty", got)
	}
	got := splitCommand([]string{"notepad.exe", "a b.txt"})
	if !strings.HasPrefix(got, "notepad.exe ") {
		t.Errorf("splitCommand lost the program: %q", got)
	}
	if !strings.Contains(got, `"a b.txt"`) {
		t.Errorf("splitCommand did not quote the argument with a space: %q", got)
	}
}

func TestParseIntOption(t *testing.T) {
	args := []string{"--width", "1280"}
	i := 0
	var width int
	if !parseIntOption(&i, args, &width) {
		t.Fatal("parseIntOption rejected a valid number")
	}
	if width != 1280 {
		t.Fatalf("width = %d, want 1280", width)
	}
	if i != 1 {
		t.Fatalf("the index moved to %d, want 1", i)
	}

	bad := []string{"--width", "wide"}
	j := 0
	var ignored int
	if parseIntOption(&j, bad, &ignored) {
		t.Fatal("parseIntOption accepted a word")
	}
}

func TestTakeEnvVarDropsIt(t *testing.T) {
	const name = "LRDP_TEST_PW"
	t.Setenv(name, "s3cret")
	if got := takeEnvVar(name); got != "s3cret" {
		t.Fatalf("takeEnvVar = %q, want the value", got)
	}
	if v, ok := os.LookupEnv(name); ok {
		t.Fatalf("%s still set to %q: the processes we start would inherit the password", name, v)
	}
}
