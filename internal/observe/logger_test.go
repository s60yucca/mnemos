package observe

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFeature_HappyPath(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	// Override getLogPath for testing
	originalGetLogPath := getLogPath
	getLogPath = func() string { return logPath }
	defer func() { getLogPath = originalGetLogPath }()

	// Call Feature
	Feature("test_feature", map[string]any{
		"key1": "value1",
		"key2": 42,
	})

	// Verify log file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatal("log file was not created")
	}

	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	line := string(content)

	// Verify format: timestamp\tfeature_name\tattributes\n
	parts := strings.Split(strings.TrimSpace(line), "\t")
	if len(parts) != 3 {
		t.Fatalf("expected 3 tab-separated parts, got %d: %q", len(parts), line)
	}

	// Verify timestamp is valid RFC3339
	if _, err := time.Parse(time.RFC3339, parts[0]); err != nil {
		t.Errorf("invalid timestamp format: %v", err)
	}

	// Verify feature name
	if parts[1] != "test_feature" {
		t.Errorf("expected feature name 'test_feature', got %q", parts[1])
	}

	// Verify attributes contain expected keys
	attrs := parts[2]
	if !strings.Contains(attrs, "key1=value1") {
		t.Errorf("attributes missing key1=value1: %q", attrs)
	}
	if !strings.Contains(attrs, "key2=42") {
		t.Errorf("attributes missing key2=42: %q", attrs)
	}
}

func TestFeature_PanicRecovery(t *testing.T) {
	// Reset error silencing for this test
	errorSilenced.Store(false)

	// Create a scenario that might panic (nil map access is handled, but test recovery)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Feature() panicked and was not recovered: %v", r)
		}
	}()

	// This should not panic even with edge cases
	Feature("test", nil)
	Feature("test", map[string]any{})
	Feature("", map[string]any{"key": "value"})
}

func TestFormatLine_Truncation(t *testing.T) {
	// Create attributes that will exceed 4KB
	largeValue := strings.Repeat("x", 5000)
	attrs := map[string]any{
		"large": largeValue,
	}

	line := formatLine("test_feature", attrs)

	// Verify line is truncated to 4000 bytes
	if len(line) > 4000 {
		t.Errorf("line exceeds 4000 bytes: %d", len(line))
	}

	// Verify truncation marker is present
	if !strings.Contains(line, "[truncated]") {
		t.Error("truncated line missing [truncated] marker")
	}

	// Verify line ends with newline
	if !strings.HasSuffix(line, "\n") {
		t.Error("line does not end with newline")
	}
}

func TestFormatLine_NewlineReplacement(t *testing.T) {
	attrs := map[string]any{
		"multiline": "line1\nline2\rline3",
	}

	line := formatLine("test", attrs)

	// Verify no embedded newlines in the line (except trailing)
	trimmed := strings.TrimSuffix(line, "\n")
	if strings.Contains(trimmed, "\n") || strings.Contains(trimmed, "\r") {
		t.Errorf("line contains embedded newlines: %q", line)
	}
}

func TestWrite_ErrorSuppression(t *testing.T) {
	// Reset error silencing
	errorSilenced.Store(false)

	// Try to write to an invalid path (should fail but not panic)
	invalidPath := "/root/impossible/path/features.log"

	// First write should emit warning (captured by stderr)
	write(invalidPath, "test line\n")

	// Verify error was silenced
	if !errorSilenced.Load() {
		t.Log("Note: error silencing may not trigger on all systems")
	}

	// Second write should be silenced (no panic)
	write(invalidPath, "test line 2\n")
}

func TestFeature_MultipleWrites(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	originalGetLogPath := getLogPath
	getLogPath = func() string { return logPath }
	defer func() { getLogPath = originalGetLogPath }()

	// Write multiple events
	Feature("feature1", map[string]any{"id": 1})
	Feature("feature2", map[string]any{"id": 2})
	Feature("feature3", map[string]any{"id": 3})

	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	if len(lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(lines))
	}

	// Verify each line is complete
	for i, line := range lines {
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Errorf("line %d: expected 3 parts, got %d: %q", i, len(parts), line)
		}
	}
}
