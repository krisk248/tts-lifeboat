//go:build legacy

// Package logger provides logging for tts-lifeboat.
// This file is used for legacy builds (Go 1.20) without slog.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Level represents log level.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

var (
	currentLevel Level = LevelInfo
	logOutput    io.Writer
)

func init() {
	logOutput = os.Stdout
	log.SetOutput(logOutput)
	log.SetFlags(0) // We'll format our own output
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
	switch strings.ToLower(cfg.Level) {
	case "debug":
		currentLevel = LevelDebug
	case "info":
		currentLevel = LevelInfo
	case "warn", "warning":
		currentLevel = LevelWarn
	case "error":
		currentLevel = LevelError
	default:
		currentLevel = LevelInfo
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

	if len(writers) == 1 {
		logOutput = writers[0]
	} else {
		logOutput = io.MultiWriter(writers...)
	}

	log.SetOutput(logOutput)
	return nil
}

func logMsg(level string, msg string, args ...any) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Build key=value pairs from args
	var kvPairs string
	for i := 0; i < len(args)-1; i += 2 {
		if i > 0 {
			kvPairs += " "
		}
		kvPairs += fmt.Sprintf("%v=%v", args[i], args[i+1])
	}

	if kvPairs != "" {
		log.Printf("%s %s %s %s", timestamp, level, msg, kvPairs)
	} else {
		log.Printf("%s %s %s", timestamp, level, msg)
	}
}

// Debug logs a debug message.
func Debug(msg string, args ...any) {
	if currentLevel <= LevelDebug {
		logMsg("DEBUG", msg, args...)
	}
}

// Info logs an info message.
func Info(msg string, args ...any) {
	if currentLevel <= LevelInfo {
		logMsg("INFO", msg, args...)
	}
}

// Warn logs a warning message.
func Warn(msg string, args ...any) {
	if currentLevel <= LevelWarn {
		logMsg("WARN", msg, args...)
	}
}

// Error logs an error message.
func Error(msg string, args ...any) {
	if currentLevel <= LevelError {
		logMsg("ERROR", msg, args...)
	}
}

// LegacyLogger is a simple logger for legacy builds.
type LegacyLogger struct {
	prefix string
}

// WithGroup returns a logger with the given group name.
func WithGroup(name string) *LegacyLogger {
	return &LegacyLogger{prefix: name}
}

// With returns a logger with the given attributes.
func With(args ...any) *LegacyLogger {
	return &LegacyLogger{}
}

// Debug logs a debug message.
func (l *LegacyLogger) Debug(msg string, args ...any) {
	if l.prefix != "" {
		msg = l.prefix + ": " + msg
	}
	Debug(msg, args...)
}

// Info logs an info message.
func (l *LegacyLogger) Info(msg string, args ...any) {
	if l.prefix != "" {
		msg = l.prefix + ": " + msg
	}
	Info(msg, args...)
}

// Warn logs a warning message.
func (l *LegacyLogger) Warn(msg string, args ...any) {
	if l.prefix != "" {
		msg = l.prefix + ": " + msg
	}
	Warn(msg, args...)
}

// Error logs an error message.
func (l *LegacyLogger) Error(msg string, args ...any) {
	if l.prefix != "" {
		msg = l.prefix + ": " + msg
	}
	Error(msg, args...)
}
