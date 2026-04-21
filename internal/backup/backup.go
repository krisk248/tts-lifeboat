// Package backup implements the three operations the menu exposes:
// NewBackup, History, Cleanup.
package backup

import (
	"archive/tar"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

// Item is one webapp entry (file or directory) the user can select.
type Item struct {
	Name  string
	Path  string
	Size  int64
	IsDir bool
}

// ListWebapps returns entries in webapps_path, sorted by name.
func ListWebapps(cfg *config.Config) ([]Item, error) {
	entries, err := os.ReadDir(cfg.WebappsPath)
	if err != nil {
		return nil, fmt.Errorf("read webapps folder: %w", err)
	}
	items := make([]Item, 0, len(entries))
	for _, e := range entries {
		full := filepath.Join(cfg.WebappsPath, e.Name())
		it := Item{Name: e.Name(), Path: full, IsDir: e.IsDir()}
		if e.IsDir() {
			it.Size = dirSize(full)
		} else if info, err := e.Info(); err == nil {
			it.Size = info.Size()
		}
		items = append(items, it)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// Run executes a backup of the given items plus extra_folders from the config.
// Destination folder = <backup_path>/YYYYMMDD/HHMM.
// Returns the destination path and total bytes copied.
func Run(cfg *config.Config, items []Item, progress func(step, total int, name string)) (string, int64, error) {
	now := time.Now()
	dest := filepath.Join(cfg.BackupPath, now.Format("20060102"), now.Format("1504"))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", 0, err
	}
	logger.Info("backup start dest=%s items=%d compression=%v", dest, len(items), cfg.Compression)

	total := len(items) + len(cfg.ExtraFolders)
	var bytes int64
	step := 0

	for _, it := range items {
		step++
		if progress != nil {
			progress(step, total, it.Name)
		}
		n, err := copyOne(it.Path, it.Name, dest, cfg.Compression)
		if err != nil {
			logger.Error("copy %s: %v", it.Name, err)
			return dest, bytes, err
		}
		bytes += n
		logger.Info("copied %s (%s)", it.Name, humanSize(n))
	}

	for _, folder := range cfg.ExtraFolders {
		step++
		name := filepath.Base(folder)
		if progress != nil {
			progress(step, total, name)
		}
		if _, err := os.Stat(folder); err != nil {
			logger.Error("extra folder %s missing, skipping", folder)
			continue
		}
		n, err := copyOne(folder, name, dest, cfg.Compression)
		if err != nil {
			logger.Error("copy extra %s: %v", folder, err)
			return dest, bytes, err
		}
		bytes += n
		logger.Info("copied extra %s (%s)", name, humanSize(n))
	}

	logger.Info("backup done dest=%s size=%s", dest, humanSize(bytes))
	return dest, bytes, nil
}

// copyOne copies a file or directory into dest, optionally as a .tar.zst archive.
// Returns bytes of original data read.
func copyOne(src, name, dest string, compress bool) (int64, error) {
	info, err := os.Stat(src)
	if err != nil {
		return 0, err
	}
	if compress {
		target := filepath.Join(dest, name+".tar.zst")
		return writeTarZst(src, target)
	}
	if info.IsDir() {
		return copyDir(src, filepath.Join(dest, name))
	}
	return copyFile(src, filepath.Join(dest, name))
}

func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	return io.Copy(out, in)
}

func copyDir(src, dst string) (int64, error) {
	var total int64
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode()|0o755)
		}
		n, err := copyFile(path, target)
		if err != nil {
			return err
		}
		total += n
		return nil
	})
	return total, err
}

func writeTarZst(src, archive string) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(archive), 0o755); err != nil {
		return 0, err
	}
	out, err := os.Create(archive)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	zw, err := zstd.NewWriter(out)
	if err != nil {
		return 0, err
	}
	defer zw.Close()

	tw := tar.NewWriter(zw)
	defer tw.Close()

	info, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	var total int64
	if !info.IsDir() {
		n, err := addFileToTar(tw, src, filepath.Base(src))
		return n, err
	}

	err = filepath.Walk(src, func(path string, fi os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if fi.IsDir() {
			hdr.Name += "/"
			return tw.WriteHeader(hdr)
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		n, err := io.Copy(tw, in)
		in.Close()
		if err != nil {
			return err
		}
		total += n
		return nil
	})
	return total, err
}

