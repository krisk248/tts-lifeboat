package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/kannan/tts-lifeboat/internal/backup"
)

var checkpointCmd = &cobra.Command{
	Use:   "checkpoint [backup-id]",
	Short: "Mark a backup as checkpoint",
	Long: `Mark an existing backup as a checkpoint so it never auto-deletes.

Checkpoints are special backups that are preserved regardless of
the retention policy. Use them for important milestones like releases.

Examples:
  lifeboat checkpoint backup-20251230-110432
  lifeboat checkpoint backup-20251230-110432 --note "Release v2.0"
  lifeboat checkpoint latest --note "Pre-migration backup"`,
	Args: cobra.ExactArgs(1),
	RunE: runCheckpoint,
}

var checkpointNote string

func init() {
	checkpointCmd.Flags().StringVar(&checkpointNote, "note", "", "add a note to the checkpoint")
}

func runCheckpoint(cmd *cobra.Command, args []string) error {
	backupID := args[0]

	b := backup.New(cfg)

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

	// Mark as checkpoint
	if err := b.MarkCheckpoint(backupID, checkpointNote); err != nil {
		return fmt.Errorf("failed to mark checkpoint: %w", err)
	}

	fmt.Println()
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Println("  ⭐ CHECKPOINT CREATED")
	fmt.Println("═══════════════════════════════════════════════════════════")
	fmt.Printf("  Backup: %s\n", backupID)
	if checkpointNote != "" {
		fmt.Printf("  Note:   %s\n", checkpointNote)
	}
	fmt.Println()
	fmt.Println("  This backup will never be auto-deleted by retention policy.")
	fmt.Println("═══════════════════════════════════════════════════════════")

	return nil
}
