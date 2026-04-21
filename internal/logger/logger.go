// Package logger writes human-readable log lines to both the terminal and
// a file under <backup_path>/logs/lifeboat.log.
package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

var fileWriter io.WriteCloser

// Init opens logs/lifeboat.log under backupDir. Safe to call multiple times;
// it replaces the previous writer.
func Init(backupDir string) error {
	if fileWriter != nil {
		_ = fileWriter.Close()
		fileWriter = nil
	}
	dir := filepath.Join(backupDir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "lifeboat.log"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	fileWriter = f
	return nil
}

// Close flushes and closes the log file.
func Close() {
	if fileWriter != nil {
		_ = fileWriter.Close()
		fileWriter = nil
	}
}

func write(level, msg string) {
	line := fmt.Sprintf("%s [%s] %s\n",
		time.Now().Format("2006-01-02 15:04:05"), level, msg)
	if fileWriter != nil {
		_, _ = fileWriter.Write([]byte(line))
	}
}

// Info writes an INFO line to the log file only (terminal stays clean).
func Info(format string, a ...any) {
	write("INFO", fmt.Sprintf(format, a...))
}

// Error writes an ERROR line to both file and stderr.
func Error(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	write("ERROR", msg)
	fmt.Fprintln(os.Stderr, "ERROR:", msg)
}
