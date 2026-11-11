package rdp

import (
	"strconv"
	"strings"

	"localrdp/internal/store"
	"localrdp/internal/win32"
)

// eqFold compares two strings ignoring ASCII case, as the verbs of the
// command line are matched.
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
	printOut("  localrdp mcp")
	printOut("  localrdp click <button> [--pid N]   Press a button without a mouse")
	printOut("  localrdp help")
	printOut("")
	printOut("Options (start / desktop):")
	printOut("  --width N          Initial window client width (even; default ~5/6 of the monitor)")
	printOut("  --height N         Initial window client height")
	printOut("  --x N --y N        Host window position")
	printOut("  --scale N          Desktop scale percent 100-500 (default: 100, the window's own pixels)")
	printOut("  --size MODE        fit (default, window decides the resolution), fixed (window and")
	printOut("                     session keep --width/--height, no resize border), or scale (the")
	printOut("                     resolution stays; a smaller window shrinks the picture to fit, a")
	printOut("                     bigger one leaves it at its size, it is never stretched up)")
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
	printOut("The mcp verb speaks Model Context Protocol over stdin/stdout (JSON-RPC 2.0),")
	printOut("for AI clients: see `localrdp mcp --help` for the tools it exposes.")
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

// Run is the command line: it reads args (the program name first) and
// returns the exit code.
func Run(args []string) int {
	logInit()

	// 不带参数启动 exe 时直接进图形界面（双击即可用）。
	if len(args) < 2 {
		// 编译成图形子系统的 exe 本来就没有控制台；控制台子系统的开发
		// 构建会带一个，这里顺手丢攒掉。
		win32.FreeConsole()
		return runGui(nil)
	}
	// 图形子系统的 exe 从终端里跑时，把输出接回那个终端。
	if win32.AttachStdio() {
		logInfo("stdio 接到父进程的控制台")
	}
	if eqFold(args[1], "-gui") || eqFold(args[1], "--gui") {
		return runGui(args[2:])
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
			return int(ExitOk)
		}
		return runSessionEntry(expect, job)
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
			return int(ExitError)
		}
		clipBridgeMain(clipRoleChild, parent)
		return int(ExitOk)
	}

	if eqFold(args[1], "-h") || eqFold(args[1], "--help") || eqFold(args[1], "/?") {
		printHelp()
		return int(ExitOk)
	}
	if eqFold(args[1], "--version") || eqFold(args[1], "-V") {
		printOut("LocalRDP " + versionString)
		return int(ExitOk)
	}

	verb := args[1]
	if eqFold(verb, "gui") || eqFold(verb, "ui") {
		return runGui(args[2:])
	}
	if eqFold(verb, "mcp") {
		return cmdMcp(args[2:])
	}
	if eqFold(verb, "click") {
		return cmdClick(args[2:])
	}
	opt := defaultOptions()
	var programArgs []string
	afterDash := false
	optionsAllowed := eqFold(verb, "start") || eqFold(verb, "desktop") || eqFold(verb, "click")
	if !optionsAllowed && len(args) > 2 {
		printErr("`" + verb + "` does not accept options or arguments.")
		return int(ExitError)
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
			return int(ExitOk)
		}
		if eqFold(a, "--version") || eqFold(a, "-V") {
			printOut("LocalRDP " + versionString)
			return int(ExitOk)
		}
		switch strings.ToLower(a) {
		case "--fixed", "--fixed-size":
			if !optionsAllowed {
				return int(ExitError)
			}
			opt.SizeMode = store.SizeFixed
			continue
		case "--size":
			if !optionsAllowed || !needValue(i, args, a) {
				return int(ExitError)
			}
			i++
			switch strings.ToLower(strings.TrimSpace(args[i])) {
			case "fit", "follow", "follow-window":
				opt.SizeMode = store.SizeFit
			case "fixed", "lock", "locked":
				opt.SizeMode = store.SizeFixed
			case "scale", "scaled", "zoom":
				opt.SizeMode = store.SizeScale
			default:
				printErr("--size takes fit, fixed or scale, not " + args[i] + ".")
				return int(ExitError)
			}
			continue
		case "--width":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.Width) {
				return int(ExitError)
			}
			continue
		case "--height":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.Height) {
				return int(ExitError)
			}
			continue
		case "--x":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.X) {
				return int(ExitError)
			}
			continue
		case "--y":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.Y) {
				return int(ExitError)
			}
			continue
		case "--scale":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.Scale) {
				return int(ExitError)
			}
			// The session takes 100-500; anything else is a slip, not a wish.
			if opt.Scale < 100 {
				opt.Scale = 100
			}
			if opt.Scale > 500 {
				opt.Scale = 500
			}
			continue
		case "--timeout":
			if !optionsAllowed || !parseIntOption(&i, args, &opt.TimeoutSec) {
				return int(ExitError)
			}
			if opt.TimeoutSec < 5 {
				opt.TimeoutSec = 5
			}
			continue
		case "--title":
			if !optionsAllowed || !needValue(i, args, a) {
				return int(ExitError)
			}
			i++
			opt.Title = args[i]
			continue
		case "--workdir":
			if !optionsAllowed || !needValue(i, args, a) {
				return int(ExitError)
			}
			i++
			opt.WorkDir = args[i]
			continue
		case "--user":
			if !optionsAllowed || !needValue(i, args, a) {
				return int(ExitError)
			}
			i++
			opt.User = args[i]
			continue
		case "--password":
			if !optionsAllowed || !needValue(i, args, a) {
				return int(ExitError)
			}
			i++
			opt.Password = args[i]
			continue
		case "--password-env":
			if !optionsAllowed || !needValue(i, args, a) {
				return int(ExitError)
			}
			i++
			opt.Password = takeEnvVar(args[i])
			continue
		case "--minimized":
			if !optionsAllowed {
				return int(ExitError)
			}
			opt.Minimized = true
			continue
		case "--hidden":
			if !optionsAllowed {
				return int(ExitError)
			}
			opt.Hidden = true
			continue
		case "--no-clipboard":
			if !optionsAllowed {
				return int(ExitError)
			}
			opt.Clipboard = false
			continue
		case "--drives":
			if !optionsAllowed {
				return int(ExitError)
			}
			opt.Drives = true
			continue
		case "--view-only":
			if !optionsAllowed {
				return int(ExitError)
			}
			opt.ViewOnly = true
			continue
		case "--keep-session":
			if !optionsAllowed {
				return int(ExitError)
			}
			opt.KeepSess = true
			continue
		case "--fullscreen", "--full-screen":
			if !optionsAllowed {
				return int(ExitError)
			}
			opt.Fullscreen = true
			continue
		}
		if strings.HasPrefix(a, "-") {
			printErr("Unknown option: " + a)
			printErr("Try `localrdp help`.")
			return int(ExitError)
		}
		programArgs = append(programArgs, a)
	}

	logInfo("verb=" + verb)

	switch {
	case eqFold(verb, "help"):
		printHelp()
		return int(ExitOk)
	case eqFold(verb, "version"):
		printOut("LocalRDP " + versionString)
		return int(ExitOk)
	case eqFold(verb, "status"):
		r := queryEnvironment()
		printStatus(r)
		if r.ready() {
			return int(ExitOk)
		}
		return int(ExitNotReady)
	case eqFold(verb, "setup"):
		printErr("`setup` has been removed. Child sessions are enabled automatically by start / desktop.")
		printErr("LocalRDP does not change the firewall, RDP listener, TermService, or credential policy.")
		return int(ExitError)
	case eqFold(verb, "stop"):
		return cmdStop()
	case eqFold(verb, "gui"), eqFold(verb, "ui"):
		return runGui(nil)
	case eqFold(verb, "desktop"):
		opt.Desktop = true
		opt.Command = splitCommand(programArgs)
		return cmdStart(opt)
	case eqFold(verb, "start"):
		if len(programArgs) == 0 {
			printErr("start requires a program. Example: localrdp start -- notepad.exe")
			printErr("For a full desktop, use: localrdp desktop")
			return int(ExitError)
		}
		opt.Desktop = false
		opt.Command = splitCommand(programArgs)
		return cmdStart(opt)
	}

	printErr("Unknown command: " + verb)
	printErr("Try `localrdp help`.")
	return int(ExitError)
}
func parseU32(s string) uint32 {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return 0
	}
	return uint32(v)
}
