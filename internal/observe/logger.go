package observe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
)

// errorSilenced tracks whether we've already emitted a write failure warning
var errorSilenced atomic.Bool

// Feature records a feature execution event to ~/.mnemos/logs/features.log.
// Never returns an error; panics are recovered internally.
// Completes in < 5ms under normal conditions.
func Feature(name string, attrs map[string]any) {
	defer func() {
		if r := recover(); r != nil {
			if !errorSilenced.Load() {
				fmt.Fprintf(os.Stderr, "observe: panic in Feature(): %v\n", r)
				errorSilenced.Store(true)
			}
		}
	}()

	line := formatLine(name, attrs)
	logPath := getLogPath()
	write(logPath, line)
}

// formatLine converts name and attrs to TSV format with 4KB truncation
func formatLine(name string, attrs map[string]any) string {
	timestamp := time.Now().UTC().Format(time.RFC3339)

	// Build attribute string
	var attrPairs []string
	for k, v := range attrs {
		// Convert value to string, replacing newlines with spaces
		valStr := fmt.Sprintf("%v", v)
		valStr = strings.ReplaceAll(valStr, "\n", " ")
		valStr = strings.ReplaceAll(valStr, "\r", " ")
		attrPairs = append(attrPairs, fmt.Sprintf("%s=%s", k, valStr))
	}
	attrStr := strings.Join(attrPairs, " ")

	line := fmt.Sprintf("%s\t%s\t%s\n", timestamp, name, attrStr)

	// Truncate if exceeds 4KB
	const maxLineSize = 4000
	if len(line) > maxLineSize {
		truncated := line[:maxLineSize-13] + "[truncated]\n"
		return truncated
	}

	return line
}

// write appends one line to the log file with O_APPEND
func write(logPath string, line string) {
	defer func() {
		if r := recover(); r != nil {
			if !errorSilenced.Load() {
				fmt.Fprintf(os.Stderr, "observe: panic in write(): %v\n", r)
				errorSilenced.Store(true)
			}
		}
	}()

	// Create parent directory if needed
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		if !errorSilenced.Load() {
			fmt.Fprintf(os.Stderr, "observe: failed to create log directory: %v\n", err)
			errorSilenced.Store(true)
		}
		return
	}

	// Open with O_APPEND for atomic writes
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		if !errorSilenced.Load() {
			fmt.Fprintf(os.Stderr, "observe: failed to open log file: %v\n", err)
			errorSilenced.Store(true)
		}
		return
	}
	defer f.Close()

	if _, err := f.WriteString(line); err != nil {
		if !errorSilenced.Load() {
			fmt.Fprintf(os.Stderr, "observe: failed to write log line: %v\n", err)
			errorSilenced.Store(true)
		}
		return
	}
}

// getLogPath returns the path to features.log
// Can be overridden in tests
var getLogPath = func() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home dir unavailable
		return filepath.Join(".", ".mnemos", "logs", "features.log")
	}
	return filepath.Join(home, ".mnemos", "logs", "features.log")
}
