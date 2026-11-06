package main

import (
	"os"
	"strconv"
	"strings"
)

func eqFold(a, b string) bool { return strings.EqualFold(a, b) }

func printHelp() {
	printOut("LocalRDP " + versionString)
	printOut("Local loopback Remote Desktop child session for the current user.")
	printOut("")
	printOut("Usage:")
	printOut("  localrdp status")
	printOut("  localrdp start [options] -- <program> [args...]")
	printOut("  localrdp desktop [options] [-- <program> [args...]]")
	printOut("  localrdp stop")
	printOut("  localrdp gui [options]")
	printOut("  localrdp help")
	printOut("")
	printOut("Options (start / desktop):")
	printOut("  --width N          Initial window client width (even; default ~5/6 of the monitor)")
	printOut("  --height N         Initial window client height")
	printOut("  --x N --y N        Host window position")
	printOut("  --scale N          Desktop scale percent 100-500 (default: follow window DPI)")
	printOut("  --fullscreen       Start full screen (Ctrl+Alt+F to toggle)")
	printOut("  --title TEXT       Host window title")
	printOut("  --workdir DIR      Working directory for the launched program")
	printOut("  --user NAME        Optional explicit user name")
	printOut("  --password PASS    Optional password (prefer --password-env)")
	printOut("  --password-env VAR Read password from the environment variable VAR")
	printOut("  --timeout SEC      Connect timeout (default 90)")
	printOut("  --minimized        Start the host window minimized")
	printOut("  --hidden           Hide the host window (session still runs)")
	printOut("  --no-clipboard     Do not redirect the clipboard (text or files)")
	printOut("  --drives           Request RDP drive mapping (\\\\tsclient); often unused on loopback")
	printOut("  --view-only        Ignore local keyboard and mouse in the host window")
	printOut("  --keep-session     Do not log off the child session when the host exits")
	printOut("")
	printOut("Options (gui):")
	printOut("  -capture FILE      Write a PNG of the window there and quit")

	printOut("")
	printOut("The child session is the same Windows account as this process. It is not a")
	printOut("security boundary: files, tokens and installed apps are shared.")
	printOut("Windows allows only one child session at a time.")
}

func needValue(i int, args []string, flag string) bool {
	if i+1 >= len(args) {
		printErr("Missing value for " + flag)
		return false
	}
	return true
}

func parseIntOption(i *int, args []string, dst *int) bool {
	flag := args[*i]
	if !needValue(*i, args, flag) {
		return false
	}
	v, err := strconv.ParseInt(strings.TrimSpace(args[*i+1]), 10, 32)
	if err != nil {
		printErr("Invalid integer for " + flag + ": " + args[*i+1])
		return false
	}
	*dst = int(v)
	*i++
	return true
}

