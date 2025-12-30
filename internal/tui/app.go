// Package tui provides the terminal user interface for tts-lifeboat.
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kannan/tts-lifeboat/internal/app"
	"github.com/kannan/tts-lifeboat/internal/backup"
	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/console"
	"github.com/kannan/tts-lifeboat/internal/tui/styles"
)

// Screen represents the current TUI screen.
type Screen int

const (
	ScreenWelcome Screen = iota
	ScreenBackup
	ScreenProgress
	ScreenRestore
	ScreenList
	ScreenComplete
	ScreenError
)

// Model is the main TUI application model.
type Model struct {
	screen       Screen
	cfg          *config.Config
	backup       *backup.Backup
	retention    *backup.RetentionManager
	width        int
	height       int
	menuIndex    int
	menuItems    []MenuItem
	backups      []backup.IndexEntry
	selectedID   string
	message      string
	error        error
	progress     float64
	progressMsg  string
	result       *backup.BackupResult
	easterEgg    string
	inputBuffer  string
}

// MenuItem represents a menu option.
type MenuItem struct {
	Key      string
	Label    string
	Disabled bool
}

// Init initializes the TUI model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Run starts the TUI application.
func Run() error {
	// Set Windows console title
	console.SetTitle(fmt.Sprintf("TTS Lifeboat v%s - Enterprise Backup", app.Version))

	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	m := Model{
		screen:    ScreenWelcome,
		cfg:       cfg,
		backup:    backup.New(cfg),
		retention: backup.NewRetentionManager(cfg),
		menuItems: []MenuItem{
			{Key: "b", Label: "New Backup"},
			{Key: "r", Label: "Restore"},
			{Key: "l", Label: "List Backups"},
			{Key: "c", Label: "Cleanup"},
			{Key: "q", Label: "Quit"},
		},
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err = p.Run()
	return err
}

// Update handles messages and updates the model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case backupCompleteMsg:
		m.screen = ScreenComplete
		m.result = msg.result
		return m, nil

	case backupProgressMsg:
		m.progress = msg.percent
		m.progressMsg = msg.message
		return m, nil

	case backupErrorMsg:
		m.screen = ScreenError
		m.error = msg.err
		return m, nil
	}

	return m, nil
}

// handleKeyPress processes keyboard input.
func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Check for easter egg
	m.inputBuffer += key
	if len(m.inputBuffer) > 10 {
		m.inputBuffer = m.inputBuffer[len(m.inputBuffer)-10:]
	}
	if strings.Contains(strings.ToLower(m.inputBuffer), "kannan") {
		m.easterEgg = app.GetEasterEgg()
	}

	switch m.screen {
	case ScreenWelcome:
		switch key {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "b":
			return m.startBackup()
		case "r":
			return m.showRestore()
		case "l":
			return m.showList()
		case "c":
			return m.runCleanup()
		case "up", "k":
			if m.menuIndex > 0 {
				m.menuIndex--
			}
		case "down", "j":
			if m.menuIndex < len(m.menuItems)-1 {
				m.menuIndex++
			}
		case "enter":
			return m.selectMenuItem()
		case "?":
			// Toggle help
		case "escape":
			m.easterEgg = ""
		}

	case ScreenList:
		switch key {
		case "escape", "q":
			m.screen = ScreenWelcome
		case "up", "k":
			if m.menuIndex > 0 {
				m.menuIndex--
			}
		case "down", "j":
			if m.menuIndex < len(m.backups)-1 {
				m.menuIndex++
			}
		case "enter", "r":
			if len(m.backups) > 0 {
				m.selectedID = m.backups[m.menuIndex].ID
				return m.doRestore()
			}
		case "p":
			// Mark as checkpoint
			if len(m.backups) > 0 {
				m.backup.MarkCheckpoint(m.backups[m.menuIndex].ID, "")
				return m.showList()
			}
		}

	case ScreenProgress:
		switch key {
		case "escape":
			// Cancel backup (would need more complex handling)
			m.screen = ScreenWelcome
		}

	case ScreenComplete, ScreenError:
		switch key {
		case "enter", "escape", "q":
			m.screen = ScreenWelcome
			m.error = nil
			m.result = nil
		}

	case ScreenRestore:
		switch key {
		case "escape", "q":
			m.screen = ScreenWelcome
		}
	}

	return m, nil
}

