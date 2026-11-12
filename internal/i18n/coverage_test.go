package i18n

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// 每个界面上要翻的字符串都在 zh 里：漏一条不会报错，只会在中文窗口里冒出
// 一段英文（「窗口大小」下面那句提示就是这么漏的），所以由这个测试守著。
func TestEveryTranslatedStringIsInTheMap(t *testing.T) {
	call := regexp.MustCompile(`\bT\("((?:[^"\\]|\\.)*)"\)`)
	var missing []string
	err := filepath.WalkDir("..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, m := range call.FindAllStringSubmatch(string(b), -1) {
			s, err := strconv.Unquote(`"` + m[1] + `"`)
			if err != nil {
				continue
			}
			if _, ok := zh[s]; !ok {
				missing = append(missing, filepath.ToSlash(p)+`: `+s)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) > 0 {
		t.Fatalf("%d string(s) have no Chinese:\n%s", len(missing), strings.Join(missing, "\n"))
	}
}
