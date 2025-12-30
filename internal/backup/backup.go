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
	Note             string
	Checkpoint       bool
	DryRun           bool
	SelectedWebapps  []string // Selected webapps to backup (empty = all)
	SelectedCustom   []string // Selected custom folders to backup (empty = all)
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
	compressor *StreamingCompressor
}

// New creates a new backup instance.
func New(cfg *config.Config) *Backup {
	return &Backup{
		config:     cfg,
		collector:  NewCollector(cfg),
		compressor: NewStreamingCompressor(cfg),
	}
}

// ProgressCallback is called during backup to report progress.
type ProgressCallback func(phase string, current, total int, message string)

// GetAvailableWebapps returns list of webapps available for backup.
func (b *Backup) GetAvailableWebapps() ([]WebappInfo, error) {
	webappsPath := config.NormalizePath(b.config.WebappsPath)

	if webappsPath == "" {
		return nil, fmt.Errorf("webapps_path not configured in lifeboat.yaml")
	}

	entries, err := os.ReadDir(webappsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read webapps directory: %w", err)
	}

	var webapps []WebappInfo
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) == ".war" {
			info, err := entry.Info()
			if err != nil {
				continue
			}

			webapp := WebappInfo{
				Name:  entry.Name(),
				Path:  filepath.Join(webappsPath, entry.Name()),
				IsWAR: filepath.Ext(entry.Name()) == ".war",
			}

			if entry.IsDir() {
				// Calculate folder size
				webapp.Size = calculateFolderSize(webapp.Path)
			} else {
				webapp.Size = info.Size()
			}

			webapps = append(webapps, webapp)
		}
	}

	return webapps, nil
}

// WebappInfo contains information about a webapp.
type WebappInfo struct {
	Name  string
	Path  string
	Size  int64
	IsWAR bool
}

// calculateFolderSize recursively calculates folder size.
func calculateFolderSize(path string) int64 {
	var size int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size
}

// GetAvailableCustomFolders returns list of configured custom folders.
func (b *Backup) GetAvailableCustomFolders() []CustomFolderInfo {
	var folders []CustomFolderInfo

	for _, folder := range b.config.CustomFolders {
		info := CustomFolderInfo{
			Title:    folder.Title,
			Path:     config.NormalizePath(folder.Path),
			Required: folder.Required,
			Exists:   false,
		}

		if stat, err := os.Stat(info.Path); err == nil {
			info.Exists = true
			if stat.IsDir() {
				info.Size = calculateFolderSize(info.Path)
			} else {
				info.Size = stat.Size()
			}
		}

		folders = append(folders, info)
	}

	return folders
}

// CustomFolderInfo contains information about a custom folder.
type CustomFolderInfo struct {
	Title    string
	Path     string
	Size     int64
	Required bool
	Exists   bool
}

// IsCompressorAvailable checks if the compressor is ready.
// For modern builds, this always returns true (pure Go).
// For legacy builds, this checks if 7-Zip is available.
func (b *Backup) IsCompressorAvailable() bool {
	return b.compressor.IsAvailable()
}

// GetCompressionFormat returns the format used by the compressor.
func (b *Backup) GetCompressionFormat() string {
	return b.compressor.GetFormat()
}

// IsSevenZipAvailable checks if 7-Zip is available (backward compat).
func (b *Backup) IsSevenZipAvailable() bool {
	return b.compressor.IsAvailable()
}

