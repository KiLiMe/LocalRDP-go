package locales

import (
	"cmp"
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/egoist/mygo/i18n"
)

// languages are the languages the interface is written in, the source
// first: the files of this directory, and no other.
var languages = []string{"en", "zh-Hans"}

func load(t *testing.T, name string) map[string]string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	raw := map[string]any{}
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	out := map[string]string{}
	for k, v := range raw {
		if strings.HasPrefix(k, "@") {
			continue
		}
		s, ok := v.(string)
		if !ok {
			t.Fatalf("%s: %s is not a string", name, k)
		}
		out[k] = s
	}
	return out
}

func names(t *testing.T, s string) []i18n.Arg {
	t.Helper()
	m, err := i18n.ParseMessage(s)
	if err != nil {
		t.Fatal(err)
	}
	args := m.Args()
	slices.SortFunc(args, func(a, b i18n.Arg) int { return cmp.Compare(a.Name, b.Name) })
	return args
}

// 每条消息都得有中文：漏一条不会报错，只会在中文窗口里冒出英文（「窗口
// 大小」下面那句提示就是这么漏的）。代码里用一个新字符串现在编译不过，
// 所以这个测试守的是另一个口子：catalog 里少了或多了 key。
func TestKeyCoverage(t *testing.T) {
	en := load(t, "en.json")
	if len(en) < 80 {
		t.Fatalf("only %d messages in en.json, want the interface's messages", len(en))
	}
	for _, lang := range languages[1:] {
		t.Run(lang, func(t *testing.T) {
			translated := load(t, lang+".json")
			for k, v := range en {
				z, ok := translated[k]
				if !ok {
					t.Errorf("%s lacks %q", lang, k)
					continue
				}
				if strings.TrimSpace(z) == "" {
					t.Errorf("%s %q is empty", lang, k)
				}
				if strings.TrimSpace(z) == strings.TrimSpace(v) {
					t.Errorf("%s %q is still English: %q", lang, k, z)
				}
				if !slices.Equal(names(t, v), names(t, z)) {
					t.Errorf("%q arguments: en %v, %s %v", k, names(t, v), lang, names(t, z))
				}
			}
			for k := range translated {
				if _, ok := en[k]; !ok {
					t.Errorf("%s has extra key %q", lang, k)
				}
			}
		})
	}
}

// 目录里有哪些语言就是哪些语言：多一个文件没人选得到，少一个会在界面上
// 静默落回英文。
func TestSupportedLanguages(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var langs []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			langs = append(langs, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	slices.Sort(langs)
	want := slices.Clone(languages)
	slices.Sort(want)
	if !slices.Equal(langs, want) {
		t.Errorf("languages %v, want %v", langs, want)
	}
}

func TestGeneratedAccessors(t *testing.T) {
	if got := In("zh-Hans").Settings(); got != "设置" {
		t.Errorf("zh-Hans Settings = %q, want 设置", got)
	}
	if got := In("zh-CN").Settings(); got != "设置" {
		t.Errorf("zh-CN falls back to zh-Hans: Settings = %q", got)
	}
	if got := In("fr").Settings(); got != "Settings" {
		t.Errorf("fr falls back to en: Settings = %q", got)
	}
	if got := NewSession(); got != "New session" {
		t.Errorf("NewSession = %q, want New session", got)
	}
}
