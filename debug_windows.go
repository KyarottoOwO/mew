//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procAllocConsole = kernel32.NewProc("AllocConsole")
	procGetStdHandle = kernel32.NewProc("GetStdHandle")
)

const (
	stdOutputHandle = ^uintptr(11) // STD_OUTPUT_HANDLE
	stdErrorHandle  = ^uintptr(12) // STD_ERROR_HANDLE
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

	fmt.Fprintln(os.Stderr, "Debug console active")
}
