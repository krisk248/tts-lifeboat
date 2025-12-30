//go:build !legacy

// Package backup provides streaming compression using klauspost/compress/zstd.
// This is the modern implementation requiring Go 1.23+.
package backup

import (
	"archive/tar"
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/klauspost/compress/zstd"

	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

// StreamingCompressor handles fast streaming compression with zstd.
type StreamingCompressor struct {
	config     *config.Config
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

// NewStreamingCompressor creates a new streaming compressor.
func NewStreamingCompressor(cfg *config.Config) *StreamingCompressor {
	return &StreamingCompressor{
		config:     cfg,
		bufferSize: 64 * 1024, // 64KB streaming buffer (low memory)
	}
}

// IsAvailable always returns true for pure Go implementation.
func (s *StreamingCompressor) IsAvailable() bool {
	return true
}

// GetFormat returns the compression format.
func (s *StreamingCompressor) GetFormat() string {
	return "tar.zst"
}

// CompressFolder compresses a folder to .tar.zst archive using streaming.
// This uses minimal memory by streaming files one by one.
func (s *StreamingCompressor) CompressFolder(srcPath, archivePath string, progress func(current int, filename string)) (*StreamingResult, error) {
	result := &StreamingResult{
		Format: "tar.zst",
		Errors: []string{},
	}

	// Ensure archive has correct extension
	if !strings.HasSuffix(archivePath, ".tar.zst") {
		archivePath = strings.TrimSuffix(archivePath, filepath.Ext(archivePath)) + ".tar.zst"
	}
	result.ArchivePath = archivePath

	// Create output file
	outFile, err := os.Create(archivePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}
	defer outFile.Close()

	// Create zstd encoder with configured level
	level := zstd.EncoderLevelFromZstd(s.config.Compression.Level)
	zstdWriter, err := zstd.NewWriter(outFile, zstd.WithEncoderLevel(level))
	if err != nil {
		return nil, fmt.Errorf("failed to create zstd writer: %w", err)
	}
	defer zstdWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(zstdWriter)
	defer tarWriter.Close()

	// Walk and add files
	fileCount := 0
	err = filepath.Walk(srcPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logger.Warn("error accessing path", "path", path, "error", err)
			result.Errors = append(result.Errors, fmt.Sprintf("access error: %s", path))
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return nil
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		fileCount++
		if progress != nil {
			progress(fileCount, relPath)
		}

		// Create tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("header error: %s", relPath))
			return nil
		}
		header.Name = filepath.ToSlash(relPath)

		if info.IsDir() {
			header.Name += "/"
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("write header error: %s", relPath))
			return nil
		}

		// If directory, we're done
		if info.IsDir() {
			return nil
		}

		// Stream file content
		srcFile, err := os.Open(path)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("open error: %s", relPath))
			return nil
		}
		defer srcFile.Close()

		// Streaming copy with small buffer
		buf := make([]byte, s.bufferSize)
		written, err := io.CopyBuffer(tarWriter, srcFile, buf)
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

	// Close writers to flush
	tarWriter.Close()
	zstdWriter.Close()
	outFile.Close()

	// Get compressed size
	stat, err := os.Stat(archivePath)
	if err == nil {
		result.CompressedSize = stat.Size()
	}

	logger.Info("streaming compression complete",
		"files", result.FilesProcessed,
		"original", FormatSize(result.OriginalSize),
		"compressed", FormatSize(result.CompressedSize))

	return result, nil
}

// CompressFolderToZip compresses a folder to .zip archive (fallback).
func (s *StreamingCompressor) CompressFolderToZip(srcPath, archivePath string, progress func(current int, filename string)) (*StreamingResult, error) {
	result := &StreamingResult{
		Format: "zip",
		Errors: []string{},
	}

	// Ensure archive has correct extension
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

		// Get relative path
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return nil
		}

		// Skip root
		if relPath == "." {
			return nil
		}

		fileCount++
		if progress != nil {
			progress(fileCount, relPath)
		}

		// Skip directories in zip (they're implicit)
		if info.IsDir() {
			return nil
		}

		// Create zip entry
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

		// Stream file content
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

	// Close writer to flush
	zipWriter.Close()
	outFile.Close()

	// Get compressed size
	stat, err := os.Stat(archivePath)
	if err == nil {
		result.CompressedSize = stat.Size()
	}

	return result, nil
}

// ExtractTarZst extracts a .tar.zst archive.
func (s *StreamingCompressor) ExtractTarZst(archivePath, destPath string, progress func(message string)) error {
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	// Create zstd decoder
	zstdReader, err := zstd.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create zstd reader: %w", err)
	}
	defer zstdReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(zstdReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		if progress != nil {
			progress(header.Name)
		}

		target := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			outFile, err := os.Create(target)
			if err != nil {
				return err
			}

			buf := make([]byte, s.bufferSize)
			if _, err := io.CopyBuffer(outFile, tarReader, buf); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				logger.Warn("failed to set permissions", "file", target, "error", err)
			}
		}
	}

	return nil
}

// ExtractZip extracts a .zip archive.
func (s *StreamingCompressor) ExtractZip(archivePath, destPath string, progress func(message string)) error {
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

// Extract extracts an archive (auto-detects format).
func (s *StreamingCompressor) Extract(archivePath, destPath string, progress func(message string)) error {
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("failed to create destination: %w", err)
	}

	if strings.HasSuffix(archivePath, ".tar.zst") {
		return s.ExtractTarZst(archivePath, destPath, progress)
	} else if strings.HasSuffix(archivePath, ".zip") {
		return s.ExtractZip(archivePath, destPath, progress)
	} else if strings.HasSuffix(archivePath, ".tar.gz") || strings.HasSuffix(archivePath, ".tgz") {
		// Use existing Compressor for tar.gz
		c := NewCompressor(s.config)
		return c.ExtractArchive(archivePath, destPath, func(current int, filename string) {
			if progress != nil {
				progress(filename)
			}
		})
	}

	return fmt.Errorf("unsupported archive format: %s", archivePath)
}
