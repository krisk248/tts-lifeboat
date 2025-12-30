// Package logger provides structured logging using slog for tts-lifeboat.
package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

var (
	// Default is the default logger instance.
	Default *slog.Logger
)

func init() {
	// Initialize with console output by default
	Default = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Config holds logger configuration.
type Config struct {
	Path     string
	Level    string
	MaxSize  int64
	MaxFiles int
	Console  bool
}

// Init initializes the logger with the given configuration.
func Init(cfg Config) error {
	var level slog.Level
	switch strings.ToLower(cfg.Level) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{
		Level: level,
	}

	var writers []io.Writer

	// Console output
	if cfg.Console {
		writers = append(writers, os.Stdout)
	}

	// File output
	if cfg.Path != "" {
		// Create log directory if it doesn't exist
		logDir := filepath.Dir(cfg.Path)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return err
		}

		file, err := os.OpenFile(cfg.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		writers = append(writers, file)
	}

	if len(writers) == 0 {
		writers = append(writers, os.Stdout)
	}

	var writer io.Writer
	if len(writers) == 1 {
		writer = writers[0]
	} else {
		writer = io.MultiWriter(writers...)
	}

	Default = slog.New(slog.NewTextHandler(writer, opts))
	return nil
}

// Debug logs a debug message.
func Debug(msg string, args ...any) {
	Default.Debug(msg, args...)
}

// Info logs an info message.
func Info(msg string, args ...any) {
	Default.Info(msg, args...)
}

// Warn logs a warning message.
func Warn(msg string, args ...any) {
	Default.Warn(msg, args...)
}

// Error logs an error message.
func Error(msg string, args ...any) {
	Default.Error(msg, args...)
}

// WithGroup returns a logger with the given group name.
func WithGroup(name string) *slog.Logger {
	return Default.WithGroup(name)
}

// With returns a logger with the given attributes.
func With(args ...any) *slog.Logger {
	return Default.With(args...)
}
