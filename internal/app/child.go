package app

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"localrdp/internal/store"

	"golang.org/x/sys/windows"
)

// hidden is what a child process runs with, so no console flashes up.
const hidden = 0x08000000

// hideWindow keeps a child process from showing a console.
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: hidden}
}

// runSelf runs this program with args and returns what it said.
func runSelf(args ...string) (string, int) {
	cmd := exec.Command(exePath(), args...)
	cmd.Dir = exeDir()
	hideWindow(cmd)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			code = exit.ExitCode()
		} else {
			code = 1
			buf.WriteString(err.Error() + "\n")
		}
	}
	return buf.String(), code
}

// childCommand is the child session of the form as a command, with the
// password in the environment where no other process can read it.
func childCommand(s store.Session, password string) *exec.Cmd {
	cmd := exec.Command(exePath(), s.Args()...)
	cmd.Dir = exeDir()
	hideWindow(cmd)
	cmd.Env = os.Environ()
	if password != "" {
		cmd.Env = append(cmd.Env, store.PasswordEnv+"="+password)
	}
	return cmd
}

// nonEmptyLines is the lines of s that are not blank.
func nonEmptyLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimRight(line, "\r"); strings.TrimSpace(line) != "" {
			out = append(out, line)
		}
	}
	return out
}

// exePath is this program.
func exePath() string {
	if path, err := os.Executable(); err == nil {
		return path
	}
	return os.Args[0]
}

// exeDir is the folder of this program, where its scripts are.
func exeDir() string {
	path := exePath()
	if i := strings.LastIndexAny(path, `\/`); i > 0 {
		return path[:i]
	}
	return "."
}

var _ = windows.UTF16PtrFromString

// utf16Ptr is s as the wide string the Windows API takes.
func utf16Ptr(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}
