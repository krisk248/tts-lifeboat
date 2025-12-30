package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

// RetentionManager handles backup retention and cleanup.
type RetentionManager struct {
	config *config.Config
}

// CleanupResult holds the result of a cleanup operation.
type CleanupResult struct {
	BackupsDeleted int
	SpaceFreed     int64
	BackupsKept    int
	Errors         []string
}

// NewRetentionManager creates a new retention manager.
func NewRetentionManager(cfg *config.Config) *RetentionManager {
	return &RetentionManager{config: cfg}
}

// Cleanup removes expired backups according to retention policy.
func (r *RetentionManager) Cleanup(dryRun bool) (*CleanupResult, error) {
	result := &CleanupResult{
		Errors: []string{},
	}

	if !r.config.Retention.Enabled {
		logger.Info("retention policy disabled, skipping cleanup")
		return result, nil
	}

	index, err := LoadIndex(r.config.GetIndexPath())
	if err != nil {
		return nil, fmt.Errorf("failed to load index: %w", err)
	}

	// Get expired backups
	expired := index.GetExpired()

	// Sort all backups by date (newest first)
	sort.Slice(index.Backups, func(i, j int) bool {
		return index.Backups[i].Date.After(index.Backups[j].Date)
	})

	// Count non-checkpoint backups
	nonCheckpointCount := 0
	for _, b := range index.Backups {
		if !b.Checkpoint {
			nonCheckpointCount++
		}
	}

	// Determine which expired backups can be deleted (respecting min_keep)
	toDelete := []IndexEntry{}
	for _, entry := range expired {
		// Check if we would go below min_keep
		if nonCheckpointCount-len(toDelete) <= r.config.Retention.MinKeep {
			logger.Info("retaining backup to maintain min_keep",
				"backup", entry.ID,
				"min_keep", r.config.Retention.MinKeep)
			continue
		}
		toDelete = append(toDelete, entry)
	}

	// Delete backups
	for _, entry := range toDelete {
		backupPath := filepath.Join(r.config.BackupPath, entry.Path)

		// Calculate size before deletion
		size, _ := r.calculateDirSize(backupPath)

		if dryRun {
			logger.Info("would delete backup (dry run)",
				"backup", entry.ID,
				"path", backupPath,
				"size", FormatSize(size))
			result.BackupsDeleted++
			result.SpaceFreed += size
			continue
		}

		// Delete backup directory
		if err := os.RemoveAll(backupPath); err != nil {
			errMsg := fmt.Sprintf("failed to delete %s: %v", entry.ID, err)
			result.Errors = append(result.Errors, errMsg)
			logger.Error("failed to delete backup", "backup", entry.ID, "error", err)
			continue
		}

		// Remove from index
		index.RemoveEntry(entry.ID)

		result.BackupsDeleted++
		result.SpaceFreed += size

		logger.Info("deleted backup",
			"backup", entry.ID,
			"size", FormatSize(size))
	}

	// Clean up empty date directories
	if !dryRun {
		r.cleanEmptyDirs()
	}

	// Save updated index
	if !dryRun && result.BackupsDeleted > 0 {
		if err := SaveIndex(r.config.GetIndexPath(), index); err != nil {
			result.Errors = append(result.Errors, "failed to update index: "+err.Error())
		}
	}

	// Count remaining backups
	result.BackupsKept = len(index.Backups) - result.BackupsDeleted

	return result, nil
}

// GetExpiredBackups returns a list of backups that have exceeded their retention period.
func (r *RetentionManager) GetExpiredBackups() ([]IndexEntry, error) {
	index, err := LoadIndex(r.config.GetIndexPath())
	if err != nil {
		return nil, err
	}
	return index.GetExpired(), nil
}

