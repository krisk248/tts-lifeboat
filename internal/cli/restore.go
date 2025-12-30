package cli

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/kannan/tts-lifeboat/internal/backup"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [backup-id|latest]",
	Short: "Restore a backup",
	Long: `Restore a backup to a target directory.

Use 'latest' to restore the most recent backup, or specify a backup ID.

Examples:
  lifeboat restore latest
  lifeboat restore latest --target ./restored
  lifeboat restore backup-20251230-110432
  lifeboat restore backup-20251230-110432 --target /path/to/restore`,
	Args: cobra.ExactArgs(1),
	RunE: runRestore,
}

var restoreTarget string

func init() {
	restoreCmd.Flags().StringVar(&restoreTarget, "target", "", "target directory for restore (default: ./rollback)")
}

func runRestore(cmd *cobra.Command, args []string) error {
	backupID := args[0]

	// Create backup instance
	b := backup.New(cfg)

	// Check 7-Zip availability
	if !b.IsSevenZipAvailable() {
		return fmt.Errorf("7-Zip not found. Please install 7-Zip from https://www.7-zip.org/ or configure seven_zip.path in lifeboat.yaml")
	}

	// Handle "latest" keyword
	if backupID == "latest" {
		latest, err := b.GetLatest()
		if err != nil {
			return fmt.Errorf("failed to get latest backup: %w", err)
		}
		if latest == nil {
			return fmt.Errorf("no backups found")
		}
		backupID = latest.ID
		fmt.Printf("📌 Latest backup: %s\n", backupID)
	}

	// Set default target
	if restoreTarget == "" {
		restoreTarget = filepath.Join(cfg.BackupPath, "rollback")
	}

	fmt.Printf("🚢 TTS Lifeboat - Restore\n")
	fmt.Printf("   Backup:  %s\n", backupID)
	fmt.Printf("   Target:  %s\n", restoreTarget)
	fmt.Println()

	// Progress callback
	progress := func(phase string, current, total int, message string) {
		switch phase {
		case "extract":
			fmt.Printf("\r📦 Extracting: %s", truncateString(message, 50))
		}
	}

	// Run restore
	if err := b.Restore(backupID, restoreTarget, progress); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Println()
	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  RESTORE COMPLETE")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Backup:    %s\n", backupID)
	fmt.Printf("  Restored:  %s\n", restoreTarget)
	fmt.Println()
	fmt.Println("  ⚠️  Important: Review restored files before replacing")
	fmt.Println("     production files!")
	fmt.Println("═══════════════════════════════════════════════════════════")

	return nil
}
