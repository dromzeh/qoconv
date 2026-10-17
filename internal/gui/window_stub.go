//go:build !production

package gui

// Run reports ErrUnavailable: this build has no window. Build with
// "-tags production" (Wails' release tag) to include it.
func Run(Config) error { return ErrUnavailable }
