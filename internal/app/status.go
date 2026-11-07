package app

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"localrdp/internal/win32"
)

// parseSessionID reads the child session of a status report.
func parseSessionID(out string) string {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "ChildSessionId") {
			continue
		}
		_, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		value = strings.TrimSpace(value)
		if n, err := strconv.Atoi(value); err == nil && n > 0 {
			return value
		}
	}
	return ""
}

// copyText puts s on the clipboard.
func copyText(s string) error {
	cmd := exec.Command("clip.exe")
	win32.HideWindow(cmd)
	cmd.Stdin = strings.NewReader(s)
	return cmd.Run()
}

// stderrf writes a line to the standard error.
func stderrf(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

var _ = syscall.SysProcAttr{}
