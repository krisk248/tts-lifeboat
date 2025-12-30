// Package backup provides the core backup engine for tts-lifeboat.
package backup

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

// SevenZip handles compression using external 7-Zip.
type SevenZip struct {
	config     *config.Config
	exePath    string
	bufferSize int
}

// SevenZipResult holds the result of a 7-Zip compression.
type SevenZipResult struct {
	OriginalSize   int64
	CompressedSize int64
	FilesProcessed int
	ArchivePath    string
	Errors         []string
}

// NewSevenZip creates a new 7-Zip compressor.
func NewSevenZip(cfg *config.Config) *SevenZip {
	return &SevenZip{
		config:     cfg,
		exePath:    findSevenZip(cfg),
		bufferSize: 32 * 1024 * 1024, // 32MB buffer for copying
	}
}

// findSevenZip locates the 7-Zip executable.
func findSevenZip(cfg *config.Config) string {
	// Check config first
	if cfg.SevenZip.Path != "" {
		path := config.NormalizePath(cfg.SevenZip.Path)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Check common locations
	var paths []string

	if runtime.GOOS == "windows" {
		paths = []string{
			`C:\Program Files\7-Zip\7z.exe`,
			`C:\Program Files (x86)\7-Zip\7z.exe`,
			`7z.exe`, // In PATH
		}
	} else {
		paths = []string{
			"/usr/bin/7z",
			"/usr/local/bin/7z",
			"7z", // In PATH
		}
	}

	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			return path
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// IsAvailable checks if 7-Zip is available on the system.
func (s *SevenZip) IsAvailable() bool {
	return s.exePath != ""
}

// GetPath returns the path to 7-Zip executable.
func (s *SevenZip) GetPath() string {
	return s.exePath
}

// CopyFolderToTemp copies a folder to a temp directory.
// This is phase 1 of copy-then-compress strategy.
func (s *SevenZip) CopyFolderToTemp(srcPath, tempPath string, progress func(current int, filename string)) (int64, int, error) {
	var totalSize int64
	var fileCount int

	// Create temp directory
	if err := os.MkdirAll(tempPath, 0755); err != nil {
		return 0, 0, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Walk source and copy
	err := filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("error accessing path during copy", "path", path, "error", err)
			return nil // Continue with other files
		}

		// Get relative path from source
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return nil
		}

		// Target path in temp
		targetPath := filepath.Join(tempPath, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// Copy file
		if progress != nil {
			progress(fileCount+1, relPath)
		}

		if err := s.copyFile(path, targetPath); err != nil {
			logger.Warn("failed to copy file", "file", path, "error", err)
			return nil // Continue with other files
		}

		totalSize += info.Size()
		fileCount++
		return nil
	})

	if err != nil {
		return totalSize, fileCount, fmt.Errorf("copy failed: %w", err)
	}

	return totalSize, fileCount, nil
}

// copyFile copies a single file.
func (s *SevenZip) copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Get source file info for permissions
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Create destination directory if needed
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	// Copy with buffer
	buf := make([]byte, s.bufferSize)
	if _, err := io.CopyBuffer(dstFile, srcFile, buf); err != nil {
		return err
	}

	// Preserve permissions
	return os.Chmod(dst, srcInfo.Mode())
}

// CompressFolder compresses a folder using 7-Zip.
// This is phase 2 of copy-then-compress strategy.
func (s *SevenZip) CompressFolder(srcPath, archivePath string, progress func(message string)) error {
	if !s.IsAvailable() {
		return fmt.Errorf("7-Zip not found. Please install 7-Zip and add to PATH or configure seven_zip.path in lifeboat.yaml")
	}

	// Determine compression level
	level := s.config.SevenZip.Level
	if level <= 0 || level > 9 {
		level = 5 // Default balanced
	}

	// Build 7z command
	// 7z a -mx5 -mmt1 archive.7z folder/
	args := []string{
		"a",                                  // Add to archive
		fmt.Sprintf("-mx%d", level),          // Compression level
		fmt.Sprintf("-mmt%d", s.config.SevenZip.Threads), // Thread count
		"-y",                                 // Assume yes on all queries
		archivePath,                          // Output archive
		srcPath + string(os.PathSeparator) + "*", // Source folder contents
	}

	if progress != nil {
		progress("Compressing with 7-Zip...")
	}

	logger.Info("running 7-Zip", "exe", s.exePath, "args", strings.Join(args, " "))

	cmd := exec.Command(s.exePath, args...)
	cmd.Dir = filepath.Dir(srcPath) // Run from parent directory

	// Capture output for logging
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("7-Zip failed", "error", err, "output", string(output))
		return fmt.Errorf("7-Zip compression failed: %w\nOutput: %s", err, string(output))
	}

	logger.Info("7-Zip completed successfully")
	return nil
}

// CompressFiles compresses specific files using 7-Zip.
func (s *SevenZip) CompressFiles(files []string, archivePath string, baseDir string) error {
	if !s.IsAvailable() {
		return fmt.Errorf("7-Zip not found")
	}

	level := s.config.SevenZip.Level
	if level <= 0 || level > 9 {
		level = 5
	}

	// Build argument list
	args := []string{
		"a",
		fmt.Sprintf("-mx%d", level),
		fmt.Sprintf("-mmt%d", s.config.SevenZip.Threads),
		"-y",
		archivePath,
	}
	args = append(args, files...)

	cmd := exec.Command(s.exePath, args...)
	cmd.Dir = baseDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("7-Zip failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// ExtractArchive extracts a 7z archive to destination.
func (s *SevenZip) ExtractArchive(archivePath, destPath string, progress func(message string)) error {
	if !s.IsAvailable() {
		return fmt.Errorf("7-Zip not found")
	}

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// 7z x archive.7z -odestination -y
	args := []string{
		"x",                                // Extract with full paths
		archivePath,                        // Archive file
		fmt.Sprintf("-o%s", destPath),      // Output directory
		"-y",                               // Assume yes
	}

	if progress != nil {
		progress("Extracting archive...")
	}

	cmd := exec.Command(s.exePath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extraction failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// RemoveFolder removes a folder and all its contents.
func (s *SevenZip) RemoveFolder(path string) error {
	return os.RemoveAll(path)
}

// CopyThenCompress performs the full copy-then-compress workflow.
// 1. Copy source to temp folder
// 2. Compress temp folder with 7-Zip
// 3. Delete temp folder
func (s *SevenZip) CopyThenCompress(srcPath, archivePath, tempPath string, copyProgress func(current int, filename string), compressProgress func(message string)) (*SevenZipResult, error) {
	result := &SevenZipResult{
		Errors: []string{},
	}

	// Phase 1: Copy to temp
	totalSize, fileCount, err := s.CopyFolderToTemp(srcPath, tempPath, copyProgress)
	if err != nil {
		return nil, fmt.Errorf("copy phase failed: %w", err)
	}
	result.OriginalSize = totalSize
	result.FilesProcessed = fileCount

	// Phase 2: Compress temp folder
	if err := s.CompressFolder(tempPath, archivePath, compressProgress); err != nil {
		// Clean up temp folder even on error
		s.RemoveFolder(tempPath)
		return nil, fmt.Errorf("compress phase failed: %w", err)
	}

	// Get compressed size
	if stat, err := os.Stat(archivePath); err == nil {
		result.CompressedSize = stat.Size()
	}
	result.ArchivePath = archivePath

	// Phase 3: Delete temp folder
	if err := s.RemoveFolder(tempPath); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("failed to clean temp folder: %v", err))
		logger.Warn("failed to remove temp folder", "path", tempPath, "error", err)
	}

	return result, nil
}
