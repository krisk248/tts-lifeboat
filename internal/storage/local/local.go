// Package local provides local filesystem storage backend.
package local

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kannan/tts-lifeboat/internal/storage"
)

// Backend implements the local filesystem storage backend.
type Backend struct {
	basePath string
}

// New creates a new local storage backend.
func New(basePath string) *Backend {
	return &Backend{basePath: basePath}
}

// Name returns the backend name.
func (b *Backend) Name() string {
	return "local"
}

// Type returns the backend type.
func (b *Backend) Type() string {
	return "local"
}

// Write writes data to the specified path.
func (b *Backend) Write(path string, reader io.Reader) error {
	fullPath := b.resolvePath(path)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, reader); err != nil {
		return fmt.Errorf("failed to write data: %w", err)
	}

	return nil
}

// Read reads data from the specified path.
func (b *Backend) Read(path string) (io.ReadCloser, error) {
	fullPath := b.resolvePath(path)
	return os.Open(fullPath)
}

// Delete removes the file/directory at the specified path.
func (b *Backend) Delete(path string) error {
	fullPath := b.resolvePath(path)
	return os.RemoveAll(fullPath)
}

// Exists checks if a path exists.
func (b *Backend) Exists(path string) (bool, error) {
	fullPath := b.resolvePath(path)
	_, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// List lists files at the specified path.
func (b *Backend) List(path string) ([]storage.FileInfo, error) {
	fullPath := b.resolvePath(path)

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}

	files := make([]storage.FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, storage.FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Unix(),
		})
	}

	return files, nil
}

// Stat returns file information.
func (b *Backend) Stat(path string) (*storage.FileInfo, error) {
	fullPath := b.resolvePath(path)

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, err
	}

	return &storage.FileInfo{
		Name:    info.Name(),
		Path:    path,
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime().Unix(),
	}, nil
}

// MkdirAll creates directories recursively.
func (b *Backend) MkdirAll(path string) error {
	fullPath := b.resolvePath(path)
	return os.MkdirAll(fullPath, 0755)
}

// resolvePath converts a relative path to absolute path.
func (b *Backend) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(b.basePath, path)
}

// Plugin implements the storage plugin interface for local filesystem.
type Plugin struct {
	backend *Backend
}

// NewPlugin creates a new local storage plugin.
func NewPlugin() *Plugin {
	return &Plugin{}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "local"
}

// Type returns the storage type.
func (p *Plugin) Type() string {
	return "local"
}

// Initialize sets up the plugin with configuration.
func (p *Plugin) Initialize(config map[string]interface{}) error {
	basePath, ok := config["path"].(string)
	if !ok {
		basePath = "."
	}

	p.backend = New(basePath)
	return nil
}

// Backend returns the storage backend.
func (p *Plugin) Backend() storage.Backend {
	return p.backend
}
