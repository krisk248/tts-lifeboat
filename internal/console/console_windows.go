//go:build windows

package console

import (
	"syscall"
	"unsafe"
)

var (
	kernel32        = syscall.NewLazyDLL("kernel32.dll")
	setConsoleTitle = kernel32.NewProc("SetConsoleTitleW")
)

func setTitle(title string) {
	// Convert string to UTF-16 for Windows API
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	setConsoleTitle.Call(uintptr(unsafe.Pointer(titlePtr)))
}
