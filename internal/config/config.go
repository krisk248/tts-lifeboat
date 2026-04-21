// Package config loads the lifeboat.toml file next to the executable.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const DefaultFile = "lifeboat.toml"

// Load reads lifeboat.toml from path (or next to the binary if empty)
// and resolves relative paths against the config's directory.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultFile
	}
	if !filepath.IsAbs(path) {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(cwd, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	cfg := Default()
	if _, err := toml.Decode(string(data), cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	dir := filepath.Dir(path)
	if cfg.BackupPath == "." || cfg.BackupPath == "" {
		cfg.BackupPath = dir
	} else if !filepath.IsAbs(cfg.BackupPath) {
		cfg.BackupPath = filepath.Join(dir, cfg.BackupPath)
	}
	cfg.WebappsPath = normalize(cfg.WebappsPath)
	cfg.BackupPath = normalize(cfg.BackupPath)
	for i, f := range cfg.ExtraFolders {
		cfg.ExtraFolders[i] = normalize(f)
	}
	return cfg, nil
}

// normalize converts mixed separators to OS-native ones.
func normalize(p string) string {
	if p == "" {
		return p
	}
	p = strings.ReplaceAll(p, "\\\\", "/")
	p = strings.ReplaceAll(p, "\\", "/")
	return filepath.FromSlash(p)
}

// Example returns the commented TOML template written by `config init`.
func Example(name, webappsPath string) string {
	return fmt.Sprintf(`# TTS Lifeboat configuration
# Place this file as lifeboat.toml next to lifeboat.exe.

name = "%s"

# Absolute path to your Tomcat webapps folder. Forward slashes work on Windows.
webapps_path = "%s"

# Where backups are written. "." = same folder as this file.
backup_path = "."

# true  = compress each item into a .tar.zst archive
# false = plain folder copy (fastest, no compression)
compression = %t

# Auto-delete backups older than this many days (0 = never delete).
retention_days = 30

# Optional extra folders to back up alongside webapps (e.g. Tomcat conf).
# Leave empty to skip.
extra_folders = []
# Example:
# extra_folders = ["C:/TTS/MyApp/Tomcat/conf"]
`, name, webappsPath, defaultCompression())
}
