// Package backup provides the core backup engine for tts-lifeboat.
package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"
)

// Metadata represents backup metadata stored in metadata.json.
type Metadata struct {
	ID              string    `json:"id"`
	CreatedAt       time.Time `json:"created_at"`
	DurationSeconds int       `json:"duration_seconds"`
	Files           FileStats `json:"files"`
	Note            string    `json:"note,omitempty"`
}

// FileStats holds file statistics for a backup.
type FileStats struct {
	Count          int    `json:"count"`
	OriginalSize   string `json:"original_size"`
	CompressedSize string `json:"compressed_size"`
}

// IndexEntry represents a single backup in the index.
type IndexEntry struct {
	ID          string    `json:"id"`
	Date        time.Time `json:"date"`
	Path        string    `json:"path"`
	Size        string    `json:"size"`
	DeleteAfter string    `json:"delete_after,omitempty"`
	Checkpoint  bool      `json:"checkpoint"`
	Note        string    `json:"note,omitempty"`
}

// Index represents the backup index stored in index.json.
type Index struct {
	Backups []IndexEntry `json:"backups"`
}

// NewMetadata creates a new backup metadata.
func NewMetadata(id string) *Metadata {
	return &Metadata{
		ID:        id,
		CreatedAt: time.Now(),
	}
}

// SaveMetadata writes metadata to a JSON file.
func SaveMetadata(path string, meta *Metadata) error {
	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}

	return nil
}

// LoadMetadata reads metadata from a JSON file.
func LoadMetadata(path string) (*Metadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata: %w", err)
	}

	var meta Metadata
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &meta, nil
}

// LoadIndex loads the backup index from the given path.
func LoadIndex(path string) (*Index, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Index{Backups: []IndexEntry{}}, nil
		}
		return nil, fmt.Errorf("failed to read index: %w", err)
	}

	var index Index
	if err := json.Unmarshal(data, &index); err != nil {
		return nil, fmt.Errorf("failed to parse index: %w", err)
	}

	return &index, nil
}

// SaveIndex writes the index to a JSON file.
func SaveIndex(path string, index *Index) error {
	// Sort by date descending (newest first)
	sort.Slice(index.Backups, func(i, j int) bool {
		return index.Backups[i].Date.After(index.Backups[j].Date)
	})

	data, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal index: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write index: %w", err)
	}

	return nil
}

// AddEntry adds a new backup entry to the index.
func (idx *Index) AddEntry(entry IndexEntry) {
	idx.Backups = append(idx.Backups, entry)
}

// GetLatest returns the most recent backup entry.
func (idx *Index) GetLatest() *IndexEntry {
	if len(idx.Backups) == 0 {
		return nil
	}

	// Sort to ensure newest first
	sort.Slice(idx.Backups, func(i, j int) bool {
		return idx.Backups[i].Date.After(idx.Backups[j].Date)
	})

	return &idx.Backups[0]
}

// GetByID finds a backup entry by its ID.
func (idx *Index) GetByID(id string) *IndexEntry {
	for i := range idx.Backups {
		if idx.Backups[i].ID == id {
			return &idx.Backups[i]
		}
	}
	return nil
}

// MarkAsCheckpoint marks a backup as a checkpoint (never auto-delete).
func (idx *Index) MarkAsCheckpoint(id string, note string) bool {
	for i := range idx.Backups {
		if idx.Backups[i].ID == id {
			idx.Backups[i].Checkpoint = true
			idx.Backups[i].DeleteAfter = ""
			if note != "" {
				idx.Backups[i].Note = note
			}
			return true
		}
	}
	return false
}

// RemoveEntry removes a backup entry by ID.
func (idx *Index) RemoveEntry(id string) bool {
	for i := range idx.Backups {
		if idx.Backups[i].ID == id {
			idx.Backups = append(idx.Backups[:i], idx.Backups[i+1:]...)
			return true
		}
	}
	return false
}

// GetExpired returns backup entries that have exceeded their delete_after date.
func (idx *Index) GetExpired() []IndexEntry {
	var expired []IndexEntry
	now := time.Now()

	for _, entry := range idx.Backups {
		if entry.Checkpoint {
			continue // Checkpoints never expire
		}

		if entry.DeleteAfter != "" {
			deleteDate, err := time.Parse("2006-01-02", entry.DeleteAfter)
			if err == nil && now.After(deleteDate) {
				expired = append(expired, entry)
			}
		}
	}

	return expired
}

// GenerateBackupID generates a unique backup ID based on timestamp.
func GenerateBackupID() string {
	return fmt.Sprintf("backup-%s", time.Now().Format("20060102-150405"))
}

// GetDateFolder returns the date folder name (YYYYMMDD).
func GetDateFolder() string {
	return time.Now().Format("20060102")
}

// GetTimeFolder returns the time folder name (HHMM).
func GetTimeFolder() string {
	return time.Now().Format("1504")
}

// FormatSize formats bytes into human-readable string.
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// ParseSize parses a human-readable size string to bytes.
func ParseSize(s string) (int64, error) {
	var value float64
	var unit string

	_, err := fmt.Sscanf(s, "%f %s", &value, &unit)
	if err != nil {
		_, err = fmt.Sscanf(s, "%f%s", &value, &unit)
		if err != nil {
			return 0, fmt.Errorf("invalid size format: %s", s)
		}
	}

	multipliers := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	mult, ok := multipliers[unit]
	if !ok {
		return 0, fmt.Errorf("unknown unit: %s", unit)
	}

	return int64(value * float64(mult)), nil
}
