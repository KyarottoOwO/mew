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
	errLogMaxSize = 1 << 20 // rotate after 1 MB
	errLogDateFmt = "2006-01-02 15:04:05"
)

var (
	errLogMu sync.Mutex
	errLogDir string
)

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
	errLogMu.Lock()
	defer errLogMu.Unlock()

	path := filepath.Join(getErrLogDir(), "errors.log")

	rotateIfTooLarge(path)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()

	ts := time.Now().Format(errLogDateFmt)
	fmt.Fprintf(f, "[%s] %s\n", ts, msg)
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
