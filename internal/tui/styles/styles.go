// Package styles provides Lipgloss styling for the TUI.
package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	Primary     = lipgloss.Color("#00BFFF") // Deep Sky Blue
	Secondary   = lipgloss.Color("#32CD32") // Lime Green
	Accent      = lipgloss.Color("#FFD700") // Gold
	Danger      = lipgloss.Color("#FF6347") // Tomato
	Muted       = lipgloss.Color("#808080") // Gray
	Success     = lipgloss.Color("#00FF7F") // Spring Green
	Warning     = lipgloss.Color("#FFA500") // Orange
	BgDark      = lipgloss.Color("#1a1a2e") // Dark background
	BgLight     = lipgloss.Color("#16213e") // Lighter background
	BorderColor = lipgloss.Color("#0f3460") // Border color

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(BorderColor).
			Padding(1, 2)

	// Title style
	TitleStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true).
			MarginBottom(1)

	// Subtitle style
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Muted).
			Italic(true)

	// Menu item styles
	MenuItemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			PaddingLeft(2)

	MenuItemSelectedStyle = lipgloss.NewStyle().
				Foreground(Primary).
				Bold(true).
				PaddingLeft(2)

	MenuItemDisabledStyle = lipgloss.NewStyle().
				Foreground(Muted).
				PaddingLeft(2)

	// Status styles
	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Danger).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning).
			Bold(true)

	// Progress bar styles
	ProgressBarEmpty = lipgloss.NewStyle().
				Foreground(Muted)

	ProgressBarFilled = lipgloss.NewStyle().
				Foreground(Primary)

	// Footer style
	FooterStyle = lipgloss.NewStyle().
			Foreground(Muted).
			MarginTop(1)

	// Help key style
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(Accent)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(Muted)

	// Checkbox styles
	CheckboxChecked = lipgloss.NewStyle().
			Foreground(Success).
			SetString("[✓]")

	CheckboxUnchecked = lipgloss.NewStyle().
				Foreground(Muted).
				SetString("[ ]")

	// Badge styles
	BadgeCheckpoint = lipgloss.NewStyle().
			Background(Accent).
			Foreground(lipgloss.Color("#000000")).
			Padding(0, 1).
			SetString("CHECKPOINT")

	BadgeExpired = lipgloss.NewStyle().
			Background(Danger).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1).
			SetString("EXPIRED")

	// Info box style
	InfoBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(0, 1).
			MarginTop(1)

	// Error box style
	ErrorBoxStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(Danger).
			Padding(0, 1).
			MarginTop(1)

	// ASCII Art banner style
	BannerStyle = lipgloss.NewStyle().
			Foreground(Primary).
			Bold(true)

	// Creator credit style
	CreatorStyle = lipgloss.NewStyle().
			Foreground(Accent).
			Italic(true)
)

// ProgressBar returns a progress bar string.
func ProgressBar(percent float64, width int) string {
	filled := int(percent * float64(width))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}

	bar := ""
	for i := 0; i < width; i++ {
		if i < filled {
			bar += ProgressBarFilled.Render("█")
		} else {
			bar += ProgressBarEmpty.Render("░")
		}
	}

	return bar
}

// RenderHelp renders a help line with key bindings.
func RenderHelp(bindings map[string]string) string {
	help := ""
	for key, desc := range bindings {
		help += HelpKeyStyle.Render("["+key+"]") + " " + HelpDescStyle.Render(desc) + "  "
	}
	return help
}

// MutedStyle returns a muted text style.
func MutedStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(Muted)
}