// Run executes a backup with the given options.
func (b *Backup) Run(opts BackupOptions, progress ProgressCallback) (*BackupResult, error) {
	result := &BackupResult{
		ID:        GenerateBackupID(),
		StartTime: time.Now(),
		Errors:    []string{},
	}

	logger.Info("starting backup", "id", result.ID)

	// Validate compressor availability
	if !b.compressor.IsAvailable() {
		return nil, fmt.Errorf("compressor not available. For legacy builds, install 7-Zip")
	}

	// Get archive extension from compressor
	archiveExt := "." + b.compressor.GetFormat()

	// Phase 1: Create backup directory structure
	if progress != nil {
		progress("init", 0, 0, "Creating backup directory...")
	}

	dateFolder := GetDateFolder()
	timeFolder := GetTimeFolder()

	var backupPath string
	if opts.Checkpoint {
		safeName := sanitizeFolderName(opts.Note)
		if safeName == "" {
			safeName = "checkpoint"
		}
		backupPath = filepath.Join(b.config.GetBackupPath(), fmt.Sprintf("%s_%s", dateFolder, safeName))
	} else {
		backupPath = filepath.Join(b.config.GetBackupPath(), dateFolder, timeFolder)
	}

	result.Path = backupPath

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Determine which webapps to backup
	webappsToBackup := opts.SelectedWebapps
	if len(webappsToBackup) == 0 {
		// No selection = backup all (either from config or all available)
		if len(b.config.Webapps) > 0 {
			webappsToBackup = b.config.Webapps
		} else {
			// Get all available webapps
			available, err := b.GetAvailableWebapps()
			if err != nil {
				return nil, fmt.Errorf("failed to get available webapps: %w", err)
			}
			for _, w := range available {
				webappsToBackup = append(webappsToBackup, w.Name)
			}
		}
	}

	if opts.DryRun {
		logger.Info("dry run - would backup webapps", "webapps", webappsToBackup)
		result.Success = true
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
		return result, nil
	}

	// Phase 2: Copy and compress webapps
	webappsPath := config.NormalizePath(b.config.WebappsPath)

	for i, webappName := range webappsToBackup {
		webappSrc := filepath.Join(webappsPath, webappName)

		if _, err := os.Stat(webappSrc); os.IsNotExist(err) {
			result.Errors = append(result.Errors, fmt.Sprintf("webapp not found: %s", webappName))
			continue
		}

		if progress != nil {
			progress("copy", i+1, len(webappsToBackup), fmt.Sprintf("Copying %s...", webappName))
		}

		logger.Info("processing webapp", "name", webappName, "source", webappSrc)

		// Archive path (extension based on compressor format)
		archivePath := filepath.Join(backupPath, sanitizeFolderName(webappName)+archiveExt)

		// Streaming compression (pure Go for modern, 7-Zip for legacy)
		compResult, err := b.compressor.CompressFolder(
			webappSrc,
			archivePath,
			func(current int, filename string) {
				if progress != nil {
					progress("compress", current, 0, filename)
				}
			},
		)

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", webappName, err))
			logger.Error("failed to backup webapp", "name", webappName, "error", err)
			continue
		}

		result.FilesProcessed += compResult.FilesProcessed
		result.OriginalSize += compResult.OriginalSize
		result.CompressedSize += compResult.CompressedSize
		result.Errors = append(result.Errors, compResult.Errors...)
	}

	// Phase 3: Backup custom folders
	customFolders := b.GetAvailableCustomFolders()
	selectedCustom := opts.SelectedCustom

	for i, folder := range customFolders {
		// Skip if not selected (when selection is provided)
		if len(selectedCustom) > 0 {
			selected := false
			for _, s := range selectedCustom {
				if s == folder.Title {
					selected = true
					break
				}
			}
			if !selected {
				continue
			}
		}

		if !folder.Exists {
			if folder.Required {
				result.Errors = append(result.Errors, fmt.Sprintf("required folder not found: %s", folder.Title))
			}
			continue
		}

		if progress != nil {
			progress("custom", i+1, len(customFolders), fmt.Sprintf("Backing up %s...", folder.Title))
		}

		logger.Info("processing custom folder", "title", folder.Title, "path", folder.Path)

		archivePath := filepath.Join(backupPath, sanitizeFolderName(folder.Title)+archiveExt)

		compResult, err := b.compressor.CompressFolder(
			folder.Path,
			archivePath,
			func(current int, filename string) {
				if progress != nil {
					progress("compress", current, 0, filename)
				}
			},
		)

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", folder.Title, err))
			logger.Error("failed to backup custom folder", "title", folder.Title, "error", err)
			continue
		}

		result.FilesProcessed += compResult.FilesProcessed
		result.OriginalSize += compResult.OriginalSize
		result.CompressedSize += compResult.CompressedSize
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

	relPath, _ := filepath.Rel(b.config.GetBackupPath(), backupPath)

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
	if !b.compressor.IsAvailable() {
		return fmt.Errorf("compressor not available")
	}

	index, err := LoadIndex(b.config.GetIndexPath())
	if err != nil {
		return err
	}

	entry := index.GetByID(backupID)
	if entry == nil {
		return fmt.Errorf("backup not found: %s", backupID)
	}

	backupPath := filepath.Join(b.config.GetBackupPath(), entry.Path)

	// Create target directory
	if err := os.MkdirAll(targetPath, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	// Find all archives in backup directory (supports multiple formats)
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	// Supported archive extensions
	archiveExts := map[string]bool{
		".tar.zst": true,
		".tar.gz":  true,
		".tgz":     true,
		".7z":      true,
		".zip":     true,
	}

	for _, e := range entries {
		name := e.Name()
		ext := filepath.Ext(name)

		// Check for .tar.zst or .tar.gz (double extension)
		if ext == ".zst" || ext == ".gz" {
			base := name[:len(name)-len(ext)]
			if filepath.Ext(base) == ".tar" {
				ext = filepath.Ext(base) + ext
			}
		}

		if !archiveExts[ext] {
			continue
		}

		archivePath := filepath.Join(backupPath, name)

		if progress != nil {
			progress("extract", 0, 0, fmt.Sprintf("Extracting %s...", name))
		}

		if err := b.compressor.Extract(archivePath, targetPath, func(msg string) {
			if progress != nil {
				progress("extract", 0, 0, msg)
			}
		}); err != nil {
			return fmt.Errorf("failed to extract %s: %w", name, err)
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