func addFileToTar(tw *tar.Writer, path, name string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return 0, err
	}
	hdr.Name = name
	if err := tw.WriteHeader(hdr); err != nil {
		return 0, err
	}
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return io.Copy(tw, f)
}

// HistoryEntry describes one past backup directory.
type HistoryEntry struct {
	Path string
	When time.Time
	Size int64
}

// History walks <backup_path>/YYYYMMDD/HHMM and returns entries newest first.
func History(cfg *config.Config) ([]HistoryEntry, error) {
	var entries []HistoryEntry
	dayEntries, err := os.ReadDir(cfg.BackupPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return entries, nil
		}
		return nil, err
	}
	for _, day := range dayEntries {
		if !day.IsDir() || !isDayFolder(day.Name()) {
			continue
		}
		dayPath := filepath.Join(cfg.BackupPath, day.Name())
		subs, err := os.ReadDir(dayPath)
		if err != nil {
			continue
		}
		for _, t := range subs {
			if !t.IsDir() || !isTimeFolder(t.Name()) {
				continue
			}
			full := filepath.Join(dayPath, t.Name())
			when, err := time.ParseInLocation("200601021504", day.Name()+t.Name(), time.Local)
			if err != nil {
				continue
			}
			entries = append(entries, HistoryEntry{
				Path: full,
				When: when,
				Size: dirSize(full),
			})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].When.After(entries[j].When) })
	return entries, nil
}

// Cleanup deletes history entries older than retention_days.
// If dryRun is true nothing is removed. Returns deleted entries and bytes freed.
func Cleanup(cfg *config.Config, dryRun bool) ([]HistoryEntry, int64, error) {
	if cfg.RetentionDays <= 0 {
		return nil, 0, nil
	}
	entries, err := History(cfg)
	if err != nil {
		return nil, 0, err
	}
	cutoff := time.Now().AddDate(0, 0, -cfg.RetentionDays)
	var deleted []HistoryEntry
	var freed int64
	for _, e := range entries {
		if !e.When.Before(cutoff) {
			continue
		}
		deleted = append(deleted, e)
		freed += e.Size
		if dryRun {
			continue
		}
		if err := os.RemoveAll(e.Path); err != nil {
			logger.Error("delete %s: %v", e.Path, err)
			continue
		}
		logger.Info("deleted old backup %s (%s)", e.Path, humanSize(e.Size))
		parent := filepath.Dir(e.Path)
		if empty, _ := isEmpty(parent); empty {
			_ = os.Remove(parent)
		}
	}
	return deleted, freed, nil
}

func isDayFolder(name string) bool {
	if len(name) != 8 {
		return false
	}
	_, err := time.Parse("20060102", name)
	return err == nil
}

func isTimeFolder(name string) bool {
	if len(name) != 4 {
		return false
	}
	_, err := time.Parse("1504", name)
	return err == nil
}

func isEmpty(dir string) (bool, error) {
	es, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	return len(es) == 0, nil
}

func dirSize(path string) int64 {
	var n int64
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			n += info.Size()
		}
		return nil
	})
	return n
}

// HumanSize formats bytes as KB/MB/GB for the UI.
func HumanSize(b int64) string { return humanSize(b) }

func humanSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// ParseSelection turns "1,3,10" into a deduped slice of 1-based indexes.
// Empty input means "all" → returned as nil.
func ParseSelection(input string, max int) ([]int, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil, nil
	}
	seen := map[int]bool{}
	var out []int
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(part, "%d", &n); err != nil {
			return nil, fmt.Errorf("invalid number %q", part)
		}
		if n < 1 || n > max {
			return nil, fmt.Errorf("number %d out of range (1-%d)", n, max)
		}
		if !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	return out, nil
}
