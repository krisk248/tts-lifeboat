package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/kannan/tts-lifeboat/internal/backup"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all backups",
	Long: `List all available backups with their details.

Examples:
  lifeboat list
  lifeboat list --limit 5
  lifeboat list --json
  lifeboat list --checkpoints`,
	RunE: runList,
}

var (
	listLimit       int
	listJSON        bool
	listCheckpoints bool
)

func init() {
	listCmd.Flags().IntVar(&listLimit, "limit", 0, "limit number of backups shown (0 = all)")
	listCmd.Flags().BoolVar(&listJSON, "json", false, "output in JSON format")
	listCmd.Flags().BoolVar(&listCheckpoints, "checkpoints", false, "show only checkpoint backups")
}

func runList(cmd *cobra.Command, args []string) error {
	b := backup.New(cfg)

	backups, err := b.List()
	if err != nil {
		return fmt.Errorf("failed to list backups: %w", err)
	}

	if len(backups) == 0 {
		fmt.Println("No backups found.")
		return nil
	}

	// Filter checkpoints if requested
	if listCheckpoints {
		filtered := []backup.IndexEntry{}
		for _, bk := range backups {
			if bk.Checkpoint {
				filtered = append(filtered, bk)
			}
		}
		backups = filtered

		if len(backups) == 0 {
			fmt.Println("No checkpoint backups found.")
			return nil
		}
	}

	// Apply limit
	if listLimit > 0 && len(backups) > listLimit {
		backups = backups[:listLimit]
	}

	// JSON output
	if listJSON {
		output := struct {
			Backups []backup.IndexEntry `json:"backups"`
			Total   int                 `json:"total"`
		}{
			Backups: backups,
			Total:   len(backups),
		}
		data, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// Table output
	fmt.Println("🚢 TTS Lifeboat - Backup History")
	fmt.Printf("   Instance: %s\n", cfg.Name)
	fmt.Println()

	fmt.Println("╭──────────────────────────────────────────────────────────────────────────────╮")
	fmt.Println("│  ID                      │  Date        │  Size     │  Status              │")
	fmt.Println("├──────────────────────────────────────────────────────────────────────────────┤")

	for _, bk := range backups {
		dateStr := bk.Date.Format("2006-01-02 15:04")

		// Determine status
		var status string
		if bk.Checkpoint {
			status = "⭐ CHECKPOINT"
		} else if bk.DeleteAfter != "" {
			deleteDate, _ := time.Parse("2006-01-02", bk.DeleteAfter)
			daysLeft := int(time.Until(deleteDate).Hours() / 24)
			if daysLeft < 0 {
				status = "🗑️  EXPIRED"
			} else if daysLeft <= 7 {
				status = fmt.Sprintf("⚠️  %dd left", daysLeft)
			} else {
				status = fmt.Sprintf("✓  %dd left", daysLeft)
			}
		} else {
			status = "✓"
		}

		// Pad and truncate
		id := padRight(truncateString(bk.ID, 24), 24)
		date := padRight(dateStr, 12)
		size := padRight(bk.Size, 9)
		statusPad := padRight(status, 20)

		fmt.Printf("│  %s│  %s│  %s│  %s│\n", id, date, size, statusPad)

		// Show note if present
		if bk.Note != "" {
			noteLine := fmt.Sprintf("   📝 %s", truncateString(bk.Note, 60))
			fmt.Printf("│  %-74s│\n", noteLine)
		}
	}

	fmt.Println("╰──────────────────────────────────────────────────────────────────────────────╯")
	fmt.Printf("\nTotal: %d backups\n", len(backups))

	return nil
}

func padRight(s string, length int) string {
	if len(s) >= length {
		return s[:length]
	}
	return s + strings.Repeat(" ", length-len(s))
}
