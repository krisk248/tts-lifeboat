package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

// BackupOptions configures backup behavior.
type BackupOptions struct {
	Note       string
	Checkpoint bool
	DryRun     bool
}

// BackupResult holds the result of a backup operation.
type BackupResult struct {
	ID             string
	Path           string
	StartTime      time.Time
	EndTime        time.Time
	Duration       time.Duration
	FilesCollected int
	FilesProcessed int
	OriginalSize   int64
	CompressedSize int64
	Errors         []string
	Success        bool
}

// Backup orchestrates the backup process.
type Backup struct {
	config     *config.Config
	collector  *Collector
	compressor *Compressor
}

// New creates a new backup instance.
func New(cfg *config.Config) *Backup {
	return &Backup{
		config:     cfg,
		collector:  NewCollector(cfg),
		compressor: NewCompressor(cfg),
	}
}

// ProgressCallback is called during backup to report progress.
type ProgressCallback func(phase string, current, total int, message string)

// Run executes a backup with the given options.
func (b *Backup) Run(opts BackupOptions, progress ProgressCallback) (*BackupResult, error) {
	result := &BackupResult{
		ID:        GenerateBackupID(),
		StartTime: time.Now(),
		Errors:    []string{},
	}

	// Phase 1: Collect files
	if progress != nil {
		progress("collect", 0, 0, "Scanning files...")
	}
	logger.Info("starting backup", "id", result.ID)

	collection := b.collector.Collect()
	result.FilesCollected = collection.TotalCount
	result.OriginalSize = collection.TotalSize
	result.Errors = append(result.Errors, collection.Errors...)

	logger.Info("files collected",
		"count", collection.TotalCount,
		"size", FormatSize(collection.TotalSize))

	if opts.DryRun {
		result.Success = true
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	// Phase 2: Create backup directory
	dateFolder := GetDateFolder()
	timeFolder := GetTimeFolder()

	var backupPath string
	if opts.Checkpoint {
		// Checkpoint format: YYYYMMDD_note
		safeName := sanitizeFolderName(opts.Note)
		if safeName == "" {
			safeName = "checkpoint"
		}
		backupPath = filepath.Join(b.config.BackupPath, fmt.Sprintf("%s_%s", dateFolder, safeName))
	} else {
		backupPath = filepath.Join(b.config.BackupPath, dateFolder, timeFolder)
	}

	result.Path = backupPath

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Phase 3: Create archives
	if progress != nil {
		progress("compress", 0, 0, "Creating archives...")
	}

	// Group files by category
	webappFiles := collection.GetFilesByCategory("webapp")
	customFiles := collection.GetFilesByCategory("custom")

	// Create webapp archive
	if len(webappFiles) > 0 {
		webappArchive := filepath.Join(backupPath, "webapp.tar.gz")
		compResult, err := b.compressor.CreateArchive(webappFiles, webappArchive, func(current, total int, filename string) {
			if progress != nil {
				progress("compress", current, total, filename)
			}
		})
		if err != nil {
			result.Errors = append(result.Errors, "webapp archive error: "+err.Error())
			logger.Error("failed to create webapp archive", "error", err)
		} else {
			result.FilesProcessed += compResult.FilesProcessed
			result.CompressedSize += compResult.CompressedSize
			result.Errors = append(result.Errors, compResult.Errors...)
		}
	}

	// Create custom folders archive (if any)
	if len(customFiles) > 0 {
		customArchive := filepath.Join(backupPath, "custom.tar.gz")
		compResult, err := b.compressor.CreateArchive(customFiles, customArchive, func(current, total int, filename string) {
			if progress != nil {
				progress("compress", current, total, filename)
			}
		})
		if err != nil {
			result.Errors = append(result.Errors, "custom archive error: "+err.Error())
			logger.Error("failed to create custom archive", "error", err)
		} else {
			result.FilesProcessed += compResult.FilesProcessed
			result.CompressedSize += compResult.CompressedSize
			result.Errors = append(result.Errors, compResult.Errors...)
		}
	}

	// Phase 4: Save metadata
	if progress != nil {
		progress("metadata", 0, 0, "Saving metadata...")
	}

	result.EndTime = time.Now()
	result.Duration = result.EndTime.Sub(result.StartTime)

	meta := &Metadata{
		ID:              result.ID,
		CreatedAt:       result.StartTime,
		DurationSeconds: int(result.Duration.Seconds()),
		Files: FileStats{
			Count:          result.FilesProcessed,
			OriginalSize:   FormatSize(result.OriginalSize),
			CompressedSize: FormatSize(result.CompressedSize),
		},
		Note: opts.Note,
	}

	metadataPath := filepath.Join(backupPath, "metadata.json")
	if err := SaveMetadata(metadataPath, meta); err != nil {
		result.Errors = append(result.Errors, "metadata error: "+err.Error())
		logger.Error("failed to save metadata", "error", err)
	}

	// Phase 5: Update index
	index, err := LoadIndex(b.config.GetIndexPath())
	if err != nil {
		logger.Warn("failed to load index, creating new", "error", err)
		index = &Index{Backups: []IndexEntry{}}
	}

	// Calculate relative path from backup root
	relPath, _ := filepath.Rel(b.config.BackupPath, backupPath)

	entry := IndexEntry{
		ID:         result.ID,
		Date:       result.StartTime,
		Path:       relPath,
		Size:       FormatSize(result.CompressedSize),
		Checkpoint: opts.Checkpoint,
		Note:       opts.Note,
	}

	if !opts.Checkpoint && b.config.Retention.Enabled && b.config.Retention.Days > 0 {
		deleteDate := result.StartTime.AddDate(0, 0, b.config.Retention.Days)
		entry.DeleteAfter = deleteDate.Format("2006-01-02")
	}

	index.AddEntry(entry)

	if err := SaveIndex(b.config.GetIndexPath(), index); err != nil {
		result.Errors = append(result.Errors, "index error: "+err.Error())
		logger.Error("failed to save index", "error", err)
	}

	result.Success = len(result.Errors) == 0

	logger.Info("backup completed",
		"id", result.ID,
		"path", result.Path,
		"duration", result.Duration,
		"files", result.FilesProcessed,
		"size", FormatSize(result.CompressedSize))

	return result, nil
}

// List returns all backups from the index.
func (b *Backup) List() ([]IndexEntry, error) {
	index, err := LoadIndex(b.config.GetIndexPath())
	if err != nil {
		return nil, err
	}
	return index.Backups, nil
}

// GetLatest returns the most recent backup.
func (b *Backup) GetLatest() (*IndexEntry, error) {
	index, err := LoadIndex(b.config.GetIndexPath())
	if err != nil {
		return nil, err
	}
	return index.GetLatest(), nil
}

// Restore extracts a backup to the target directory.
func (b *Backup) Restore(backupID, targetPath string, progress ProgressCallback) error {
	index, err := LoadIndex(b.config.GetIndexPath())
	if err != nil {
		return err
	}

	entry := index.GetByID(backupID)
	if entry == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	backupPath := filepath.Join(b.config.BackupPath, entry.Path)

	// Create target directory
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Extract webapp archive
	webappArchive := filepath.Join(backupPath, "webapp.tar.gz")
	if _, err := os.Stat(webappArchive); err == nil {
		if progress != nil {
			progress("extract", 0, 0, "Extracting webapp archive...")
		}
		if err := b.compressor.ExtractArchive(webappArchive, targetPath, func(current int, filename string) {
			if progress != nil {
				progress("extract", current, 0, filename)
			}
		}); err != nil {
			return fmt.Errorf("failed to extract webapp archive: %w", err)
		}
	}

	// Extract custom archive
	customArchive := filepath.Join(backupPath, "custom.tar.gz")
	if _, err := os.Stat(customArchive); err == nil {
		if progress != nil {
			progress("extract", 0, 0, "Extracting custom archive...")
		}
		if err := b.compressor.ExtractArchive(customArchive, targetPath, func(current int, filename string) {
			if progress != nil {
				progress("extract", current, 0, filename)
			}
		}); err != nil {
			return fmt.Errorf("failed to extract custom archive: %w", err)
		}
	}

	logger.Info("restore completed", "backup", backupID, "target", targetPath)
	return nil
}

// MarkCheckpoint marks a backup as a checkpoint.
func (b *Backup) MarkCheckpoint(backupID, note string) error {
	index, err := LoadIndex(b.config.GetIndexPath())
	if err != nil {
		return err
	}

	if !index.MarkAsCheckpoint(backupID, note) {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	return SaveIndex(b.config.GetIndexPath(), index)
}
