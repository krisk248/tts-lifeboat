package backup

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

// FileEntry represents a file to be backed up.
type FileEntry struct {
	SourcePath   string // Full path to the source file
	RelativePath string // Path relative to backup root
	Size         int64
	IsDir        bool
	Category     string // "webapp", "custom", etc.
}

// CollectionResult holds the result of file collection.
type CollectionResult struct {
	Files      []FileEntry
	TotalSize  int64
	TotalCount int
	Errors     []string
}

// Collector collects files for backup based on configuration.
type Collector struct {
	config *config.Config
}

// NewCollector creates a new file collector.
func NewCollector(cfg *config.Config) *Collector {
	return &Collector{config: cfg}
}

// Collect gathers all files to be backed up.
func (c *Collector) Collect() *CollectionResult {
	result := &CollectionResult{
		Files:  []FileEntry{},
		Errors: []string{},
	}

	// Collect webapp files
	c.collectWebapps(result)

	// Collect custom folders
	c.collectCustomFolders(result)

	return result
}

// collectWebapps collects files from the webapps directory.
func (c *Collector) collectWebapps(result *CollectionResult) {
	webappsPath := c.config.WebappsPath

	// If specific webapps are listed, only collect those
	if len(c.config.Webapps) > 0 {
		for _, webapp := range c.config.Webapps {
			webappPath := filepath.Join(webappsPath, webapp)
			if _, err := os.Stat(webappPath); os.IsNotExist(err) {
				result.Errors = append(result.Errors, "webapp not found: "+webapp)
				continue
			}
			c.collectPath(webappPath, "webapps/"+webapp, "webapp", result)
		}
	} else {
		// Collect all webapps
		entries, err := os.ReadDir(webappsPath)
		if err != nil {
			result.Errors = append(result.Errors, "failed to read webapps directory: "+err.Error())
			return
		}

		for _, entry := range entries {
			webappPath := filepath.Join(webappsPath, entry.Name())
			c.collectPath(webappPath, "webapps/"+entry.Name(), "webapp", result)
		}
	}
}

// collectCustomFolders collects files from custom folders.
func (c *Collector) collectCustomFolders(result *CollectionResult) {
	for _, folder := range c.config.CustomFolders {
		if _, err := os.Stat(folder.Path); os.IsNotExist(err) {
			if folder.Required {
				result.Errors = append(result.Errors, "required folder not found: "+folder.Path)
			} else {
				logger.Warn("optional folder not found", "path", folder.Path, "title", folder.Title)
			}
			continue
		}

		// Use folder title as the relative path base
		safeTitle := sanitizeFolderName(folder.Title)
		c.collectPathWithPatterns(folder.Path, safeTitle, "custom", folder.Include, folder.Exclude, result)
	}
}

// collectPath collects all files from a path.
func (c *Collector) collectPath(srcPath, relBase, category string, result *CollectionResult) {
	c.collectPathWithPatterns(srcPath, relBase, category, nil, nil, result)
}

// collectPathWithPatterns collects files with include/exclude patterns.
func (c *Collector) collectPathWithPatterns(srcPath, relBase, category string, include, exclude []string, result *CollectionResult) {
	info, err := os.Stat(srcPath)
	if err != nil {
		result.Errors = append(result.Errors, "failed to stat path: "+err.Error())
		return
	}

	// If it's a file (like a .war file), add it directly
	if !info.IsDir() {
		entry := FileEntry{
			SourcePath:   srcPath,
			RelativePath: relBase,
			Size:         info.Size(),
			IsDir:        false,
			Category:     category,
		}
		result.Files = append(result.Files, entry)
		result.TotalSize += info.Size()
		result.TotalCount++
		return
	}

	// Walk directory
	err = filepath.WalkDir(srcPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Warn("error accessing path", "path", path, "error", err)
			return nil // Continue walking
		}

		// Get relative path from source
		relPath, err := filepath.Rel(srcPath, path)
		if err != nil {
			return nil
		}

		// Full relative path for backup
		fullRelPath := filepath.Join(relBase, relPath)

		// Apply include patterns (if specified)
		if len(include) > 0 && !d.IsDir() {
			matched := false
			for _, pattern := range include {
				if matchPattern(d.Name(), pattern) || matchPattern(relPath, pattern) {
					matched = true
					break
				}
			}
			if !matched {
				return nil
			}
		}

		// Apply exclude patterns
		for _, pattern := range exclude {
			if matchPattern(d.Name(), pattern) || matchPattern(relPath, pattern) {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			return nil
		}

		entry := FileEntry{
			SourcePath:   path,
			RelativePath: fullRelPath,
			Size:         info.Size(),
			IsDir:        d.IsDir(),
			Category:     category,
		}

		result.Files = append(result.Files, entry)
		if !d.IsDir() {
			result.TotalSize += info.Size()
			result.TotalCount++
		}

		return nil
	})

	if err != nil {
		result.Errors = append(result.Errors, "walk error: "+err.Error())
	}
}

// matchPattern matches a filename against a glob pattern.
func matchPattern(name, pattern string) bool {
	// Handle ** pattern for recursive matching
	if strings.Contains(pattern, "**") {
		pattern = strings.ReplaceAll(pattern, "**", "*")
	}

	matched, _ := filepath.Match(pattern, name)
	return matched
}

// sanitizeFolderName creates a safe folder name from a title.
func sanitizeFolderName(title string) string {
	// Replace spaces and special chars with underscores
	title = strings.ReplaceAll(title, " ", "_")
	title = strings.ReplaceAll(title, "/", "_")
	title = strings.ReplaceAll(title, "\\", "_")
	title = strings.ReplaceAll(title, ":", "_")
	return strings.ToLower(title)
}

// GetFilesByCategory filters files by category.
func (r *CollectionResult) GetFilesByCategory(category string) []FileEntry {
	var files []FileEntry
	for _, f := range r.Files {
		if f.Category == category {
			files = append(files, f)
		}
	}
	return files
}

// GetDirectories returns only directory entries.
func (r *CollectionResult) GetDirectories() []FileEntry {
	var dirs []FileEntry
	for _, f := range r.Files {
		if f.IsDir {
			dirs = append(dirs, f)
		}
	}
	return dirs
}

// GetFiles returns only file entries (not directories).
func (r *CollectionResult) GetFiles() []FileEntry {
	var files []FileEntry
	for _, f := range r.Files {
		if !f.IsDir {
			files = append(files, f)
		}
	}
	return files
}
