package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

// Compressor handles file compression for backups.
type Compressor struct {
	config         *config.Config
	skipExtensions map[string]bool
	bufferSize     int
}

// CompressionResult holds the result of a compression operation.
type CompressionResult struct {
	OriginalSize   int64
	CompressedSize int64
	FilesProcessed int
	FilesSkipped   int
	Errors         []string
}

// NewCompressor creates a new file compressor.
func NewCompressor(cfg *config.Config) *Compressor {
	skipExt := make(map[string]bool)
	for _, ext := range cfg.Compression.SkipExtensions {
		skipExt[strings.ToLower(ext)] = true
	}

	return &Compressor{
		config:         cfg,
		skipExtensions: skipExt,
		bufferSize:     32 * 1024 * 1024, // 32MB buffer
	}
}

// ShouldCompress determines if a file should be compressed.
func (c *Compressor) ShouldCompress(filename string) bool {
	if !c.config.Compression.Enabled {
		return false
	}

	ext := strings.ToLower(filepath.Ext(filename))
	return !c.skipExtensions[ext]
}

// CreateArchive creates a tar.gz archive from collected files.
func (c *Compressor) CreateArchive(files []FileEntry, outputPath string, progress func(current, total int, filename string)) (*CompressionResult, error) {
	result := &CompressionResult{
		Errors: []string{},
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create archive: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer with configured level
	gzWriter, err := gzip.NewWriterLevel(outFile, c.config.Compression.Level)
	if err != nil {
		return nil, fmt.Errorf("failed to create gzip writer: %w", err)
	}
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Process files
	total := len(files)
	for i, entry := range files {
		if progress != nil {
			progress(i+1, total, entry.RelativePath)
		}

		if err := c.addToArchive(tarWriter, entry, result); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", entry.RelativePath, err))
			logger.Warn("failed to add file to archive", "file", entry.RelativePath, "error", err)
		}
	}

	// Get compressed size
	tarWriter.Close()
	gzWriter.Close()
	outFile.Close()

	stat, err := os.Stat(outputPath)
	if err == nil {
		result.CompressedSize = stat.Size()
	}

	return result, nil
}

// addToArchive adds a single file or directory to the tar archive.
func (c *Compressor) addToArchive(tw *tar.Writer, entry FileEntry, result *CompressionResult) error {
	info, err := os.Stat(entry.SourcePath)
	if err != nil {
		return err
	}

	// Create tar header
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}

	// Use relative path in archive
	header.Name = filepath.ToSlash(entry.RelativePath)

	if entry.IsDir {
		header.Name += "/"
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	// If it's a directory, we're done
	if entry.IsDir {
		return nil
	}

	// Open source file
	srcFile, err := os.Open(entry.SourcePath)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Copy file content
	buf := make([]byte, c.bufferSize)
	written, err := io.CopyBuffer(tw, srcFile, buf)
	if err != nil {
		return err
	}

	result.OriginalSize += written
	result.FilesProcessed++

	return nil
}

// CopyFile copies a single file without compression (for already compressed files).
func (c *Compressor) CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

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

	buf := make([]byte, c.bufferSize)
	_, err = io.CopyBuffer(dstFile, srcFile, buf)
	return err
}

// ExtractArchive extracts a tar.gz archive to the destination.
func (c *Compressor) ExtractArchive(archivePath, destPath string, progress func(current int, filename string)) error {
	// Open archive
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	count := 0
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("tar read error: %w", err)
		}

		count++
		if progress != nil {
			progress(count, header.Name)
		}

		// Determine output path
		target := filepath.Join(destPath, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			// Create parent directory
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}

			// Create file
			outFile, err := os.Create(target)
			if err != nil {
				return err
			}

			buf := make([]byte, c.bufferSize)
			if _, err := io.CopyBuffer(outFile, tarReader, buf); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()

			// Set file permissions
			if err := os.Chmod(target, os.FileMode(header.Mode)); err != nil {
				logger.Warn("failed to set permissions", "file", target, "error", err)
			}
		}
	}

	return nil
}

// CalculateCompressionRatio returns the compression ratio.
func (r *CompressionResult) CalculateCompressionRatio() float64 {
	if r.OriginalSize == 0 {
		return 0
	}
	return float64(r.CompressedSize) / float64(r.OriginalSize) * 100
}

// GetSavings returns the bytes saved through compression.
func (r *CompressionResult) GetSavings() int64 {
	return r.OriginalSize - r.CompressedSize
}
