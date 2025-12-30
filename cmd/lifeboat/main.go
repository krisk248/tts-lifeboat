// TTS Lifeboat - Enterprise backup solution for Tomcat applications
// Created with ❤️ by Kannan
package main

import (
	"fmt"
	"os"

	"github.com/kannan/tts-lifeboat/internal/cli"
)

func main() {
	// Detect if we should run interactive UI or CLI
	// Interactive UI is launched when:
	// 1. No command-line arguments (just "lifeboat")
	// 2. Running in interactive terminal
	// 3. Not piped/redirected

	if shouldRunUI() {
		if err := runUI(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Press Enter to exit...")
			fmt.Scanln()
			os.Exit(1)
		}
		return
	}

	// Run CLI
	cli.Execute()
}

// shouldRunUI determines if interactive UI should be launched.
func shouldRunUI() bool {
	// If there are command arguments, use CLI
	if len(os.Args) > 1 {
		return false
	}

	// Check if running in interactive terminal
	// This checks if stdin/stdout are connected to a terminal
	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}

	stdoutInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	// If stdin or stdout is a pipe/redirect, use CLI
	if (stdinInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}
	if (stdoutInfo.Mode() & os.ModeCharDevice) == 0 {
		return false
	}

	return true
}