// GetBackupStats returns statistics about all backups.
func (r *RetentionManager) GetBackupStats() (*BackupStats, error) {
	index, err := LoadIndex(r.config.GetIndexPath())
	if err != nil {
		return nil, err
	}

	stats := &BackupStats{
		TotalBackups:      len(index.Backups),
		CheckpointBackups: 0,
		RegularBackups:    0,
		ExpiredBackups:    0,
		TotalSize:         0,
	}

	expired := index.GetExpired()
	expiredMap := make(map[string]bool)
	for _, e := range expired {
		expiredMap[e.ID] = true
	}

	for _, backup := range index.Backups {
		if backup.Checkpoint {
			stats.CheckpointBackups++
		} else {
			stats.RegularBackups++
		}

		if expiredMap[backup.ID] {
			stats.ExpiredBackups++
		}

		// Parse size and add to total
		size, _ := ParseSize(backup.Size)
		stats.TotalSize += size
	}

	if stats.TotalBackups > 0 {
		stats.OldestBackup = &index.Backups[len(index.Backups)-1]
		stats.NewestBackup = &index.Backups[0]
	}

	return stats, nil
}

// BackupStats holds statistics about backups.
type BackupStats struct {
	TotalBackups      int
	CheckpointBackups int
	RegularBackups    int
	ExpiredBackups    int
	TotalSize         int64
	OldestBackup      *IndexEntry
	NewestBackup      *IndexEntry
}

// calculateDirSize returns the total size of a directory.
func (r *RetentionManager) calculateDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

// cleanEmptyDirs removes empty date directories.
func (r *RetentionManager) cleanEmptyDirs() {
	entries, err := os.ReadDir(r.config.BackupPath)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Skip special directories and files
		name := entry.Name()
		if name == "logs" || name == "index.json" {
			continue
		}

		dirPath := filepath.Join(r.config.BackupPath, name)

		// Check if directory is empty
		subEntries, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		if len(subEntries) == 0 {
			os.Remove(dirPath)
			logger.Debug("removed empty directory", "path", dirPath)
		}
	}
}

// ForceDelete deletes a specific backup regardless of retention policy.
func (r *RetentionManager) ForceDelete(backupID string) error {
	index, err := LoadIndex(r.config.GetIndexPath())
	if err != nil {
		return err
	}

	entry := index.GetByID(backupID)
	if entry == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	backupPath := filepath.Join(r.config.BackupPath, entry.Path)

	// Delete backup directory
	if err := os.RemoveAll(backupPath); err != nil {
		return fmt.Errorf("failed to delete backup: %w", err)
	}

	// Remove from index
	index.RemoveEntry(backupID)

	// Save updated index
	if err := SaveIndex(r.config.GetIndexPath(), index); err != nil {
		return fmt.Errorf("failed to update index: %w", err)
	}

	// Clean up empty dirs
	r.cleanEmptyDirs()

	logger.Info("force deleted backup", "backup", backupID)
	return nil
}

// ExtendRetention extends the delete_after date for a backup.
func (r *RetentionManager) ExtendRetention(backupID string, days int) error {
	index, err := LoadIndex(r.config.GetIndexPath())
	if err != nil {
		return err
	}

	for i := range index.Backups {
		if index.Backups[i].ID == backupID {
			if index.Backups[i].Checkpoint {
				return fmt.Errorf("checkpoint backups don't have expiration dates")
			}

			// Calculate new date
			var newDate time.Time
			if index.Backups[i].DeleteAfter != "" {
				currentDate, err := time.Parse("2006-01-02", index.Backups[i].DeleteAfter)
				if err != nil {
					newDate = time.Now().AddDate(0, 0, days)
				} else {
					newDate = currentDate.AddDate(0, 0, days)
				}
			} else {
				newDate = time.Now().AddDate(0, 0, days)
			}

			index.Backups[i].DeleteAfter = newDate.Format("2006-01-02")
			return SaveIndex(r.config.GetIndexPath(), index)
		}
	}

	return fmt.Errorf("backup not found: %s", backupID)
}
