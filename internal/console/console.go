// Package console provides cross-platform console utilities.
package console

// SetTitle sets the console window title.
// On Windows, this uses the Windows API.
// On Linux/Unix, this is a no-op (terminals handle titles differently).
func SetTitle(title string) {
	setTitle(title)
}

// Clear clears the console screen.
// On Windows, this uses cmd /c cls or Windows Console API.
// On Linux/Unix, this uses ANSI escape codes.
func Clear() {
	ClearScreen()
}
