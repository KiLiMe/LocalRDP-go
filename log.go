package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var (
	logMu    sync.Mutex
	logPath  string
	logOnce  sync.Once
	logFile  *os.File
	quietLog bool
)

// logInit mirrors LogInit(): create %LOCALAPPDATA%\LocalRDP and open localrdp.log.
func logInit() {
	logOnce.Do(func() {
		procSetConsoleCP.Call(65001) // CP_UTF8
		dir := filepath.Join(localAppData(), "LocalRDP")
		ensureDir(dir)
		logPath = filepath.Join(dir, "localrdp.log")
		f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			logFile = f
		}
	})
}

func appendLog(level, msg string) {
	if quietLog {
		return
	}
	logInit()
	logMu.Lock()
	defer logMu.Unlock()
	if logFile == nil {
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05.000")
	logFile.WriteString(ts + " [" + level + "] " + msg + "\n")
	if level == "ERROR" {
		logFile.Sync()
	}
}

func logInfo(msg string)           { appendLog("INFO", msg) }
func logf(format string, a ...any) { appendLog("INFO", fmt.Sprintf(format, a...)) }
func logWarn(msg string)           { appendLog("WARN", msg) }
func logError(msg string)          { appendLog("ERROR", msg) }

func printOut(msg string) { os.Stdout.WriteString(msg + "\r\n") }
func printErr(msg string) { os.Stderr.WriteString(msg + "\r\n") }

func printOutf(format string, a ...any) { printOut(fmt.Sprintf(format, a...)) }
func printErrf(format string, a ...any) { printErr(fmt.Sprintf(format, a...)) }
