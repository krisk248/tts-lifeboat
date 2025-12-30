// Package storage provides storage abstraction for tts-lifeboat.
// This allows for future extensibility to cloud storage backends.
package storage

import (
	"io"
)

// Backend defines the interface for storage backends.
type Backend interface {
	// Name returns the backend name.
	Name() string

	// Type returns the backend type (local, s3, azure, gcs).
	Type() string

	// Write writes data to the specified path.
	Write(path string, reader io.Reader) error

	// Read reads data from the specified path.
	Read(path string) (io.ReadCloser, error)

	// Delete removes the file/directory at the specified path.
	Delete(path string) error

	// Exists checks if a path exists.
	Exists(path string) (bool, error)

	// List lists files at the specified path.
	List(path string) ([]FileInfo, error)

	// Stat returns file information.
	Stat(path string) (*FileInfo, error)

	// MkdirAll creates directories recursively.
	MkdirAll(path string) error
}

// FileInfo represents file metadata.
type FileInfo struct {
	Name    string
	Path    string
	Size    int64
	IsDir   bool
	ModTime int64
}

// Plugin defines the interface for storage plugins.
// This allows for future extensibility to different storage backends.
type Plugin interface {
	// Name returns the plugin name.
	Name() string

	// Type returns the storage type.
	Type() string

	// Initialize sets up the plugin with configuration.
	Initialize(config map[string]interface{}) error

	// Backend returns the storage backend.
	Backend() Backend
}

// Registry holds registered storage plugins.
type Registry struct {
	plugins map[string]Plugin
}

// NewRegistry creates a new plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register adds a plugin to the registry.
func (r *Registry) Register(plugin Plugin) {
	r.plugins[plugin.Name()] = plugin
}

// Get retrieves a plugin by name.
func (r *Registry) Get(name string) (Plugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// List returns all registered plugin names.
func (r *Registry) List() []string {
	names := make([]string, 0, len(r.plugins))
	for name := range r.plugins {
		names = append(names, name)
	}
	return names
}
