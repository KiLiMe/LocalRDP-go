package app

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"localrdp/internal/store"
	"localrdp/internal/win32"
)

// runSelf runs this program with args and returns what it said.
func runSelf(args ...string) (string, int) {
	cmd := exec.Command(win32.ExePath(), args...)
	cmd.Dir = win32.ExeDir()
	win32.HideWindow(cmd)
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
// password in its environment rather than on the command line, so other users
// cannot see it in the process list. Processes of the same user can still read
// it; the child drops the variable as soon as it has read it (--password-env).
func childCommand(s store.Session, password string) *exec.Cmd {
	cmd := exec.Command(win32.ExePath(), s.Args()...)
	cmd.Dir = win32.ExeDir()
	win32.NoConsole(cmd)
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
