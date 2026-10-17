//go:build production

package gui

import (
	"syscall"
	"unsafe"
)

var (
	kernel32              = syscall.NewLazyDLL("kernel32.dll")
	getConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	freeConsole           = kernel32.NewProc("FreeConsole")
)

// detachConsole closes the console window Windows opened for qoconv when it
// was double-clicked. qoconv is a console program so the CLI can print; when
// it started its own console (we're the only process attached), the GUI
// doesn't need it. Launched from a terminal, the terminal is left alone.
func detachConsole() {
	var pids [2]uint32
	n, _, _ := getConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n == 1 {
		freeConsole.Call()
	}
}
