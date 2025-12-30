//go:build legacy

// Package backup provides streaming compression for legacy builds.
// Uses external 7-Zip since klauspost/compress requires Go 1.23+.
package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

// StreamingCompressor handles compression for legacy builds using 7-Zip.
type StreamingCompressor struct {
	config     *config.Config
	sevenZip   *SevenZip
	bufferSize int
}

// StreamingResult holds the result of a streaming compression.
type StreamingResult struct {
	OriginalSize   int64
	CompressedSize int64
	FilesProcessed int
	ArchivePath    string
	Format         string
	Errors         []string
}

// NewStreamingCompressor creates a new streaming compressor for legacy build.
func NewStreamingCompressor(cfg *config.Config) *StreamingCompressor {
	return &StreamingCompressor{
		config:     cfg,
		sevenZip:   NewSevenZip(cfg),
		bufferSize: 64 * 1024, // 64KB buffer
	}
}

// IsAvailable returns true if 7-Zip is available.
func (s *StreamingCompressor) IsAvailable() bool {
	return s.sevenZip.IsAvailable()
}

// GetFormat returns the compression format.
func (s *StreamingCompressor) GetFormat() string {
	if s.sevenZip.IsAvailable() {
		return "7z"
	}
	return "zip"
}

// CompressFolder compresses a folder using 7-Zip (legacy) or zip fallback.
func (s *StreamingCompressor) CompressFolder(srcPath, archivePath string, progress func(current int, filename string)) (*StreamingResult, error) {
	// Try 7-Zip first
	if s.sevenZip.IsAvailable() {
		return s.compressWithSevenZip(srcPath, archivePath, progress)
	}

	// Fallback to zip
	return s.CompressFolderToZip(srcPath, archivePath, progress)
}

// compressWithSevenZip uses external 7-Zip for compression.
func (s *StreamingCompressor) compressWithSevenZip(srcPath, archivePath string, progress func(current int, filename string)) (*StreamingResult, error) {
	result := &StreamingResult{
		Format: "7z",
		Errors: []string{},
	}

	// Ensure .7z extension
	if !strings.HasSuffix(archivePath, ".7z") {
		archivePath = strings.TrimSuffix(archivePath, filepath.Ext(archivePath)) + ".7z"
	}
	result.ArchivePath = archivePath

	// Create temp folder for copy-then-compress
	tempPath := archivePath + ".tmp"

	// Copy to temp (handles locked files)
	copyProgress := func(current int, filename string) {
		if progress != nil {
			progress(current, filename)
		}
	}

	totalSize, fileCount, err := s.sevenZip.CopyFolderToTemp(srcPath, tempPath, copyProgress)
	if err != nil {
		return nil, fmt.Errorf("copy phase failed: %w", err)
	}
	result.OriginalSize = totalSize
	result.FilesProcessed = fileCount

	// Compress temp folder
	compressProgress := func(message string) {
		logger.Info("compress", "status", message)
	}

	if err := s.sevenZip.CompressFolder(tempPath, archivePath, compressProgress); err != nil {
		s.sevenZip.RemoveFolder(tempPath)
		return nil, fmt.Errorf("compress phase failed: %w", err)
	}

	// Get compressed size
	if stat, err := os.Stat(archivePath); err == nil {
		result.CompressedSize = stat.Size()
	}

	// Cleanup temp
	if err := s.sevenZip.RemoveFolder(tempPath); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("temp cleanup failed: %v", err))
	}

	return result, nil
}

// CompressFolderToZip compresses a folder to .zip archive (fallback).
func (s *StreamingCompressor) CompressFolderToZip(srcPath, archivePath string, progress func(current int, filename string)) (*StreamingResult, error) {
	result := &StreamingResult{
		Format: "zip",
		Errors: []string{},
	}

	// Ensure .zip extension
	if !strings.HasSuffix(archivePath, ".zip") {
		archivePath = strings.TrimSuffix(archivePath, filepath.Ext(archivePath)) + ".zip"
	}
	result.ArchivePath = archivePath

	// Create output file
	outFile, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}
	defer outFile.Close()

	// Create zip writer
	zipWriter := zip.NewWriter(outFile)
	defer zipWriter.Close()

	// Walk and add files
	fileCount := 0
	err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("error accessing path", "path", path, "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("access error: %s", path))
			return nil
		}

		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return nil
		}

		if relPath == "." {
			return nil
		}

		fileCount++
		if progress != nil {
			progress(fileCount, relPath)
		}

		if info.IsDir() {
			return nil
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("header error: %s", relPath))
			return nil
		}
		header.Name = filepath.ToSlash(relPath)
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("create error: %s", relPath))
			return nil
		}

		srcFile, err := os.Open(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("open error: %s", relPath))
			return nil
		}
		defer srcFile.Close()

		buf := make([]byte, s.bufferSize)
		written, err := io.CopyBuffer(writer, srcFile, buf)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("copy error: %s", relPath))
			return nil
		}

		result.OriginalSize += written
		result.FilesProcessed++
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk failed: %w", err)
	}

	zipWriter.Close()
	outFile.Close()

	if stat, err := os.Stat(archivePath); err == nil {
		result.CompressedSize = stat.Size()
	}

	return result, nil
}

// Extract extracts an archive (auto-detects format).
func (s *StreamingCompressor) Extract(archivePath, destPath string, progress func(message string)) error {
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	// Use 7-Zip for .7z files
	if strings.HasSuffix(archivePath, ".7z") && s.sevenZip.IsAvailable() {
		return s.sevenZip.ExtractArchive(archivePath, destPath, progress)
	}

	// Use zip for .zip files
	if strings.HasSuffix(archivePath, ".zip") {
		return s.extractZip(archivePath, destPath, progress)
	}

	// Use tar.gz for .tar.gz files
	if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		c := NewCompressor(s.config)
		return c.ExtractArchive(archivePath, destPath, func(current int, filename string) {
			if progress != nil {
				progress(filename)
			}
		})
	}

	// Try 7-Zip for other formats
	if s.sevenZip.IsAvailable() {
		return s.sevenZip.ExtractArchive(archivePath, destPath, progress)
	}

	return fmt.Errorf("unsupported archive format: %s", archivePath)
}

// extractZip extracts a .zip archive.
func (s *StreamingCompressor) extractZip(archivePath, destPath string, progress func(message string)) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open zip: %w", err)
	}
	defer reader.Close()

	for _, file := range reader.File {
		if progress != nil {
			progress(file.Name)
		}

		target := filepath.Join(destPath, file.Name)

		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		srcFile, err := file.Open()
		if err != nil {
			return err
		}

		dstFile, err := os.Create(target)
		if err != nil {
			srcFile.Close()
			return err
		}

		buf := make([]byte, s.bufferSize)
		_, err = io.CopyBuffer(dstFile, srcFile, buf)
		srcFile.Close()
		dstFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}
