// Package interactive provides a simple text-based interactive CLI for legacy systems.
// This package is used when building with the "legacy" build tag for systems
// that don't support Go 1.24+ (like Windows 2008 R2).
// NOTE: No ANSI colors - Windows 2008 R2 cmd.exe doesn't support them.
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
	"github.com/kannan/tts-lifeboat/internal/console"
)

// Run starts the interactive CLI application.
func Run() error {
	// Set Windows console title
	console.SetTitle(fmt.Sprintf("TTS Lifeboat v%s - Enterprise Backup", app.Version))

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
			fmt.Println()
			fmt.Println("  Thank you for using TTS Lifeboat!")
			fmt.Println("  \"In case of sinking Tomcat, grab the Lifeboat!\"")
			fmt.Println()
			return nil
		default:
			fmt.Println()
			fmt.Println("  Invalid choice. Press Enter to continue...")
			reader.ReadString('\n')
		}
	}
}

func clearScreen() {
	// Clear screen using ANSI (works on most terminals)
	// For Windows 2008, this may not work but won't show garbage
	fmt.Print("\033[2J\033[H")
}

func printBanner(cfg *config.Config) {
	fmt.Println()
	fmt.Println("  +============================================================+")
	fmt.Println("  |                                                            |")
	fmt.Println("  |   TTS LIFEBOAT - Enterprise Backup Solution                |")
	fmt.Printf("  |   Version: %-47s |\n", app.Version)
	fmt.Println("  |                                                            |")
	fmt.Println("  +============================================================+")
	fmt.Println()

	// Instance info
	fmt.Printf("  Instance:    %s\n", cfg.Name)
	fmt.Printf("  Environment: %s\n", cfg.Environment)
	fmt.Println()

	// Show backup stats
	stats, _ := backup.NewRetentionManager(cfg).GetBackupStats()
	if stats != nil && stats.TotalBackups > 0 {
		lastBackup := "never"
		if stats.NewestBackup != nil {
			lastBackup = stats.NewestBackup.Date.Format("2006-01-02 15:04")
		}
		fmt.Printf("  Backups: %d  |  Last: %s\n", stats.TotalBackups, lastBackup)
		fmt.Println()
	}

	fmt.Printf("  Created by %s\n", app.Creator)
	fmt.Println()
}

func printMenu() {
	fmt.Println("  +------------------------------------------------------------+")
	fmt.Println("  |                       MAIN MENU                            |")
	fmt.Println("  +------------------------------------------------------------+")
	fmt.Println("  |                                                            |")
	fmt.Println("  |   [1]  Create New Backup                                   |")
	fmt.Println("  |   [2]  Create Checkpoint Backup (Protected)                |")
	fmt.Println("  |   [3]  Restore from Backup                                 |")
	fmt.Println("  |   [4]  View Backup History                                 |")
	fmt.Println("  |   [5]  Cleanup Old Backups                                 |")
	fmt.Println("  |   [6]  Exit                                                |")
	fmt.Println("  |                                                            |")
	fmt.Println("  +------------------------------------------------------------+")
	fmt.Println()
	fmt.Print("  Enter your choice (1-6): ")
}

