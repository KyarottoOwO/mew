package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	errLogMaxSize = 1 << 20
	errLogDateFmt = "2006-01-02 15:04:05"
)

var (
	errLogMu  sync.Mutex
	errLogDir string
	debugConsoleEnabled bool
)

const (
	ansiReset   = "\033[0m"
	ansiRed     = "\033[31m"
	ansiGreen   = "\033[32m"
	ansiYellow  = "\033[33m"
	ansiCyan    = "\033[36m"
	ansiGray    = "\033[90m"
)

func consoleLog(color, msg string) {
	if !debugConsoleEnabled {
		return
	}
	fmt.Fprintf(os.Stderr, "%s%s%s\n", color, msg, ansiReset)
}

func getErrLogDir() string {
	if errLogDir != "" {
		return errLogDir
	}
	dir := os.Getenv("LOCALAPPDATA")
	if dir == "" {
		dir = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Local")
	}
	errLogDir = filepath.Join(dir, "mew", "Logs")
	os.MkdirAll(errLogDir, 0o755)
	return errLogDir
}

func logError(msg string) {
	ts := time.Now().Format(errLogDateFmt)
	formatted := fmt.Sprintf("[%s] %s", ts, msg)
	consoleLog(ansiRed, formatted)

	errLogMu.Lock()
	defer errLogMu.Unlock()

	path := filepath.Join(getErrLogDir(), "errors.log")
	rotateIfTooLarge(path)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintln(f, formatted)
}

func rotateIfTooLarge(path string) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < errLogMaxSize {
		return
	}
	os.Rename(path, path+".old")
}

func formatError(prefix string, err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s", prefix, strings.TrimSpace(err.Error()))
}

func logErrorf(format string, args ...interface{}) {
	logError(fmt.Sprintf(format, args...))
}

func debugLog(msg string) {
	ts := time.Now().Format(errLogDateFmt)
	formatted := fmt.Sprintf("[%s] %s", ts, msg)

	if strings.Contains(msg, "[ERROR]") {
		consoleLog(ansiRed, formatted)
	} else if strings.Contains(msg, "[WARN]") {
		consoleLog(ansiYellow, formatted)
	} else {
		consoleLog(ansiGray, formatted)
	}

	errLogMu.Lock()
	defer errLogMu.Unlock()

	path := filepath.Join(getErrLogDir(), "debug.log")
	rotateIfTooLarge(path)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintln(f, formatted)
}

func debugLogf(format string, args ...interface{}) {
	debugLog(fmt.Sprintf(format, args...))
}
