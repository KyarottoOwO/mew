//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	procAllocConsole    = kernel32.NewProc("AllocConsole")
	procGetStdHandle    = kernel32.NewProc("GetStdHandle")
	procSetConsoleCP    = kernel32.NewProc("SetConsoleCP")
	procSetConsoleOP    = kernel32.NewProc("SetConsoleOutputCP")
	procSetConsoleTitle = kernel32.NewProc("SetConsoleTitleW")
)

const (
	stdOutputHandle = ^uintptr(11)
	stdErrorHandle  = ^uintptr(12)
	utf8Codepage    = 65001
)

func allocConsole() {
	procAllocConsole.Call()

	hStdout, _, _ := procGetStdHandle.Call(stdOutputHandle)
	hStderr, _, _ := procGetStdHandle.Call(stdErrorHandle)

	if hStdout != 0 && hStdout != ^uintptr(0) {
		os.Stdout = os.NewFile(hStdout, "os.Stdout")
	}
	if hStderr != 0 && hStderr != ^uintptr(0) {
		os.Stderr = os.NewFile(hStderr, "os.Stderr")
	}

	procSetConsoleCP.Call(utf8Codepage)
	procSetConsoleOP.Call(utf8Codepage)

	title, _ := syscall.UTF16PtrFromString("MEW Debug Console")
	procSetConsoleTitle.Call(uintptr(unsafe.Pointer(title)))

	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "\033[36m╔══════════════════════════════════════╗\033[0m")
	fmt.Fprintln(os.Stderr, "\033[36m║\033[0m \033[1;36mMEW Debug Console\033[0m                     \033[36m║\033[0m")
	fmt.Fprintln(os.Stderr, "\033[36m║\033[0m DevTools opens automatically            \033[36m║\033[0m")
	fmt.Fprintln(os.Stderr, "\033[36m║\033[0m Logs: debug.log | errors.log         \033[36m║\033[0m")
	fmt.Fprintln(os.Stderr, "\033[36m╚══════════════════════════════════════╝\033[0m")
	fmt.Fprintln(os.Stderr, "")
}
