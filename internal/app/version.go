// Package app provides application-level functionality for tts-lifeboat.
package app

import (
	"fmt"
	"runtime"
)

var (
	// Version is the application version (set at build time).
	Version = "dev"
	// Commit is the git commit hash (set at build time).
	Commit = "unknown"
	// Date is the build date (set at build time).
	Date = "unknown"
	// Creator is the application creator.
	Creator = "Kannan"
)

// GetVersion returns the full version string.
func GetVersion() string {
	return fmt.Sprintf("%s (%s)", Version, Commit[:min(7, len(Commit))])
}

// GetVersionInfo returns detailed version information.
func GetVersionInfo() string {
	return fmt.Sprintf(`TTS Lifeboat v%s
Commit: %s
Built:  %s
Go:     %s
OS:     %s/%s
Creator: %s`,
		Version, Commit, Date, runtime.Version(), runtime.GOOS, runtime.GOARCH, Creator)
}

// GetBanner returns the ASCII art banner.
func GetBanner() string {
	return `
████████╗████████╗███████╗    ██╗     ██╗███████╗███████╗██████╗  ██████╗  █████╗ ████████╗
╚══██╔══╝╚══██╔══╝██╔════╝    ██║     ██║██╔════╝██╔════╝██╔══██╗██╔═══██╗██╔══██╗╚══██╔══╝
   ██║      ██║   ███████╗    ██║     ██║█████╗  █████╗  ██████╔╝██║   ██║███████║   ██║
   ██║      ██║   ╚════██║    ██║     ██║██╔══╝  ██╔══╝  ██╔══██╗██║   ██║██╔══██║   ██║
   ██║      ██║   ███████║    ███████╗██║██║     ███████╗██████╔╝╚██████╔╝██║  ██║   ██║
   ╚═╝      ╚═╝   ╚══════╝    ╚══════╝╚═╝╚═╝     ╚══════╝╚═════╝  ╚═════╝ ╚═╝  ╚═╝   ╚═╝
`
}

// GetSmallBanner returns a smaller ASCII art banner.
func GetSmallBanner() string {
	return `
╭─────────────────────────────────────────╮
│  ████████╗████████╗███████╗             │
│  ╚══██╔══╝╚══██╔══╝██╔════╝  LIFEBOAT   │
│     ██║      ██║   ███████╗  v` + Version + `      │
│     ██║      ██║   ╚════██║             │
│     ╚═╝      ╚═╝   ╚══════╝             │
╰─────────────────────────────────────────╯
`
}

// GetEasterEgg returns a fun easter egg message.
func GetEasterEgg() string {
	return fmt.Sprintf(`
╭─────────────────────────────────────────╮
│   🚢 TTS Lifeboat                       │
│   "Your Tomcat's Best Friend"           │
│                                         │
│   Created with ❤️  by %s              │
│                                         │
│   "In case of sinking Tomcat,           │
│    grab the Lifeboat!"                  │
╰─────────────────────────────────────────╯
`, Creator)
}

// GetCredits returns full credits.
func GetCredits() string {
	return fmt.Sprintf(`
╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║   TTS LIFEBOAT - Enterprise Backup Solution                   ║
║                                                               ║
║   Created by: %s                                           ║
║                                                               ║
║   "Saving your Tomcat applications,                           ║
║    one backup at a time."                                     ║
║                                                               ║
║   Built with Go • Powered by passion                          ║
║                                                               ║
║   Special thanks to:                                          ║
║   - Charm.sh for Bubble Tea & Lipgloss                        ║
║   - Steve Francia for Cobra                                   ║
║   - Klaus Post for fast compression                           ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝
`, Creator)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
