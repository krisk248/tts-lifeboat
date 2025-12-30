//go:build windows

package console

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

var (
	kernel32                       = syscall.NewLazyDLL("kernel32.dll")
	setConsoleTitleProc            = kernel32.NewProc("SetConsoleTitleW")
	getStdHandleProc               = kernel32.NewProc("GetStdHandle")
	getConsoleScreenBufferInfoProc = kernel32.NewProc("GetConsoleScreenBufferInfo")
	fillConsoleOutputCharacterProc = kernel32.NewProc("FillConsoleOutputCharacterW")
	setConsoleCursorPositionProc   = kernel32.NewProc("SetConsoleCursorPosition")
)

const (
	stdOutputHandle = ^uintptr(0) - 11 + 1 // STD_OUTPUT_HANDLE = -11
)

type coord struct {
	x int16
	y int16
}

type smallRect struct {
	left   int16
	top    int16
	right  int16
	bottom int16
}

type consoleScreenBufferInfo struct {
	size              coord
	cursorPosition    coord
	attributes        uint16
	window            smallRect
	maximumWindowSize coord
}

func setTitle(title string) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	setConsoleTitleProc.Call(uintptr(unsafe.Pointer(titlePtr)))
}

// ClearScreen clears the console screen using Windows Console API.
// This works on all Windows versions including Windows 2008 R2.
func ClearScreen() {
	// Try using cmd /c cls first (most reliable)
	cmd := exec.Command("cmd", "/c", "cls")
	cmd.Stdout = os.Stdout
	cmd.Run()
}

// ClearScreenAPI clears using Windows Console API directly.
// Fallback if cmd /c cls doesn't work.
func ClearScreenAPI() {
	handle, _, _ := getStdHandleProc.Call(stdOutputHandle)
	if handle == 0 {
		return
	}

	var info consoleScreenBufferInfo
	ret, _, _ := getConsoleScreenBufferInfoProc.Call(handle, uintptr(unsafe.Pointer(&info)))
	if ret == 0 {
		return
	}

	// Calculate console size
	consoleSize := uint32(info.size.x) * uint32(info.size.y)

	// Fill with spaces
	var written uint32
	origin := coord{x: 0, y: 0}
	fillConsoleOutputCharacterProc.Call(
		handle,
		uintptr(' '),
		uintptr(consoleSize),
		*(*uintptr)(unsafe.Pointer(&origin)),
		uintptr(unsafe.Pointer(&written)),
	)

	// Move cursor to top-left
	setConsoleCursorPositionProc.Call(handle, *(*uintptr)(unsafe.Pointer(&origin)))
}
