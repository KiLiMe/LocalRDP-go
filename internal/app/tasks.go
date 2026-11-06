package app

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"localrdp/internal/i18n"
	"localrdp/internal/store"
)

// logMu serializes the writes to the log file, which the child processes
// append to too.
var logMu sync.Mutex

// runLogging runs this program with args and returns its exit code.
func runLogging(args ...string) int {
	cmd := exec.Command(exePath(), args...)
	cmd.Dir = exeDir()
	hideWindow(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return 1
	}
	_, _ = io.Copy(io.Discard, stdout)
	err = cmd.Wait()
	if exit, ok := err.(*exec.ExitError); ok {
		return exit.ExitCode()
	}
	if err != nil {
		return 1
	}
	return 0
}

// appendToLog adds lines to the log file, which every process appends to.
func appendToLog(lines []string) {
	if len(lines) == 0 {
		return
	}
	logMu.Lock()
	defer logMu.Unlock()
	_ = os.MkdirAll(store.Dir(), 0o755)
	f, err := os.OpenFile(store.LogFile(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	for _, line := range lines {
		_, _ = f.WriteString(line + "\r\n")
	}
}

// i18nT is the text of the interface in the chosen language.
func i18nT(s string) string { return i18n.T(s) }

// escapeVB makes s safe inside a VBScript string literal.
func escapeVB(s string) string { return strings.ReplaceAll(s, `"`, `""`) }
