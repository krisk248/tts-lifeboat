//go:build !windows

package console

// setTitle is a no-op on non-Windows systems.
// Linux/Unix terminals handle titles via shell or terminal emulator.
func setTitle(title string) {
	// No-op on Linux/Unix
}