// View renders the current screen.
func (m Model) View() string {
	if m.easterEgg != "" {
		return m.easterEgg + "\n\nPress ESC to continue..."
	}

	switch m.screen {
	case ScreenWelcome:
		return m.viewWelcome()
	case ScreenList:
		return m.viewList()
	case ScreenProgress:
		return m.viewProgress()
	case ScreenComplete:
		return m.viewComplete()
	case ScreenError:
		return m.viewError()
	default:
		return m.viewWelcome()
	}
}

// viewWelcome renders the welcome screen.
func (m Model) viewWelcome() string {
	var sb strings.Builder

	// Banner
	banner := `
╭──────────────────────────────────────────────────────────────╮
│   ████████╗████████╗███████╗    ██╗     ██╗███████╗███████╗  │
│   ╚══██╔══╝╚══██╔══╝██╔════╝    ██║     ██║██╔════╝██╔════╝  │
│      ██║      ██║   ███████╗    ██║     ██║█████╗  █████╗    │
│      ██║      ██║   ╚════██║    ██║     ██║██╔══╝  ██╔══╝    │
│      ╚═╝      ╚═╝   ╚══════╝    ╚══════╝╚═╝╚═╝     ╚══════╝  │
│                                                              │
│               LIFEBOAT - Enterprise Backup                   │`

	sb.WriteString(styles.BannerStyle.Render(banner))
	sb.WriteString("\n")

	// Version and instance info
	info := fmt.Sprintf(`│   v%s                                                     │
│                                                              │
│   Instance: %-30s             │
│   Environment: %-27s             │`, app.Version, truncate(m.cfg.Name, 30), truncate(m.cfg.Environment, 27))
	sb.WriteString(info)
	sb.WriteString("\n")

	// Get backup stats
	stats, _ := m.retention.GetBackupStats()
	if stats != nil && stats.TotalBackups > 0 {
		lastBackup := "never"
		if stats.NewestBackup != nil {
			lastBackup = stats.NewestBackup.Date.Format("2006-01-02 15:04")
		}
		statsLine := fmt.Sprintf("│   Backups: %-10d Last: %-23s │", stats.TotalBackups, lastBackup)
		sb.WriteString(statsLine)
		sb.WriteString("\n")
	}

	sb.WriteString("│                                                              │\n")
	sb.WriteString("╰──────────────────────────────────────────────────────────────╯\n")
	sb.WriteString("\n")

	// Menu
	for i, item := range m.menuItems {
		cursor := "  "
		style := styles.MenuItemStyle
		if i == m.menuIndex {
			cursor = "▸ "
			style = styles.MenuItemSelectedStyle
		}

		label := fmt.Sprintf("%s[%s] %s", cursor, item.Key, item.Label)
		sb.WriteString(style.Render(label))
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(styles.CreatorStyle.Render("Created by " + app.Creator + " • Press ? for help"))

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, sb.String())
}

