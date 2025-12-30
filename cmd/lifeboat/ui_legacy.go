//go:build legacy

// TTS Lifeboat - Legacy interactive CLI
// This file is used for legacy builds (Go 1.20) for systems like Windows 2008 R2
package main

import "github.com/kannan/tts-lifeboat/internal/interactive"

// runUI starts the simple text-based interactive CLI.
func runUI() error {
	return interactive.Run()
}
