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
		console.Clear()
		printBanner(cfg, b)
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

func printBanner(cfg *config.Config, b *backup.Backup) {
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

	// 7-Zip status
	if b.IsSevenZipAvailable() {
		fmt.Printf("  7-Zip:       Found (%s)\n", b.GetSevenZipPath())
	} else {
		fmt.Println("  7-Zip:       NOT FOUND - Please install 7-Zip!")
	}
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
	console.Clear()
	backupType := "Standard"
	if checkpoint {
		backupType = "Checkpoint"
	}

	fmt.Println()
	fmt.Printf("  === %s Backup ===\n", backupType)
	fmt.Println()

	// Check 7-Zip availability
	if !b.IsSevenZipAvailable() {
		fmt.Println("  ERROR: 7-Zip not found!")
		fmt.Println()
		fmt.Println("  Please install 7-Zip from https://www.7-zip.org/")
		fmt.Println("  Or configure the path in lifeboat.yaml under seven_zip.path")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	// Get available webapps
	webapps, err := b.GetAvailableWebapps()
	if err != nil {
		fmt.Printf("  ERROR: %s\n", err.Error())
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	if len(webapps) == 0 {
		fmt.Println("  No webapps found in configured path.")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	// Show folder selection
	selectedWebapps := selectWebapps(webapps, reader)
	if len(selectedWebapps) == 0 {
		fmt.Println()
		fmt.Println("  No webapps selected. Backup cancelled.")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

	// Ask for optional note
	console.Clear()
	fmt.Println()
	fmt.Printf("  === %s Backup ===\n", backupType)
	fmt.Println()
	fmt.Printf("  Selected webapps: %d\n", len(selectedWebapps))
	for _, name := range selectedWebapps {
		fmt.Printf("    - %s\n", name)
	}
	fmt.Println()
	fmt.Print("  Enter backup note (optional, press Enter to skip): ")
	note, _ := reader.ReadString('\n')
	note = strings.TrimSpace(note)

	fmt.Println()
	fmt.Println("  Starting backup...")
	fmt.Println()

	opts := backup.BackupOptions{
		Note:            note,
		Checkpoint:      checkpoint,
		SelectedWebapps: selectedWebapps,
	}

	startTime := time.Now()
	result, err := b.Run(opts, func(phase string, current, total int, message string) {
		if total > 0 {
			pct := float64(current) / float64(total) * 100
			fmt.Printf("\r  [%s] %.0f%% (%d/%d) - %s          ",
				phase, pct, current, total, truncate(message, 30))
		} else {
			fmt.Printf("\r  [%s] %s                              ", phase, truncate(message, 40))
		}
	})

	fmt.Println()
	fmt.Println()

	if err != nil {
		fmt.Printf("  ERROR: %s\n", err.Error())
	} else {
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

		if len(result.Errors) > 0 {
			fmt.Println()
			fmt.Println("  Warnings/Errors:")
			for _, e := range result.Errors {
				fmt.Printf("    - %s\n", e)
			}
		}
	}

	fmt.Println()
	fmt.Print("  Press Enter to continue...")
	reader.ReadString('\n')
}

// selectWebapps shows folder selection screen and returns selected webapp names.
func selectWebapps(webapps []backup.WebappInfo, reader *bufio.Reader) []string {
	// Track selection state (all selected by default)
	selected := make([]bool, len(webapps))
	for i := range selected {
		selected[i] = true
	}

	for {
		console.Clear()
		fmt.Println()
		fmt.Println("  === Select Webapps to Backup ===")
		fmt.Println()
		fmt.Println("  Enter number to toggle, 'a' for all, 'n' for none, 'c' to continue:")
		fmt.Println()

		// Calculate total size
		var totalSize int64
		var selectedCount int

		// Display webapps with selection status
		for i, w := range webapps {
			marker := "[ ]"
			if selected[i] {
				marker = "[X]"
				totalSize += w.Size
				selectedCount++
			}

			typeStr := "folder"
			if w.IsWAR {
				typeStr = "WAR"
			}

			fmt.Printf("  %s [%d] %-30s (%s, %s)\n",
				marker,
				i+1,
				truncate(w.Name, 30),
				typeStr,
				backup.FormatSize(w.Size))
		}

		fmt.Println()
		fmt.Printf("  Selected: %d webapps, Total size: %s\n", selectedCount, backup.FormatSize(totalSize))
		fmt.Println()
		fmt.Print("  Enter choice: ")

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))

		switch input {
		case "c", "continue", "":
			// Return selected webapp names
			var result []string
			for i, w := range webapps {
				if selected[i] {
					result = append(result, w.Name)
				}
			}
			return result

		case "a", "all":
			// Select all
			for i := range selected {
				selected[i] = true
			}

		case "n", "none":
			// Deselect all
			for i := range selected {
				selected[i] = false
			}

		case "q", "quit", "cancel":
			// Cancel - return empty
			return nil

		default:
			// Try to parse as number
			num, err := strconv.Atoi(input)
			if err == nil && num >= 1 && num <= len(webapps) {
				// Toggle selection
				selected[num-1] = !selected[num-1]
			}
		}
	}
}

func runRestore(b *backup.Backup, reader *bufio.Reader) {
	console.Clear()
	fmt.Println()
	fmt.Println("  === Restore Backup ===")
	fmt.Println()

	// Check 7-Zip availability
	if !b.IsSevenZipAvailable() {
		fmt.Println("  ERROR: 7-Zip not found!")
		fmt.Println()
		fmt.Println("  Please install 7-Zip from https://www.7-zip.org/")
		fmt.Print("\n  Press Enter to continue...")
		reader.ReadString('\n')
		return
	}

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
		fmt.Printf("\r  [%s] %s                              ", phase, truncate(message, 40))
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
	console.Clear()
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
	console.Clear()
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
