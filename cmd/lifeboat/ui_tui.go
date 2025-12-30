//go:build !legacy

// TTS Lifeboat - Modern TUI interface using Bubble Tea
// This file is used for modern builds (Go 1.24+)
package main

import "github.com/kannan/tts-lifeboat/internal/tui"

// runUI starts the modern Bubble Tea TUI.
func runUI() error {
	return tui.Run()
}
