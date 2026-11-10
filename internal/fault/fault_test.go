package fault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// all is every code the program hands out. A code that is not listed here is
// not checked, so this list is the inventory.
var all = []Code{
	Edition, RdpControl, InsideChild, ChildSessionsOff, Transport, TermService,
	NoChildSupport, TLSHandshake, ConnectTimeout, LogonRejected, TakenOver,
	Disconnected, ControlBroken, LaunchFailed, ClipBridgeFailed,
}

func TestEveryCodeHasOneLine(t *testing.T) {
	seen := map[Code]bool{}
	for _, c := range all {
		if seen[c] {
			t.Errorf("code %s is handed out twice", c)
		}
		seen[c] = true
		if !c.Known() {
			t.Errorf("code %s has no line to show", c)
		}
	}
	if len(about) != len(all) {
		t.Errorf("%d codes in the table, %d in the list", len(about), len(all))
	}
}

// The codes only help if the document spells them out: this is what keeps the
// program and docs/故障代码.md from drifting apart.
func TestCodesAreDocumented(t *testing.T) {
	path := filepath.Join("..", "..", "docs", "故障代码.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	text := string(data)
	for _, c := range all {
		if !strings.Contains(text, string(c)) {
			t.Errorf("%s is missing from docs/故障代码.md", c)
		}
	}
}

func TestLineIsTheCodeAndWhereToRead(t *testing.T) {
	line := Line(TLSHandshake, "0xD07")
	for _, want := range []string{"E202", "0xD07", doc} {
		if !strings.Contains(line, want) {
			t.Errorf("line %q does not carry %q", line, want)
		}
	}
	if bare := Line(ControlBroken, ""); strings.Contains(bare, "（）") {
		t.Errorf("empty detail left empty brackets: %q", bare)
	}
}

func TestFindPicksTheFaultOut(t *testing.T) {
	want := Line(LogonRejected, "0x807")
	lines := []string{"", "事件 OnConnecting", want, "仍然等待中 Connected=0"}
	if got, ok := Find(lines); !ok || got != want {
		t.Errorf("Find = %q, %v; want %q", got, ok, want)
	}
	if _, ok := Find([]string{"OnDisconnected Disconnected locally.", "E999 not a code"}); ok {
		t.Error("Find took a line that is not a fault")
	}
}
