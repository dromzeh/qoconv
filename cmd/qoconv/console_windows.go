package main

import (
	"bufio"
	"fmt"
	"os"
	"syscall"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetConsoleWindow = kernel32.NewProc("GetConsoleWindow")
	procAttachConsole    = kernel32.NewProc("AttachConsole")
	procAllocConsole     = kernel32.NewProc("AllocConsole")
)

const attachParentProcess = ^uintptr(0) // ATTACH_PARENT_PROCESS, (DWORD)-1

// setupConsole gives qoconv the console its command-line modes need.
//
// Windows release builds are GUI programs (-H windowsgui), so double-clicking
// opens the app without flashing a console window first; the price is that
// they start with no console at all. So: CLI use attaches to the terminal it
// was run from, and the TUI, which needs keyboard input the terminal's shell
// would also be reading, gets a console window of its own. A console build
// (plain go build) already has a console and is left alone.
//
// The returned func runs before exit: a console qoconv opened itself waits for
// Enter, so its output can be read before the window closes.
func setupConsole(cli, tui bool) (done func()) {
	done = func() {}
	if hwnd, _, _ := procGetConsoleWindow.Call(); hwnd != 0 {
		return done
	}
	switch {
	case tui:
		if ok, _, _ := procAllocConsole.Call(); ok != 0 {
			useConsole(true)
			return func() {
				fmt.Print("\nPress Enter to close.")
				bufio.NewReader(os.Stdin).ReadString('\n')
			}
		}
	case cli:
		if ok, _, _ := procAttachConsole.Call(attachParentProcess); ok != 0 {
			useConsole(false)
		}
	}
	return done
}

// useConsole points the standard streams at the attached console, except
// ones the caller redirected (e.g. qoconv --input x > log.txt).
func useConsole(input bool) {
	if out, err := os.OpenFile("CONOUT$", os.O_RDWR, 0); err == nil {
		if !redirected(os.Stdout) {
			os.Stdout = out
		}
		if !redirected(os.Stderr) {
			os.Stderr = out
		}
	}
	if input {
		if in, err := os.OpenFile("CONIN$", os.O_RDWR, 0); err == nil {
			os.Stdin = in
		}
	}
}

// redirected reports whether a GUI program was handed a real stream for f
// (it gets none otherwise).
func redirected(f *os.File) bool {
	if f == nil {
		return false
	}
	h := syscall.Handle(f.Fd())
	return h != 0 && h != syscall.InvalidHandle
}
