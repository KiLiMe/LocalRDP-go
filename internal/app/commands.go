// Command watching: the window can be driven from outside. `localrdp click
// <name>` leaves a line in the command file; this reads it on the UI thread
// and runs the same code the button runs.
package app

import (
	"os"
	"strings"
	"time"

	"localrdp/internal/store"
)

// watchCommands polls the command file and presses the buttons it names.
// One goroutine per window; it ends when the window closes.
func (a *App) watchCommands() {
	go func() {
		for {
			time.Sleep(120 * time.Millisecond)
			if a.Win == nil {
				continue
			}
			names := takeCommands()
			for _, name := range names {
				a.press(name)
			}
		}
	}()
}

// takeCommands reads and removes the command file.
func takeCommands() []string {
	path := store.CommandFile()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	_ = os.Remove(path)
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if name := strings.TrimSpace(line); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// press runs one named button on the UI thread.
func (a *App) press(name string) {
	a.Do(func() {
		act := (*actions)(a)
		id := a.activeID()
		a.appendLog([]string{"click " + name})
		switch name {
		case "open":
			act.OpenSession()
		case "status":
			act.RefreshStatus()
		case "stop":
			act.LogOff()
		case "privilege":
			act.GrantPrivileges()
		case "forget-password":
			act.ForgetPassword()
		case "data-folder":
			act.OpenDataFolder()
		case "reconnect":
			act.ReconnectSession(id)
		case "show":
			act.ShowSession(id)
		case "close-session":
			act.CloseSession(id)
		case "refresh":
			act.RefreshStatus()
		case "clear-log":
			act.ClearLog()
		case "copy-log":
			act.CopyLog()
		default:
			a.appendLog([]string{"unknown click " + name + "，可用：" + strings.Join(store.GuiCommands, " ")})
		}
	})
}

// activeID is the session of the page in front, or "" when the form is.
func (a *App) activeID() string {
	if a.St.Active < 0 || a.St.Active >= len(a.St.Sessions) {
		return ""
	}
	return a.St.Sessions[a.St.Active].Session.ID()
}
