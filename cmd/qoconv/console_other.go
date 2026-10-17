//go:build !windows

package main

// setupConsole is a no-op: only Windows builds start without a console.
func setupConsole(cli, tui bool) (done func()) { return func() {} }
