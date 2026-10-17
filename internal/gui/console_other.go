//go:build production && !windows

package gui

// detachConsole is a no-op: only Windows opens a console for a GUI launch.
func detachConsole() {}