func readChoice(reader *bufio.Reader) int {
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	// Easter egg check
	if strings.ToLower(input) == "kannan" {
		fmt.Println(app.GetEasterEgg())
		fmt.Print("\n  Press Enter to continue...")
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

	fmt.Println()
	fmt.Printf("  === %s Backup ===\n", backupType)
	fmt.Println()

	// Ask for optional note
	fmt.Print("  Enter backup note (optional, press Enter to skip): ")
	note, _ := reader.ReadString('\n')
	note = strings.TrimSpace(note)

	fmt.Println()
	fmt.Println("  Starting backup...")
	fmt.Println()

	opts := backup.BackupOptions{
		Note: note,
	}

	startTime := time.Now()
	result, err := b.Run(opts, func(phase string, current, total int, message string) {
		if total > 0 {
			pct := float64(current) / float64(total) * 100
			fmt.Printf("\r  [%s] %.0f%% (%d/%d) - %s          ",
				phase, pct, current, total, truncate(message, 30))
		} else {
			fmt.Printf("\r  [%s] %s                              ", phase, message)
		}
	})

	fmt.Println()
	fmt.Println()

	if err != nil {
		fmt.Printf("  ERROR: %s\n", err.Error())
	} else {
		// Mark as checkpoint if requested
		if checkpoint && result != nil {
			b.MarkCheckpoint(result.ID, note)
		}

		fmt.Println("  +------------------------------------------------------------+")
		fmt.Println("  |                   BACKUP COMPLETE!                         |")
		fmt.Println("  +------------------------------------------------------------+")
		fmt.Println()
		fmt.Printf("  Backup ID:   %s\n", result.ID)
		fmt.Printf("  Files:       %d\n", result.FilesProcessed)
		fmt.Printf("  Original:    %s\n", backup.FormatSize(result.OriginalSize))
		fmt.Printf("  Compressed:  %s\n", backup.FormatSize(result.CompressedSize))
		fmt.Printf("  Duration:    %s\n", time.Since(startTime).Round(time.Millisecond))
		if checkpoint {
			fmt.Println("  Checkpoint:  Yes (Protected from auto-cleanup)")
		}
	}

	fmt.Println()
	fmt.Print("  Press Enter to continue...")
	reader.ReadString('\n')
}

func runRestore(b *backup.Backup, reader *bufio.Reader) {
	clearScreen()
	fmt.Println()
	fmt.Println("  === Restore Backup ===")
	fmt.Println()

	// List available backups
	backups, err := b.List()
	if err != nil {
		fmt.Printf("  ERROR: %s\n", err.Error())
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if len(backups) == 0 {
		fmt.Println("  No backups available.")
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
		fmt.Printf("  [%d] %s  %s  %s%s\n",
			i+1,
			bk.ID,
			bk.Date.Format("2006-01-02 15:04"),
			bk.Size,
			status)
	}

	fmt.Println()
	fmt.Print("  Enter backup number to restore (0 to cancel): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	choice, err := strconv.Atoi(input)
	if err != nil || choice < 0 || choice > len(backups) {
		fmt.Println("  Invalid selection.")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if choice == 0 {
		return
	}

	selectedBackup := backups[choice-1]

	fmt.Println()
	fmt.Printf("  WARNING: This will restore backup %s\n", selectedBackup.ID)
	fmt.Print("  Enter target path (leave empty for original location): ")
	targetPath, _ := reader.ReadString('\n')
	targetPath = strings.TrimSpace(targetPath)

	fmt.Println()
	fmt.Print("  Are you sure you want to restore? (yes/no): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "yes" {
		fmt.Println("  Restore cancelled.")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	fmt.Println()
	fmt.Println("  Restoring backup...")

	err = b.Restore(selectedBackup.ID, targetPath, func(phase string, current, total int, message string) {
		if total > 0 {
			pct := float64(current) / float64(total) * 100
			fmt.Printf("\r  [%s] %.0f%% (%d/%d) - %s          ",
				phase, pct, current, total, truncate(message, 30))
		}
	})
	if err != nil {
		fmt.Printf("\n  ERROR: %s\n", err.Error())
	} else {
		fmt.Printf("\n  Restore completed successfully!\n")
	}

	fmt.Print("\n  Press Enter to continue...")
	reader.ReadString('\n')
}

func viewHistory(b *backup.Backup, reader *bufio.Reader) {
	clearScreen()
	fmt.Println()
	fmt.Println("  === Backup History ===")
	fmt.Println()

	backups, err := b.List()
	if err != nil {
		fmt.Printf("  ERROR: %s\n", err.Error())
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if len(backups) == 0 {
		fmt.Println("  No backups found.")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	fmt.Printf("  %-28s %-18s %-12s %s\n", "Backup ID", "Date", "Size", "Status")
	fmt.Println("  " + strings.Repeat("-", 70))

	for _, bk := range backups {
		status := ""
		if bk.Checkpoint {
			status = "[CHECKPOINT]"
		}
		fmt.Printf("  %-28s %-18s %-12s %s\n",
			bk.ID,
			bk.Date.Format("2006-01-02 15:04"),
			bk.Size,
			status)
		if bk.Note != "" {
			fmt.Printf("    Note: %s\n", bk.Note)
		}
	}

	fmt.Println()
	fmt.Print("  Press Enter to continue...")
	reader.ReadString('\n')
}

func runCleanup(retention *backup.RetentionManager, reader *bufio.Reader) {
	clearScreen()
	fmt.Println()
	fmt.Println("  === Cleanup Old Backups ===")
	fmt.Println()

	// First do a dry run
	fmt.Println("  Analyzing backups...")
	result, err := retention.Cleanup(true) // dry run
	if err != nil {
		fmt.Printf("  ERROR: %s\n", err.Error())
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if result.BackupsDeleted == 0 {
		fmt.Println()
		fmt.Println("  No backups need to be cleaned up.")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	fmt.Printf("\n  Backups to remove: %d\n", result.BackupsDeleted)
	fmt.Printf("  Space to free:     %s\n", backup.FormatSize(result.SpaceFreed))

	fmt.Println()
	fmt.Print("  Are you sure you want to delete these backups? (yes/no): ")
	confirm, _ := reader.ReadString('\n')
	confirm = strings.TrimSpace(strings.ToLower(confirm))

	if confirm != "yes" {
		fmt.Println("  Cleanup cancelled.")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	// Actually run cleanup
	result, err = retention.Cleanup(false)
	if err != nil {
		fmt.Printf("  ERROR: %s\n", err.Error())
	} else {
		fmt.Printf("\n  Cleanup complete! Freed %s\n", backup.FormatSize(result.SpaceFreed))
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
