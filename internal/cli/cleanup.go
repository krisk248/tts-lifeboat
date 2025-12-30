package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kannan/tts-lifeboat/internal/backup"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove expired backups",
	Long: `Remove backups that have exceeded their retention period.

The cleanup respects the min_keep setting to ensure a minimum number
of backups are always retained. Checkpoint backups are never deleted.

Examples:
  lifeboat cleanup             # Preview what would be deleted
  lifeboat cleanup --dry-run   # Same as above
  lifeboat cleanup --force     # Actually delete expired backups`,
	RunE: runCleanup,
}

var (
	cleanupDryRun bool
	cleanupForce  bool
)

func init() {
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", true, "preview deletions without removing files")
	cleanupCmd.Flags().BoolVar(&cleanupForce, "force", false, "actually delete expired backups")
}

func runCleanup(cmd *cobra.Command, args []string) error {
	// Create retention manager
	rm := backup.NewRetentionManager(cfg)

	// If --force is specified, disable dry-run
	dryRun := cleanupDryRun
	if cleanupForce {
		dryRun = false
	}

	fmt.Println("🚢 TTS Lifeboat - Cleanup")
	fmt.Printf("   Instance: %s\n", cfg.Name)
	fmt.Println()

	if dryRun {
		fmt.Println("🔍 DRY RUN - No files will be deleted")
		fmt.Println()
	}

	// Get stats first
	stats, err := rm.GetBackupStats()
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}

	fmt.Println("📊 Current Status:")
	fmt.Printf("   Total backups:     %d\n", stats.TotalBackups)
	fmt.Printf("   Regular backups:   %d\n", stats.RegularBackups)
	fmt.Printf("   Checkpoints:       %d (protected)\n", stats.CheckpointBackups)
	fmt.Printf("   Expired:           %d\n", stats.ExpiredBackups)
	fmt.Printf("   Total size:        %s\n", backup.FormatSize(stats.TotalSize))
	fmt.Println()

	// Run cleanup
	result, err := rm.Cleanup(dryRun)
	if err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	// Print results
	fmt.Println("═══════════════════════════════════════════════════════════")
	if dryRun {
		fmt.Println("  CLEANUP PREVIEW")
	} else {
		fmt.Println("  CLEANUP COMPLETE")
	}
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Backups %s:   %d\n", map[bool]string{true: "to delete", false: "deleted"}[dryRun], result.BackupsDeleted)
	fmt.Printf("  Space %s:     %s\n", map[bool]string{true: "to free", false: "freed"}[dryRun], backup.FormatSize(result.SpaceFreed))
	fmt.Printf("  Backups kept:       %d\n", result.BackupsKept)

	if len(result.Errors) > 0 {
		fmt.Println()
		fmt.Println("  ⚠️  Errors:")
		for _, e := range result.Errors {
			fmt.Printf("     - %s\n", e)
		}
	}

	fmt.Println("═══════════════════════════════════════════════════════════")

	if dryRun && result.BackupsDeleted > 0 {
		fmt.Println()
		fmt.Println("💡 To actually delete these backups, run:")
		fmt.Println("   lifeboat cleanup --force")
	}

	return nil
}
