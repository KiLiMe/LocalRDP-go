package rdp

import (
	"strings"

	"localrdp/internal/app"
)

// runGui opens the window, which draws the state the app keeps.
func runGui(args []string) int {
	opts, ok := guiOptions(args)
	if !ok {
		return int(ExitError)
	}
	return app.Run(opts)
}

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
