// TTS Lifeboat - simple menu-driven backup tool for Tomcat webapps.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kannan/tts-lifeboat/internal/app"
	"github.com/kannan/tts-lifeboat/internal/backup"
	"github.com/kannan/tts-lifeboat/internal/config"
	"github.com/kannan/tts-lifeboat/internal/logger"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	// `lifeboat init` writes a starter TOML next to the binary and exits.
	if len(os.Args) > 1 && os.Args[1] == "init" {
		if err := writeInitTemplate(); err != nil {
			fmt.Fprintln(os.Stderr, "ERROR:", err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Create lifeboat.toml next to this executable.")
		fmt.Fprintln(os.Stderr, "Run `lifeboat init` to generate a template.")
		pause(reader)
		os.Exit(1)
	}
	if err := logger.Init(cfg.BackupPath); err != nil {
		fmt.Fprintln(os.Stderr, "WARN: could not open log file:", err)
	}
	defer logger.Close()
	logger.Info("session start name=%s webapps=%s backup=%s", cfg.Name, cfg.WebappsPath, cfg.BackupPath)

	for {
		clearScreen()
		printHeader(cfg)
		printMenu(cfg)
		choice := strings.TrimSpace(readLine(reader, "Enter your choice (1-4): "))
		switch choice {
		case "1":
			runNewBackup(cfg, reader)
		case "2":
			runHistory(cfg, reader)
		case "3":
			runCleanup(cfg, reader)
		case "4", "q", "Q":
			fmt.Println("Goodbye.")
			return
		default:
			fmt.Println("Invalid choice.")
			pause(reader)
		}
	}
}

func printHeader(cfg *config.Config) {
	fmt.Println("===============================================")
	fmt.Println("   TTS LIFEBOAT v" + app.Version)
	fmt.Println("   Created by " + app.Creator + " from TTS")
	fmt.Println("   Project: " + cfg.Name)
	fmt.Println("===============================================")
	fmt.Println()
}

func printMenu(cfg *config.Config) {
	fmt.Println("What would you like to do?")
	fmt.Println()
	fmt.Println("  1. Create New Backup")
	fmt.Println("  2. View Backup History")
	if cfg.RetentionDays > 0 {
		fmt.Printf("  3. Cleanup Old Backups (older than %d days)\n", cfg.RetentionDays)
	} else {
		fmt.Println("  3. Cleanup Old Backups (disabled: retention_days = 0)")
	}
	fmt.Println("  4. Exit")
	fmt.Println()
}

func runNewBackup(cfg *config.Config, reader *bufio.Reader) {
	items, err := backup.ListWebapps(cfg)
	if err != nil {
		fmt.Println("ERROR:", err)
		logger.Error("list webapps: %v", err)
		pause(reader)
		return
	}
	if len(items) == 0 {
		fmt.Println("No items found in", cfg.WebappsPath)
		pause(reader)
		return
	}

	fmt.Printf("\nFound %d items in %s:\n", len(items), cfg.WebappsPath)
	for i, it := range items {
		kind := "file"
		if it.IsDir {
			kind = "dir "
		}
		fmt.Printf("  [%2d] %s  %-6s  %s\n", i+1, kind, backup.HumanSize(it.Size), it.Name)
	}
	fmt.Println()

	input := strings.TrimSpace(readLine(reader, "Enter numbers to backup (e.g. 1,3  or blank for ALL): "))
	selected, err := backup.ParseSelection(input, len(items))
	if err != nil {
		fmt.Println("ERROR:", err)
		pause(reader)
		return
	}
	var chosen []backup.Item
	if selected == nil {
		chosen = items
	} else {
		for _, n := range selected {
			chosen = append(chosen, items[n-1])
		}
	}

	fmt.Println()
	fmt.Printf("Backing up %d items (compression=%v)...\n", len(chosen), cfg.Compression)
	start := time.Now()
	dest, bytes, err := backup.Run(cfg, chosen, func(step, total int, name string) {
		fmt.Printf("  [%d/%d] %s\n", step, total, name)
	})
	if err != nil {
		fmt.Println("ERROR:", err)
		pause(reader)
		return
	}
	fmt.Println()
	fmt.Println("Backup complete.")
	fmt.Println("  Location:", dest)
	fmt.Println("  Size:    ", backup.HumanSize(bytes))
	fmt.Println("  Duration:", time.Since(start).Round(time.Millisecond))
	pause(reader)
}

func runHistory(cfg *config.Config, reader *bufio.Reader) {
	entries, err := backup.History(cfg)
	if err != nil {
		fmt.Println("ERROR:", err)
		pause(reader)
		return
	}
	fmt.Println()
	if len(entries) == 0 {
		fmt.Println("No previous backups.")
		pause(reader)
		return
	}
	fmt.Printf("Backup history (%d total):\n\n", len(entries))
	fmt.Println("  When                  Size      Path")
	fmt.Println("  --------------------  --------  ------------------------------------")
	for _, e := range entries {
		fmt.Printf("  %-20s  %-8s  %s\n",
			e.When.Format("2006-01-02 15:04"),
			backup.HumanSize(e.Size),
			e.Path)
	}
	pause(reader)
}

func runCleanup(cfg *config.Config, reader *bufio.Reader) {
	if cfg.RetentionDays <= 0 {
		fmt.Println("Retention disabled (retention_days = 0).")
		pause(reader)
		return
	}
	preview, freed, err := backup.Cleanup(cfg, true)
	if err != nil {
		fmt.Println("ERROR:", err)
		pause(reader)
		return
	}
	fmt.Println()
	if len(preview) == 0 {
		fmt.Printf("Nothing to delete. No backups older than %d days.\n", cfg.RetentionDays)
		pause(reader)
		return
	}
	fmt.Printf("Backups older than %d days:\n\n", cfg.RetentionDays)
	for _, e := range preview {
		fmt.Printf("  %s  %-8s  %s\n",
			e.When.Format("2006-01-02 15:04"),
			backup.HumanSize(e.Size),
			e.Path)
	}
	fmt.Printf("\nTotal space to free: %s\n\n", backup.HumanSize(freed))

	ans := strings.ToLower(strings.TrimSpace(readLine(reader, "Delete these backups? (y/N): ")))
	if ans != "y" && ans != "yes" {
		fmt.Println("Cancelled.")
		pause(reader)
		return
	}
	deleted, freed, err := backup.Cleanup(cfg, false)
	if err != nil {
		fmt.Println("ERROR:", err)
		pause(reader)
		return
	}
	fmt.Printf("\nDeleted %d backup(s), freed %s.\n", len(deleted), backup.HumanSize(freed))
	pause(reader)
}

func writeInitTemplate() error {
	out := config.DefaultFile
	if _, err := os.Stat(out); err == nil {
		return fmt.Errorf("%s already exists", out)
	}
	content := config.Example("my-webapp", "")
	if err := os.WriteFile(out, []byte(content), 0o644); err != nil {
		return err
	}
	abs, _ := filepath.Abs(out)
	fmt.Println("Created:", abs)
	fmt.Println("Edit the file and set name + webapps_path, then run `lifeboat`.")
	return nil
}

func readLine(r *bufio.Reader, prompt string) string {
	fmt.Print(prompt)
	line, err := r.ReadString('\n')
	if err != nil {
		return ""
	}
	return strings.TrimRight(line, "\r\n")
}

func pause(r *bufio.Reader) {
	fmt.Println()
	fmt.Print("Press Enter to continue...")
	_, _ = r.ReadString('\n')
}

func clearScreen() {
	// Simple cross-platform: emit a bunch of newlines. Avoids cmd/terminal
	// specific escape sequences for Windows 2008 R2 compatibility.
	fmt.Print(strings.Repeat("\n", 2))
}
