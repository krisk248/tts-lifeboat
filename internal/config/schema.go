// Package config provides YAML configuration loading and validation for tts-lifeboat.
package config

import (
	"path/filepath"
	"strings"
)

// Config represents the main configuration structure for tts-lifeboat.
type Config struct {
	Name          string         `yaml:"name"`
	Environment   string         `yaml:"environment"`
	WebappsPath   string         `yaml:"webapps_path"`
	BackupPath    string         `yaml:"backup_path"`
	Webapps       []string       `yaml:"webapps"`
	CustomFolders []CustomFolder `yaml:"custom_folders"`
	Retention     Retention      `yaml:"retention"`
	Compression   Compression    `yaml:"compression"`
	SevenZip      SevenZip       `yaml:"seven_zip"`
	Logging       Logging        `yaml:"logging"`
}

// CustomFolder represents an additional folder to backup.
type CustomFolder struct {
	Title    string   `yaml:"title"`
	Path     string   `yaml:"path"`
	Required bool     `yaml:"required"`
	Include  []string `yaml:"include,omitempty"`
	Exclude  []string `yaml:"exclude,omitempty"`
}

// Retention defines backup retention policy.
type Retention struct {
	Days    int  `yaml:"days"`
	MinKeep int  `yaml:"min_keep"`
	Enabled bool `yaml:"enabled"`
}

// Compression defines compression settings.
type Compression struct {
	Enabled        bool     `yaml:"enabled"`
	Level          int      `yaml:"level"`
	SkipExtensions []string `yaml:"skip_extensions"`
}

// SevenZip defines 7-Zip external compressor settings.
type SevenZip struct {
	Path    string `yaml:"path"`
	Level   int    `yaml:"level"`
	Threads int    `yaml:"threads"`
}

// Logging defines logging configuration.
type Logging struct {
	Path     string `yaml:"path"`
	Level    string `yaml:"level"`
	MaxSize  string `yaml:"max_size"`
	MaxFiles int    `yaml:"max_files"`
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Name:          "my-webapp",
		Environment:   "production",
		WebappsPath:   "",
		BackupPath:    ".",
		Webapps:       []string{},
		CustomFolders: []CustomFolder{},
		Retention: Retention{
			Days:    30,
			MinKeep: 5,
			Enabled: true,
		},
		Compression: Compression{
			Enabled: true,
			Level:   5,
			SkipExtensions: []string{
				".war", ".jar", ".zip", ".gz", ".tar.gz", ".tgz",
				".7z", ".rar", ".bz2", ".xz",
			},
		},
		SevenZip: SevenZip{
			Path:    "", // Will auto-detect
			Level:   5,  // Balanced
			Threads: 1,  // Low RAM usage
		},
		Logging: Logging{
			Path:     "./logs/lifeboat.log",
			Level:    "info",
			MaxSize:  "10MB",
			MaxFiles: 5,
		},
	}
}

// NormalizePath converts path to OS-native format and handles both slash types.
func NormalizePath(path string) string {
	// Convert forward slashes to backslashes on Windows (filepath.Clean handles this)
	// Also handles C:\\ and C:/ formats
	path = strings.ReplaceAll(path, "\\\\", "/")
	path = strings.ReplaceAll(path, "\\", "/")
	return filepath.FromSlash(path)
}

// GetWebappsPath returns the normalized webapps path.
func (c *Config) GetWebappsPath() string {
	return NormalizePath(c.WebappsPath)
}

// GetBackupPath returns the normalized backup path.
func (c *Config) GetBackupPath() string {
	return NormalizePath(c.BackupPath)
}

// GetSevenZipPath returns the 7-Zip executable path.
// If not configured, returns common default locations.
func (c *Config) GetSevenZipPath() string {
	if c.SevenZip.Path != "" {
		return NormalizePath(c.SevenZip.Path)
	}
	// Default paths to try
	return ""
}
