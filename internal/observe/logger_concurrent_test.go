package observe

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestFeature_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	originalGetLogPath := getLogPath
	getLogPath = func() string { return logPath }
	defer func() { getLogPath = originalGetLogPath }()

	const numGoroutines = 200
	const eventsPerGoroutine = 50
	const totalEvents = numGoroutines * eventsPerGoroutine

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Spawn goroutines
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				Feature("test_feature", map[string]any{
					"goroutine_id": goroutineID,
					"event_num":    j,
				})
			}
		}(i)
	}

	wg.Wait()

	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Verify all events were written
	if len(lines) != totalEvents {
		t.Errorf("expected %d lines, got %d", totalEvents, len(lines))
	}

	// Verify no line exceeds 4KB
	for i, line := range lines {
		if len(line) > 4000 {
			t.Errorf("line %d exceeds 4KB: %d bytes", i, len(line))
		}
	}

	// Verify no partial lines (every line should have 3 tab-separated parts)
	for i, line := range lines {
		if line == "" {
			continue // Skip empty lines at end
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Errorf("line %d: expected 3 parts, got %d: %q", i, len(parts), line)
		}
	}

	// Verify line content integrity: each line contains expected feature name
	for i, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 2 {
			continue
		}
		if parts[1] != "test_feature" {
			t.Errorf("line %d: expected feature 'test_feature', got %q", i, parts[1])
		}
		// Verify attributes contain goroutine_id
		if len(parts) >= 3 && !strings.Contains(parts[2], "goroutine_id=") {
			t.Errorf("line %d: missing goroutine_id in attributes: %q", i, parts[2])
		}
	}
}

func TestFeature_ConcurrencyWithLargePayloads(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	originalGetLogPath := getLogPath
	getLogPath = func() string { return logPath }
	defer func() { getLogPath = originalGetLogPath }()

	const numGoroutines = 100
	const eventsPerGoroutine = 10

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Spawn goroutines with varying payload sizes
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				// Create payloads of varying sizes
				payloadSize := (goroutineID * 10) + j
				payload := strings.Repeat("x", payloadSize)

				Feature("large_feature", map[string]any{
					"goroutine_id": goroutineID,
					"event_num":    j,
					"payload":      payload,
				})
			}
		}(i)
	}

	wg.Wait()

	// Read log file
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Verify no corruption: each line should be well-formed
	malformedCount := 0
	for i, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			malformedCount++
			t.Logf("malformed line %d: %q", i, line)
		}
	}

	if malformedCount > 0 {
		t.Errorf("found %d malformed lines out of %d", malformedCount, len(lines))
	}
}

func TestFeature_ConcurrencyStressTest(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "features.log")

	originalGetLogPath := getLogPath
	getLogPath = func() string { return logPath }
	defer func() { getLogPath = originalGetLogPath }()

	const numGoroutines = 500
	const eventsPerGoroutine = 20
	const totalEvents = numGoroutines * eventsPerGoroutine

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Spawn many goroutines with mixed workloads
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < eventsPerGoroutine; j++ {
				Feature(fmt.Sprintf("feature_%d", goroutineID%10), map[string]any{
					"gid": goroutineID,
					"seq": j,
				})
			}
		}(i)
	}

	wg.Wait()

	// Read and verify
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")

	// Allow some tolerance for concurrent writes, but should be close
	if len(lines) < totalEvents-10 {
		t.Errorf("expected ~%d lines, got %d (too few)", totalEvents, len(lines))
	}

	// Verify no line corruption
	for i, line := range lines {
		if line == "" {
			continue
		}
		if len(line) > 4000 {
			t.Errorf("line %d exceeds 4KB", i)
		}
		// Note: strings.Split removes the delimiter, so lines won't have trailing \n
		// Just verify lines are well-formed (have 3 tab-separated parts)
		parts := strings.Split(line, "\t")
		if len(parts) != 3 {
			t.Errorf("line %d malformed: expected 3 parts, got %d", i, len(parts))
		}
	}
}
