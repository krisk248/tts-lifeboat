//go:build !windows

package console

import "fmt"

// setTitle is a no-op on non-Windows systems.
func setTitle(title string) {
	// No-op on Linux/Unix
}

// ClearScreen clears the terminal using ANSI escape codes.
func ClearScreen() {
	fmt.Print("\033[2J\033[H")
}
