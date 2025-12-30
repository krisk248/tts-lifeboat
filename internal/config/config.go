package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// DefaultConfigFile is the default configuration filename.
	DefaultConfigFile = "lifeboat.yaml"
)

// Load reads and parses a configuration file from the given path.
// If path is empty, it looks for lifeboat.yaml in the current directory.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultConfigFile
	}

	// Make path absolute if relative
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("failed to get working directory: %w", err)
		}
		path = filepath.Join(cwd, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Resolve relative paths
	configDir := filepath.Dir(path)
	cfg.resolvePaths(configDir)

	return cfg, nil
}

// LoadFromBytes parses configuration from YAML bytes.
func LoadFromBytes(data []byte) (*Config, error) {
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}
	return cfg, nil
}

// Save writes the configuration to a YAML file.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// resolvePaths converts relative paths to absolute paths based on config directory.
func (c *Config) resolvePaths(configDir string) {
	// Resolve backup path
	if c.BackupPath == "." || c.BackupPath == "" {
		c.BackupPath = configDir
	} else if !filepath.IsAbs(c.BackupPath) {
		c.BackupPath = filepath.Join(configDir, c.BackupPath)
	}

	// Resolve logging path
	if c.Logging.Path != "" && !filepath.IsAbs(c.Logging.Path) {
		c.Logging.Path = filepath.Join(configDir, c.Logging.Path)
	}
}

// GetBackupDestination returns the full path for a new backup.
// Format: backup_path/YYYYMMDD/HHMM
func (c *Config) GetBackupDestination(date string, time string) string {
	return filepath.Join(c.BackupPath, date, time)
}

// GetCheckpointDestination returns the full path for a checkpoint backup.
// Format: backup_path/YYYYMMDD_description
func (c *Config) GetCheckpointDestination(date string, description string) string {
	return filepath.Join(c.BackupPath, fmt.Sprintf("%s_%s", date, description))
}

// GetIndexPath returns the path to the backup index file.
func (c *Config) GetIndexPath() string {
	return filepath.Join(c.BackupPath, "index.json")
}

// GetLogsPath returns the path to the logs directory.
func (c *Config) GetLogsPath() string {
	return filepath.Join(c.BackupPath, "logs")
}
