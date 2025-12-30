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
	config    *config.Config
	collector *Collector
	sevenZip  *SevenZip
}

// New creates a new backup instance.
func New(cfg *config.Config) *Backup {
	return &Backup{
		config:    cfg,
		collector: NewCollector(cfg),
		sevenZip:  NewSevenZip(cfg),
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

// IsSevenZipAvailable checks if 7-Zip is available.
func (b *Backup) IsSevenZipAvailable() bool {
	return b.sevenZip.IsAvailable()
}

// GetSevenZipPath returns the 7-Zip path.
func (b *Backup) GetSevenZipPath() string {
	return b.sevenZip.GetPath()
}

// Run executes a backup with the given options.
func (b *Backup) Run(opts BackupOptions, progress ProgressCallback) (*BackupResult, error) {
	result := &BackupResult{
		ID:        GenerateBackupID(),
		StartTime: time.Now(),
		Errors:    []string{},
	}

	logger.Info("starting backup", "id", result.ID)

	// Validate 7-Zip availability
	if !b.sevenZip.IsAvailable() {
		return nil, fmt.Errorf("7-Zip not found. Please install 7-Zip or configure seven_zip.path in lifeboat.yaml")
	}

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

	// Temp directory for copy-then-compress
	tempPath := filepath.Join(backupPath, "temp")

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

		// Archive path
		archivePath := filepath.Join(backupPath, fmt.Sprintf("%s.7z", sanitizeFolderName(webappName)))
		webappTemp := filepath.Join(tempPath, webappName)

		// Copy-then-compress
		szResult, err := b.sevenZip.CopyThenCompress(
			webappSrc,
			archivePath,
			webappTemp,
			func(current int, filename string) {
				if progress != nil {
					progress("copy", current, 0, filename)
				}
			},
			func(message string) {
				if progress != nil {
					progress("compress", i+1, len(webappsToBackup), message)
				}
			},
		)

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", webappName, err))
			logger.Error("failed to backup webapp", "name", webappName, "error", err)
			continue
		}

		result.FilesProcessed += szResult.FilesProcessed
		result.OriginalSize += szResult.OriginalSize
		result.CompressedSize += szResult.CompressedSize
		result.Errors = append(result.Errors, szResult.Errors...)
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

		archivePath := filepath.Join(backupPath, fmt.Sprintf("%s.7z", sanitizeFolderName(folder.Title)))
		folderTemp := filepath.Join(tempPath, sanitizeFolderName(folder.Title))

		szResult, err := b.sevenZip.CopyThenCompress(
			folder.Path,
			archivePath,
			folderTemp,
			func(current int, filename string) {
				if progress != nil {
					progress("copy", current, 0, filename)
				}
			},
			func(message string) {
				if progress != nil {
					progress("compress", i+1, len(customFolders), message)
				}
			},
		)

		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", folder.Title, err))
			logger.Error("failed to backup custom folder", "title", folder.Title, "error", err)
			continue
		}

		result.FilesProcessed += szResult.FilesProcessed
		result.OriginalSize += szResult.OriginalSize
		result.CompressedSize += szResult.CompressedSize
	}

	// Clean up temp directory if it exists
	if _, err := os.Stat(tempPath); err == nil {
		os.RemoveAll(tempPath)
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
	if !b.sevenZip.IsAvailable() {
		return fmt.Errorf("7-Zip not found")
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

	// Find all .7z archives in backup directory
	entries, err := os.ReadDir(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %w", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".7z" {
			archivePath := filepath.Join(backupPath, e.Name())

			if progress != nil {
				progress("extract", 0, 0, fmt.Sprintf("Extracting %s...", e.Name()))
			}

			if err := b.sevenZip.ExtractArchive(archivePath, targetPath, func(msg string) {
				if progress != nil {
					progress("extract", 0, 0, msg)
				}
			}); err != nil {
				return fmt.Errorf("failed to extract %s: %w", e.Name(), err)
			}
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
