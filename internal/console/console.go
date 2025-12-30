// Package console provides cross-platform console utilities.
package console

// SetTitle sets the console window title.
// On Windows, this uses the Windows API.
// On Linux/Unix, this is a no-op (terminals handle titles differently).
func SetTitle(title string) {
	setTitle(title)
}
