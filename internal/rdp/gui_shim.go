package rdp

import (
	"strings"

	"localrdp/internal/app"
	"localrdp/internal/win32"
)

// runGui opens the window, which draws the state the app keeps. A second
// start does not open a second window: it brings the one that is running
// to the front and leaves.
func runGui(args []string) int {
	opts, ok := guiOptions(args)
	if !ok {
		return int(ExitError)
	}
	if opts.Capture == "" && !win32.ClaimSingleInstance(guiMutex) {
		if win32.ActivateWindow(win32.GuiClassName, guiTitle) {
			logInfo("窗口已在运行，已把它叫到前面")
		} else {
			logWarn("窗口已在运行，但没找到它的窗口")
		}
		return int(ExitOk)
	}
	return app.Run(opts)
}

// guiTitle is the title of the window, and guiMutex the name of the lock
// that keeps it one window per session.
const (
	guiTitle = "LocalRDP"
	// The Local\ prefix scopes the lock to this logon session, so two
	// accounts logged on at once each keep their own window.
	guiMutex = `Local\LocalRDP.Gui`
)

// guiOptions reads the switches of the window's command line.
func guiOptions(args []string) (app.Options, bool) {
	var opts app.Options
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
		case eqFold(arg, "-capture") || eqFold(arg, "--capture"):
			v, ok := next()
			if !ok {
				printErr("localrdp: -capture takes a file")
				return opts, false
			}
			opts.Capture = v
		case eqFold(arg, "-h") || eqFold(arg, "--help") || eqFold(arg, "-help"):
			printGuiHelp()
			return opts, false
		default:
			printErr("localrdp: unknown option for the window: " + arg)
			return opts, false
		}
	}
	return opts, true
}

// printGuiHelp is the help of the window.
func printGuiHelp() {
	println(`localrdp gui [options]

Options:
  -capture FILE   draw the window once into FILE, then close it
  -h, --help      this text`)
}