// viewList renders the backup list screen.
func (m Model) viewList() string {
	var sb strings.Builder

	sb.WriteString(styles.TitleStyle.Render("🚢 Backup History"))
	sb.WriteString("\n\n")

	if len(m.backups) == 0 {
		sb.WriteString(styles.MutedStyle().Render("No backups found."))
		sb.WriteString("\n\n")
		sb.WriteString(styles.FooterStyle.Render("[ESC] Back"))
		return sb.String()
	}

	// List backups
	for i, bk := range m.backups {
		cursor := "  "
		style := styles.MenuItemStyle
		if i == m.menuIndex {
			cursor = "▸ "
			style = styles.MenuItemSelectedStyle
		}

		dateStr := bk.Date.Format("2006-01-02 15:04")
		status := ""
		if bk.Checkpoint {
			status = " ⭐"
		}

		line := fmt.Sprintf("%s%-26s  %s  %s%s", cursor, bk.ID, dateStr, bk.Size, status)
		sb.WriteString(style.Render(line))
		sb.WriteString("\n")

		if bk.Note != "" && i == m.menuIndex {
			sb.WriteString(styles.SubtitleStyle.Render(fmt.Sprintf("    📝 %s", bk.Note)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n")
	sb.WriteString(styles.FooterStyle.Render("[Enter] Restore  [P] Checkpoint  [ESC] Back"))

	return sb.String()
}

// viewProgress renders the progress screen.
func (m Model) viewProgress() string {
	var sb strings.Builder

	sb.WriteString(styles.TitleStyle.Render("🚢 Backup in Progress"))
	sb.WriteString("\n\n")

	// Progress bar
	bar := styles.ProgressBar(m.progress, 40)
	pct := fmt.Sprintf("%.0f%%", m.progress*100)
	sb.WriteString(fmt.Sprintf("  %s %s\n", bar, pct))
	sb.WriteString("\n")

	// Current file
	if m.progressMsg != "" {
		sb.WriteString(styles.SubtitleStyle.Render("  " + truncate(m.progressMsg, 50)))
	}

	sb.WriteString("\n\n")
	sb.WriteString(styles.FooterStyle.Render("[ESC] Cancel"))

	return sb.String()
}

// viewComplete renders the completion screen.
func (m Model) viewComplete() string {
	var sb strings.Builder

	sb.WriteString(styles.SuccessStyle.Render("✅ BACKUP COMPLETE"))
	sb.WriteString("\n\n")

	if m.result != nil {
		sb.WriteString(fmt.Sprintf("  ID:       %s\n", m.result.ID))
		sb.WriteString(fmt.Sprintf("  Files:    %d\n", m.result.FilesProcessed))
		sb.WriteString(fmt.Sprintf("  Size:     %s → %s\n",
			backup.FormatSize(m.result.OriginalSize),
			backup.FormatSize(m.result.CompressedSize)))
		sb.WriteString(fmt.Sprintf("  Duration: %s\n", m.result.Duration.Round(100000000)))
	}

	sb.WriteString("\n")
	sb.WriteString(styles.FooterStyle.Render("[Enter] Continue"))

	return sb.String()
}

// viewError renders the error screen.
func (m Model) viewError() string {
	var sb strings.Builder

	sb.WriteString(styles.ErrorStyle.Render("❌ ERROR"))
	sb.WriteString("\n\n")

	if m.error != nil {
		sb.WriteString(styles.ErrorStyle.Render(m.error.Error()))
	}

	sb.WriteString("\n\n")
	sb.WriteString(styles.FooterStyle.Render("[Enter] Continue"))

	return sb.String()
}

// Helper methods

func (m Model) selectMenuItem() (tea.Model, tea.Cmd) {
	if m.menuIndex >= len(m.menuItems) {
		return m, nil
	}

	item := m.menuItems[m.menuIndex]
	switch item.Key {
	case "b":
		return m.startBackup()
	case "r":
		return m.showRestore()
	case "l":
		return m.showList()
	case "c":
		return m.runCleanup()
	case "q":
		return m, tea.Quit
	}

	return m, nil
}

func (m Model) startBackup() (tea.Model, tea.Cmd) {
	m.screen = ScreenProgress
	m.progress = 0
	m.progressMsg = "Starting..."

	return m, m.doBackup()
}

func (m Model) doBackup() tea.Cmd {
	return func() tea.Msg {
		opts := backup.BackupOptions{}

		result, err := m.backup.Run(opts, func(phase string, current, total int, message string) {
			if total > 0 {
				// Can't send messages from here in this simple model
				// Would need channels or async pattern
			}
		})

		if err != nil {
			return backupErrorMsg{err: err}
		}

		return backupCompleteMsg{result: result}
	}
}

func (m Model) showList() (tea.Model, tea.Cmd) {
	backups, err := m.backup.List()
	if err != nil {
		m.error = err
		m.screen = ScreenError
		return m, nil
	}

	m.backups = backups
	m.menuIndex = 0
	m.screen = ScreenList
	return m, nil
}

func (m Model) showRestore() (tea.Model, tea.Cmd) {
	return m.showList()
}

func (m Model) doRestore() (tea.Model, tea.Cmd) {
	// Simplified - would need more UI for target selection
	m.message = "Restore functionality requires CLI. Use: lifeboat restore " + m.selectedID
	m.screen = ScreenError
	m.error = fmt.Errorf("for restore, use CLI: lifeboat restore %s", m.selectedID)
	return m, nil
}

func (m Model) runCleanup() (tea.Model, tea.Cmd) {
	result, err := m.retention.Cleanup(true) // Dry run
	if err != nil {
		m.error = err
		m.screen = ScreenError
		return m, nil
	}

	m.message = fmt.Sprintf("Cleanup preview: %d backups to delete, %s to free",
		result.BackupsDeleted, backup.FormatSize(result.SpaceFreed))

	// For actual cleanup, prompt user or redirect to CLI
	return m, nil
}

// Messages

type backupCompleteMsg struct {
	result *backup.BackupResult
}

type backupProgressMsg struct {
	percent float64
	message string
}

type backupErrorMsg struct {
	err error
}

// Helpers

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