func main() {
	logInit()

	args := os.Args
	// 不带参数启动 exe 时直接进图形界面（双击即可用）。
	if len(args) < 2 {
		os.Exit(runGui(nil))
	}
	if eqFold(args[1], "-gui") || eqFold(args[1], "--gui") {
		os.Exit(runGui(args[2:]))
	}
	if len(args) >= 2 && eqFold(args[1], "--session-entry") {
		job := ""
		expect := uint32(kNoSession)
		for i := 2; i < len(args); i++ {
			switch {
			case eqFold(args[i], "--job") && i+1 < len(args):
				i++
				job = args[i]
			case eqFold(args[i], "--expect-session") && i+1 < len(args):
				i++
				expect = parseU32(args[i])
			}
		}
		if job == "" {
			logError("--session-entry requires --job")
			return
		}
		os.Exit(runSessionEntry(expect, job))
	}

	if len(args) >= 2 && eqFold(args[1], "--clip-bridge") {
		expect := uint32(kNoSession)
		var parent uint32
		for i := 2; i < len(args); i++ {
			switch {
			case eqFold(args[i], "--expect-session") && i+1 < len(args):
				i++
				expect = parseU32(args[i])
			case eqFold(args[i], "--parent-pid") && i+1 < len(args):
				i++
				parent = parseU32(args[i])
			}
		}
		if cur := currentSessionID(); expect != kNoSession && cur != expect {
			logError("--clip-bridge 在错误的会话中运行")
			os.Exit(int(ExitError))
		}
		clipBridgeMain(clipRoleChild, parent)
		return
	}

	if eqFold(args[1], "-h") || eqFold(args[1], "--help") || eqFold(args[1], "/?") {
		printHelp()
		return
	}
	if eqFold(args[1], "--version") || eqFold(args[1], "-V") {
		printOut("LocalRDP " + versionString)
		return
	}

	verb := args[1]
	if eqFold(verb, "gui") || eqFold(verb, "ui") {
		os.Exit(runGui(args[2:]))
	}
	opt := defaultOptions()
	var programArgs []string
	afterDash := false
	optionsAllowed := eqFold(verb, "start") || eqFold(verb, "desktop")
	if !optionsAllowed && len(args) > 2 {
		printErr("`" + verb + "` does not accept options or arguments.")
		os.Exit(int(ExitError))
	}

	for i := 2; i < len(args); i++ {
		a := args[i]
		if afterDash {
			programArgs = append(programArgs, a)
			continue
		}
		if eqFold(a, "--") {
			afterDash = true
			continue
		}
		if eqFold(a, "-h") || eqFold(a, "--help") || eqFold(a, "/?") {
			printHelp()
			return
		}
		if eqFold(a, "--version") || eqFold(a, "-V") {
			printOut("LocalRDP " + versionString)
			return
		}
		switch strings.ToLower(a) {
		case "--width":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.Width) {
				os.Exit(int(ExitError))
			}
			continue
		case "--height":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.Height) {
				os.Exit(int(ExitError))
			}
			continue
		case "--x":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.X) {
				os.Exit(int(ExitError))
			}
			continue
		case "--y":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.Y) {
				os.Exit(int(ExitError))
			}
			continue
		case "--scale":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.Scale) {
				os.Exit(int(ExitError))
			}
			continue
		case "--timeout":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.TimeoutSec) {
				os.Exit(int(ExitError))
			}
			if opt.TimeoutSec < 5 {
				opt.TimeoutSec = 5
			}
			continue
		case "--title":
			if !optionsAllowed || !needValue(i, args, a) {
				os.Exit(int(ExitError))
			}
			i++
			opt.Title = args[i]
			continue
		case "--workdir":
			if !optionsAllowed || !needValue(i, args, a) {
				os.Exit(int(ExitError))
			}
			i++
			opt.WorkDir = args[i]
			continue
		case "--user":
			if !optionsAllowed || !needValue(i, args, a) {
				os.Exit(int(ExitError))
			}
			i++
			opt.User = args[i]
			continue
		case "--password":
			if !optionsAllowed || !needValue(i, args, a) {
				os.Exit(int(ExitError))
			}
			i++
			opt.Password = args[i]
			continue
		case "--password-env":
			if !optionsAllowed || !needValue(i, args, a) {
				os.Exit(int(ExitError))
			}
			i++
			opt.Password = getEnvVar(args[i])
			continue
		case "--minimized":
			if !optionsAllowed {
				os.Exit(int(ExitError))
			}
			opt.Minimized = true
			continue
		case "--hidden":
			if !optionsAllowed {
				os.Exit(int(ExitError))
			}
			opt.Hidden = true
			continue
		case "--no-clipboard":
			if !optionsAllowed {
				os.Exit(int(ExitError))
			}
			opt.Clipboard = false
			continue
		case "--drives":
			if !optionsAllowed {
				os.Exit(int(ExitError))
			}
			opt.Drives = true
			continue
		case "--view-only":
			if !optionsAllowed {
				os.Exit(int(ExitError))
			}
			opt.ViewOnly = true
			continue
		case "--keep-session":
			if !optionsAllowed {
				os.Exit(int(ExitError))
			}
			opt.KeepSess = true
			continue
		case "--fullscreen", "--full-screen":
			if !optionsAllowed {
				os.Exit(int(ExitError))
			}
			opt.Fullscreen = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			printErr("Unknown option: " + a)
			printErr("Try `localrdp help`.")
			os.Exit(int(ExitError))
		}
		programArgs = append(programArgs, a)
	}

	logInfo("verb=" + verb)

	switch {
	case eqFold(verb, "help"):
		printHelp()
		return
	case eqFold(verb, "version"):
		printOut("LocalRDP " + versionString)
		return
	case eqFold(verb, "status"):
		r := queryEnvironment()
		printStatus(r)
		if r.ready() {
			os.Exit(int(ExitOk))
		}
		os.Exit(int(ExitNotReady))
	case eqFold(verb, "setup"):
		printErr("`setup` has been removed. Child sessions are enabled automatically by start / desktop.")
		printErr("LocalRDP does not change the firewall, RDP listener, TermService, or credential policy.")
		os.Exit(int(ExitError))
	case eqFold(verb, "stop"):
		os.Exit(cmdStop())
	case eqFold(verb, "gui"), eqFold(verb, "ui"):
		os.Exit(runGui(nil))
	case eqFold(verb, "desktop"):
		opt.Desktop = true
		opt.Command = splitCommand(programArgs)
		os.Exit(cmdStart(opt))
	case eqFold(verb, "start"):
		if len(programArgs) == 0 {
			printErr("start requires a program. Example: localrdp start -- notepad.exe")
			printErr("For a full desktop, use: localrdp desktop")
			os.Exit(int(ExitError))
		}
		opt.Desktop = false
		opt.Command = splitCommand(programArgs)
		os.Exit(cmdStart(opt))
	}

	printErr("Unknown command: " + verb)
	printErr("Try `localrdp help`.")
	os.Exit(int(ExitError))
}

func parseU32(s string) uint32 {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(v)
}
