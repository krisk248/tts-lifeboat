package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kannan/tts-lifeboat/internal/backup"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Create a new backup",
	Long: `Create a new backup of configured webapps and custom folders.

Examples:
  lifeboat backup                    # Interactive backup
  lifeboat backup --all              # Backup everything non-interactively
  lifeboat backup --all --note "Pre-deployment"
  lifeboat backup --checkpoint --note "Release v2.0"
  lifeboat backup --dry-run          # Preview what would be backed up`,
	RunE: runBackup,
}

var (
	backupAll        bool
	backupNote       string
	backupCheckpoint bool
	backupDryRun     bool
)

func init() {
	backupCmd.Flags().BoolVar(&backupAll, "all", false, "backup all files without interactive selection")
	backupCmd.Flags().StringVar(&backupNote, "note", "", "add a note to this backup")
	backupCmd.Flags().BoolVar(&backupCheckpoint, "checkpoint", false, "mark as checkpoint (never auto-delete)")
	backupCmd.Flags().BoolVar(&backupDryRun, "dry-run", false, "preview backup without creating files")
}

func runBackup(cmd *cobra.Command, args []string) error {
	// Validate config
	if result := cfg.Validate(); !result.Valid {
		return fmt.Errorf("configuration invalid:\n%s", result.String())
	}

	// Create backup instance
	b := backup.New(cfg)

	opts := backup.BackupOptions{
		Note:       backupNote,
		Checkpoint: backupCheckpoint,
		DryRun:     backupDryRun,
	}

	if backupDryRun {
		fmt.Println("🔍 DRY RUN - No files will be created")
		fmt.Println()
	}

	// Progress callback for CLI
	progress := func(phase string, current, total int, message string) {
		switch phase {
		case "collect":
			fmt.Printf("📂 Collecting files: %s\n", message)
		case "compress":
			if total > 0 {
				pct := float64(current) / float64(total) * 100
				fmt.Printf("\r💾 Processing: [%3.0f%%] %s", pct, truncateString(message, 50))
			}
		case "metadata":
			fmt.Printf("\n📝 %s\n", message)
		}
	}

	fmt.Printf("🚢 TTS Lifeboat - Starting backup\n")
	fmt.Printf("   Instance: %s (%s)\n", cfg.Name, cfg.Environment)
	fmt.Println()

	// Run backup
	result, err := b.Run(opts, progress)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// Print summary
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	if backupDryRun {
		fmt.Println("  DRY RUN SUMMARY")
	} else {
		fmt.Println("  BACKUP COMPLETE")
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  ID:         %s\n", result.ID)
	if !backupDryRun {
		fmt.Printf("  Location:   %s\n", result.Path)
	}
	fmt.Printf("  Files:      %d\n", result.FilesCollected)
	fmt.Printf("  Size:       %s → %s\n",
		backup.FormatSize(result.OriginalSize),
		backup.FormatSize(result.CompressedSize))
	fmt.Printf("  Duration:   %s\n", result.Duration.Round(100000000))

	if backupCheckpoint {
		fmt.Println("  Type:       ⭐ CHECKPOINT (never auto-deletes)")
	}

	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("  ⚠️  Warnings:")
		for _, e := range result.Errors {
			fmt.Printf("     - %s\n", e)
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════")

	if !result.Success {
		return fmt.Errorf("backup completed with errors")
	}

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
