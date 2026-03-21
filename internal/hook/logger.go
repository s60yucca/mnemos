package hook

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// NewHookLogger creates a logger that writes JSON to both stderr and a log file.
// Log file path: .mnemos/logs/hooks.log if .mnemos/ exists in projectDir,
// otherwise falls back to ~/.mnemos/logs/hooks.log.
// If the log file cannot be opened, falls back to stderr-only logging.
func NewHookLogger(projectDir string, logLevel string) *slog.Logger {
	lvl := parseLogLevel(logLevel)
	opts := &slog.HandlerOptions{Level: lvl}

	logFile := resolveLogFilePath(projectDir)
	w := openLogWriter(logFile)

	handler := slog.NewJSONHandler(w, opts)
	return slog.New(handler)
}

// parseLogLevel converts a string log level to slog.Level.
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// resolveLogFilePath returns the log file path based on whether .mnemos/ exists in projectDir.
func resolveLogFilePath(projectDir string) string {
	if projectDir != "" {
		mnemosDir := filepath.Join(projectDir, ".mnemos")
		if info, err := os.Stat(mnemosDir); err == nil && info.IsDir() {
			return filepath.Join(mnemosDir, "logs", "hooks.log")
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".mnemos", "logs", "hooks.log")
}

// openLogWriter opens the log file for appending, creating the directory if needed.
// Falls back to stderr-only if the file cannot be opened.
func openLogWriter(logFilePath string) io.Writer {
	if logFilePath == "" {
		return os.Stderr
	}

	if err := os.MkdirAll(filepath.Dir(logFilePath), 0o755); err != nil {
		return os.Stderr
	}

	f, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return os.Stderr
	}

	return io.MultiWriter(os.Stderr, f)
}
