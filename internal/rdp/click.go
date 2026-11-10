package rdp

import (
	"strconv"
	"strings"

	"localrdp/internal/store"
	"localrdp/internal/win32"
)

// cmdClick presses a button without a mouse. The session window's own bar
// (fullscreen, reconnect, disconnect) is reached by posting the command the
// bar posts; the window's buttons are reached through the command file the
// window watches (see store.GuiCommands).
func cmdClick(args []string) int {
	pid := 0
	name := ""
	for i := 0; i < len(args); i++ {
		arg := args[i]
		value := ""
		if j := strings.IndexByte(arg, '='); j >= 0 {
			arg, value = arg[:j], arg[j+1:]
		}
		next := func() (string, bool) {
			if value != "" {
				return value, true
			}
			if i+1 < len(args) {
				i++
				return args[i], true
			}
			return "", false
		}
		switch {
		case eqFold(arg, "--list"):
			printOut("session window: " + strings.Join(hostClickNames, " "))
			printOut("window: " + strings.Join(store.GuiCommands, " "))
			return int(ExitOk)
		case eqFold(arg, "--pid"):
			v, ok := next()
			if !ok {
				printErr("--pid needs a process id.")
				return int(ExitError)
			}
			if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				pid = n
			} else {
				printErr("--pid takes a number, not " + v + ".")
				return int(ExitError)
			}
		case strings.HasPrefix(arg, "-"):
			printErr("Unknown option: " + arg)
			return int(ExitError)
		default:
			if name != "" {
				printErr("One button at a time: " + name + " " + arg)
				return int(ExitError)
			}
			name = strings.ToLower(strings.TrimSpace(arg))
		}
	}
	if name == "" {
		printErr("Which button? Try `localrdp click --list`.")
		return int(ExitError)
	}
	_, isHost := hostClicks[name]
	if !isHost && !store.KnownGuiCommand(name) {
		printErr("Unknown button: " + name + ". Try `localrdp click --list`.")
		return int(ExitError)
	}
	if cmd, ok := hostClicks[name]; ok {
		if pid == 0 {
			pid = win32.HostProcess()
		}
		if pid == 0 {
			printErr("No session window is open.")
			return int(ExitError)
		}
		if !win32.PostCommand(pid, cmd) {
			printErr("Could not reach the session window of pid " + win32.Itoa(pid) + ".")
			return int(ExitError)
		}
		printOut("Clicked " + name + " in the session window (pid " + win32.Itoa(pid) + ").")
		return int(ExitOk)
	}
	if !win32.GuiRunning() {
		printErr("The window is not open: start `localrdp gui` first.")
		return int(ExitError)
	}
	if err := store.WriteGuiCommand(name); err != nil {
		printErr("Can't click " + name + ": " + err.Error() + " (is the window open?)")
		return int(ExitError)
	}
	printOut("Clicked " + name + ".")
	return int(ExitOk)
}

// hostClicks are the buttons of a session window, by the name `localrdp
// click` takes. The order is the order they are listed in.
var hostClicks = map[string]uintptr{
	"fullscreen": win32.CmdFullScreen,
	"reconnect":  win32.CmdReconnect,
	"disconnect": win32.CmdDisconnect,
}

// hostClickNames lists them for --list, in a stable order.
var hostClickNames = []string{"fullscreen", "reconnect", "disconnect"}
