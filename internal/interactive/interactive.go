// Package interactive provides a simple text-based interactive CLI for legacy systems.
// This package is used when building with the "legacy" build tag for systems
// that don't support Go 1.24+ (like Windows 2008 R2).
package interactive

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kannan/tts-lifeboat/internal/app"
	"github.com/kannan/tts-lifeboat/internal/backup"
	"github.com/kannan/tts-lifeboat/internal/config"
)

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorRed    = "\033[31m"
	colorBold   = "\033[1m"
)

// Run starts the interactive CLI application.
func Run() error {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	b := backup.New(cfg)
	retention := backup.NewRetentionManager(cfg)
	reader := bufio.NewReader(os.Stdin)

	for {
		clearScreen()
		printBanner(cfg)
		printMenu()

		choice := readChoice(reader)

		switch choice {
		case 1:
			runBackup(b, reader, false)
		case 2:
			runBackup(b, reader, true)
		case 3:
			runRestore(b, reader)
		case 4:
			viewHistory(b, reader)
		case 5:
			runCleanup(retention, reader)
		case 6:
			fmt.Println(colorGreen + "\nThank you for using TTS Lifeboat!" + colorReset)
			fmt.Println(colorCyan + "\"In case of sinking Tomcat, grab the Lifeboat!\"" + colorReset)
			return nil
		default:
			fmt.Println(colorRed + "\nInvalid choice. Press Enter to continue..." + colorReset)
			reader.ReadString('\n')
		}
	}
}

func clearScreen() {
	// ANSI escape code to clear screen and move cursor to top
	fmt.Print("\033[2J\033[H")
}

func printBanner(cfg *config.Config) {
	fmt.Println(colorGreen + colorBold)
	fmt.Println("  ╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ║   ████████╗████████╗███████╗    ██╗     ██╗███████╗███████╗  ║")
	fmt.Println("  ║   ╚══██╔══╝╚══██╔══╝██╔════╝    ██║     ██║██╔════╝██╔════╝  ║")
	fmt.Println("  ║      ██║      ██║   ███████╗    ██║     ██║█████╗  █████╗    ║")
	fmt.Println("  ║      ██║      ██║   ╚════██║    ██║     ██║██╔══╝  ██╔══╝    ║")
	fmt.Println("  ║      ╚═╝      ╚═╝   ╚══════╝    ╚══════╝╚═╝╚═╝     ╚══════╝  ║")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ║               LIFEBOAT - Enterprise Backup                   ║")
	fmt.Printf("  ║                      v%-10s                            ║\n", app.Version)
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════════╝")
	fmt.Println(colorReset)

	// Instance info
	fmt.Printf("  %sInstance:%s %s\n", colorCyan, colorReset, cfg.Name)
	fmt.Printf("  %sEnvironment:%s %s\n", colorCyan, colorReset, cfg.Environment)
	fmt.Println()

	// Show backup stats
	stats, _ := backup.NewRetentionManager(cfg).GetBackupStats()
	if stats != nil && stats.TotalBackups > 0 {
		lastBackup := "never"
		if stats.NewestBackup != nil {
			lastBackup = stats.NewestBackup.Date.Format("2006-01-02 15:04")
		}
		fmt.Printf("  %sBackups:%s %d  |  %sLast:%s %s\n",
			colorCyan, colorReset, stats.TotalBackups,
			colorCyan, colorReset, lastBackup)
		fmt.Println()
	}

	fmt.Printf("  %sCreated by %s%s\n", colorYellow, app.Creator, colorReset)
	fmt.Println()
}

func printMenu() {
	fmt.Println(colorGreen + "  ╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("  ║                         MAIN MENU                            ║")
	fmt.Println("  ╠══════════════════════════════════════════════════════════════╣")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ║   [1]  Create New Backup                                     ║")
	fmt.Println("  ║   [2]  Create Checkpoint Backup (Protected)                  ║")
	fmt.Println("  ║   [3]  Restore from Backup                                   ║")
	fmt.Println("  ║   [4]  View Backup History                                   ║")
	fmt.Println("  ║   [5]  Cleanup Old Backups                                   ║")
	fmt.Println("  ║   [6]  Exit                                                  ║")
	fmt.Println("  ║                                                              ║")
	fmt.Println("  ╚══════════════════════════════════════════════════════════════╝" + colorReset)
	fmt.Println()
	fmt.Print(colorCyan + "  Enter your choice (1-6): " + colorReset)
}

func readChoice(reader *bufio.Reader) int {
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	// Easter egg check
	if strings.ToLower(input) == "kannan" {
		fmt.Println(app.GetEasterEgg())
		fmt.Print("\nPress Enter to continue...")
		reader.ReadString('\n')
		return 0
	}

	choice, err := strconv.Atoi(input)
	if err != nil {
		return 0
	}
	return choice
}

func runBackup(b *backup.Backup, reader *bufio.Reader, checkpoint bool) {
	clearScreen()
	backupType := "Standard"
	if checkpoint {
		backupType = "Checkpoint"
	}

	fmt.Printf("\n%s  === %s Backup ===%s\n\n", colorGreen+colorBold, backupType, colorReset)

	// Ask for optional note
	fmt.Print(colorCyan + "  Enter backup note (optional, press Enter to skip): " + colorReset)
	note, _ := reader.ReadString('\n')
	note = strings.TrimSpace(note)

	fmt.Println()
	fmt.Println(colorYellow + "  Starting backup..." + colorReset)
	fmt.Println()

	opts := backup.BackupOptions{
		Note: note,
	}

	startTime := time.Now()
	result, err := b.Run(opts, func(phase string, current, total int, message string) {
		if total > 0 {
			pct := float64(current) / float64(total) * 100
			fmt.Printf("\r  [%s] %s: %.0f%% (%d/%d) - %s",
				phase, colorCyan, pct, current, total, truncate(message, 40)+colorReset)
		} else {
			fmt.Printf("\r  [%s] %s%s", phase, message, strings.Repeat(" ", 30))
		}
	})

	fmt.Println()
	fmt.Println()

	if err != nil {
		fmt.Printf("%s  ERROR: %s%s\n", colorRed, err.Error(), colorReset)
	} else {
		// Mark as checkpoint if requested
		if checkpoint && result != nil {
			b.MarkCheckpoint(result.ID, note)
		}

		fmt.Println(colorGreen + "  ╔══════════════════════════════════════════════════════════════╗")
		fmt.Println("  ║                    BACKUP COMPLETE!                          ║")
		fmt.Println("  ╚══════════════════════════════════════════════════════════════╝" + colorReset)
		fmt.Println()
		fmt.Printf("  %sBackup ID:%s     %s\n", colorCyan, colorReset, result.ID)
		fmt.Printf("  %sFiles:%s         %d\n", colorCyan, colorReset, result.FilesProcessed)
		fmt.Printf("  %sOriginal:%s      %s\n", colorCyan, colorReset, backup.FormatSize(result.OriginalSize))
		fmt.Printf("  %sCompressed:%s    %s\n", colorCyan, colorReset, backup.FormatSize(result.CompressedSize))
		fmt.Printf("  %sDuration:%s      %s\n", colorCyan, colorReset, time.Since(startTime).Round(time.Millisecond))
		if checkpoint {
			fmt.Printf("  %sCheckpoint:%s    Yes (Protected from auto-cleanup)\n", colorCyan, colorReset)
		}
	}

	fmt.Println()
	fmt.Print(colorYellow + "  Press Enter to continue..." + colorReset)
	reader.ReadString('\n')
}

func runRestore(b *backup.Backup, reader *bufio.Reader) {
	clearScreen()
	fmt.Printf("\n%s  === Restore Backup ===%s\n\n", colorGreen+colorBold, colorReset)

	// List available backups
	backups, err := b.List()
	if err != nil {
		fmt.Printf("%s  ERROR: %s%s\n", colorRed, err.Error(), colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if len(backups) == 0 {
		fmt.Println(colorYellow + "  No backups available." + colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	// Show backups
	fmt.Println("  Available backups:")
	fmt.Println()
	for i, bk := range backups {
		status := ""
		if bk.Checkpoint {
			status = " [CHECKPOINT]"
		}
		fmt.Printf("  %s[%d]%s %s  %s  %s%s\n",
			colorCyan, i+1, colorReset,
			bk.ID,
			bk.Date.Format("2006-01-02 15:04"),
			bk.Size,
			status)
	}

	fmt.Println()
	fmt.Print(colorCyan + "  Enter backup number to restore (0 to cancel): " + colorReset)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 0 || choice > len(backups) {
		fmt.Println(colorRed + "  Invalid selection." + colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if choice == 0 {
		return
	}

	selectedBackup := backups[choice-1]

	fmt.Println()
	fmt.Printf(colorYellow+"  WARNING: This will restore backup %s%s\n", selectedBackup.ID, colorReset)
	fmt.Print(colorCyan + "  Enter target path (leave empty for original location): " + colorReset)
	targetPath, _ := reader.ReadString('\n')
	targetPath = strings.TrimSpace(targetPath)

	fmt.Println()
	fmt.Printf(colorRed+"  Are you sure you want to restore? (yes/no): %s", colorReset)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "yes" {
		fmt.Println(colorYellow + "  Restore cancelled." + colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	fmt.Println()
	fmt.Println(colorYellow + "  Restoring backup..." + colorReset)

	err = b.Restore(selectedBackup.ID, targetPath, func(phase string, current, total int, message string) {
		if total > 0 {
			pct := float64(current) / float64(total) * 100
			fmt.Printf("\r  [%s] %.0f%% (%d/%d) - %s",
				phase, pct, current, total, truncate(message, 40))
		}
	})
	if err != nil {
		fmt.Printf("\n%s  ERROR: %s%s\n", colorRed, err.Error(), colorReset)
	} else {
		fmt.Printf("\n%s  Restore completed successfully!%s\n", colorGreen, colorReset)
	}

	fmt.Print("\n  Press Enter to continue...")
	reader.ReadString('\n')
}

func viewHistory(b *backup.Backup, reader *bufio.Reader) {
	clearScreen()
	fmt.Printf("\n%s  === Backup History ===%s\n\n", colorGreen+colorBold, colorReset)

	backups, err := b.List()
	if err != nil {
		fmt.Printf("%s  ERROR: %s%s\n", colorRed, err.Error(), colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if len(backups) == 0 {
		fmt.Println(colorYellow + "  No backups found." + colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	fmt.Printf("  %s%-30s %-18s %-12s %s%s\n",
		colorCyan, "Backup ID", "Date", "Size", "Status", colorReset)
	fmt.Println("  " + strings.Repeat("-", 75))

	for _, bk := range backups {
		status := ""
		if bk.Checkpoint {
			status = colorGreen + "[CHECKPOINT]" + colorReset
		}
		fmt.Printf("  %-30s %-18s %-12s %s\n",
			bk.ID,
			bk.Date.Format("2006-01-02 15:04"),
			bk.Size,
			status)
		if bk.Note != "" {
			fmt.Printf("    %sNote: %s%s\n", colorYellow, bk.Note, colorReset)
		}
	}

	fmt.Println()
	fmt.Print(colorYellow + "  Press Enter to continue..." + colorReset)
	reader.ReadString('\n')
}

func runCleanup(retention *backup.RetentionManager, reader *bufio.Reader) {
	clearScreen()
	fmt.Printf("\n%s  === Cleanup Old Backups ===%s\n\n", colorGreen+colorBold, colorReset)

	// First do a dry run
	fmt.Println(colorYellow + "  Analyzing backups..." + colorReset)
	result, err := retention.Cleanup(true) // dry run
	if err != nil {
		fmt.Printf("%s  ERROR: %s%s\n", colorRed, err.Error(), colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if result.BackupsDeleted == 0 {
		fmt.Println(colorGreen + "\n  No backups need to be cleaned up." + colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	fmt.Printf("\n  %sBackups to remove:%s %d\n", colorCyan, colorReset, result.BackupsDeleted)
	fmt.Printf("  %sSpace to free:%s    %s\n", colorCyan, colorReset, backup.FormatSize(result.SpaceFreed))

	fmt.Println()
	fmt.Printf(colorRed + "  Are you sure you want to delete these backups? (yes/no): " + colorReset)
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "yes" {
		fmt.Println(colorYellow + "  Cleanup cancelled." + colorReset)
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	// Actually run cleanup
	result, err = retention.Cleanup(false)
	if err != nil {
		fmt.Printf("%s  ERROR: %s%s\n", colorRed, err.Error(), colorReset)
	} else {
		fmt.Printf("\n%s  Cleanup complete! Freed %s%s\n",
			colorGreen, backup.FormatSize(result.SpaceFreed), colorReset)
	}

	fmt.Print("\n  Press Enter to continue...")
	reader.ReadString('\n')
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
