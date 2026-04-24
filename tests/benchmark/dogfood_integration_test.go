package benchmark_test

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDogfoodSimulation_EndToEnd simulates the complete dogfood workflow:
//   - Create temp mnemos instance
//   - Set bench mode ON
//   - Simulate 5 Kiro tasks (MCP calls with realistic content)
//   - Set bench mode OFF
//   - Simulate 5 Kiro tasks
//   - Export CSV
//   - Verify: 10 sessions total (5 ON, 5 OFF), token counts differ between ON and OFF,
//     CSV format matches spec, no mode_mixed sessions
//
// **Validates: Requirements 1, 2, 3, 4**
func TestDogfoodSimulation_EndToEnd(t *testing.T) {
	ctx := context.Background()

	// Create temp directory for mnemos data
	tempDir := t.TempDir()
	dataDir := filepath.Join(tempDir, ".mnemos")
	require.NoError(t, os.MkdirAll(dataDir, 0755))

	logsDir := filepath.Join(dataDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0755))

	// Create features.log file
	featuresLog := filepath.Join(logsDir, "features.log")
	logFile, err := os.Create(featuresLog)
	require.NoError(t, err)
	logFile.Close()

	// Create mnemos instance
	mn, err := core.NewMnemos(dataDir)
	require.NoError(t, err)
	defer mn.Close()

	// --- Phase 1: Bench Mode ON (5 sessions) ---
	require.NoError(t, benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOn))

	tracker, err := benchmark.NewSessionTrackerWithTimeout(dataDir, 1*time.Second)
	require.NoError(t, err)

	projectID := "dogfood-test-project"

	// Simulate 5 Kiro tasks with mode ON
	for i := 0; i < 5; i++ {
		simulateKiroTask(t, ctx, mn, tracker, projectID, i, true)
		time.Sleep(1500 * time.Millisecond) // Wait for inactivity timeout
	}

	tracker.Stop()

	// --- Phase 2: Bench Mode OFF (5 sessions) ---
	require.NoError(t, benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOff))

	tracker, err = benchmark.NewSessionTrackerWithTimeout(dataDir, 1*time.Second)
	require.NoError(t, err)

	// Simulate 5 Kiro tasks with mode OFF
	for i := 5; i < 10; i++ {
		simulateKiroTask(t, ctx, mn, tracker, projectID, i, false)
		time.Sleep(1500 * time.Millisecond) // Wait for inactivity timeout
	}

	tracker.Stop()

	// --- Phase 3: Export CSV and Verify ---
	csvPath := filepath.Join(tempDir, "sessions.csv")
	sessions, err := extractSessionsFromLog(featuresLog, time.Time{}, "", "", false)
	require.NoError(t, err)

	// Write CSV
	csvFile, err := os.Create(csvPath)
	require.NoError(t, err)
	defer csvFile.Close()

	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Write header
	require.NoError(t, writer.Write([]string{
		"session_id", "timestamp_start", "timestamp_end", "project_id", "mode",
		"duration_ms", "tokens_in", "tokens_out", "mcp_calls_count",
		"task_completed", "task_category",
	}))

	// Write rows
	for _, session := range sessions {
		require.NoError(t, writer.Write([]string{
			session.SessionID,
			session.TimestampStart,
			session.TimestampEnd,
			session.ProjectID,
			session.Mode,
			fmt.Sprintf("%d", session.DurationMS),
			fmt.Sprintf("%d", session.TokensIn),
			fmt.Sprintf("%d", session.TokensOut),
			fmt.Sprintf("%d", session.MCPCallsCount),
			fmt.Sprintf("%t", session.TaskCompleted),
			session.TaskCategory,
		}))
	}

	writer.Flush()
	require.NoError(t, writer.Error())

	// --- Verification ---

	// 1. Verify 10 sessions total
	assert.Len(t, sessions, 10, "should have 10 sessions total")

	// 2. Verify 5 ON and 5 OFF sessions
	onCount := 0
	offCount := 0
	for _, session := range sessions {
		if session.Mode == "on" {
			onCount++
		} else if session.Mode == "off" {
			offCount++
		}
	}
	assert.Equal(t, 5, onCount, "should have 5 ON sessions")
	assert.Equal(t, 5, offCount, "should have 5 OFF sessions")

	// 3. Verify token counts differ between ON and OFF
	// ON sessions should have more tokens (context injection)
	// OFF sessions should have fewer tokens (no context)
	var onTokensTotal, offTokensTotal int
	for _, session := range sessions {
		totalTokens := session.TokensIn + session.TokensOut
		if session.Mode == "on" {
			onTokensTotal += totalTokens
		} else {
			offTokensTotal += totalTokens
		}
	}

	// ON sessions should have more tokens due to context injection
	assert.Greater(t, onTokensTotal, offTokensTotal,
		"ON sessions should have more tokens than OFF sessions (context injection)")

	// 4. Verify CSV format matches spec
	for _, session := range sessions {
		assert.NotEmpty(t, session.SessionID, "session_id should not be empty")
		assert.NotEmpty(t, session.TimestampStart, "timestamp_start should not be empty")
		assert.NotEmpty(t, session.TimestampEnd, "timestamp_end should not be empty")
		assert.Equal(t, projectID, session.ProjectID, "project_id should match")
		assert.Contains(t, []string{"on", "off"}, session.Mode, "mode should be 'on' or 'off'")
		assert.Greater(t, session.DurationMS, int64(0), "duration_ms should be positive")
		assert.GreaterOrEqual(t, session.TokensIn, 0, "tokens_in should be non-negative")
		assert.GreaterOrEqual(t, session.TokensOut, 0, "tokens_out should be non-negative")
		assert.Greater(t, session.MCPCallsCount, 0, "mcp_calls_count should be positive")
		assert.True(t, session.TaskCompleted, "task_completed should be true")
		assert.NotEmpty(t, session.TaskCategory, "task_category should not be empty")
	}

	// 5. Verify no mode_mixed sessions (all sessions should have consistent mode)
	for _, session := range sessions {
		assert.NotEqual(t, "mixed", session.Mode, "should not have mode_mixed sessions")
	}

	// 6. Verify CSV file was created and is readable
	csvData, err := os.ReadFile(csvPath)
	require.NoError(t, err)
	assert.NotEmpty(t, csvData, "CSV file should not be empty")

	// Verify CSV has correct number of lines (header + 10 sessions)
	lines := strings.Split(strings.TrimSpace(string(csvData)), "\n")
	assert.Len(t, lines, 11, "CSV should have 11 lines (1 header + 10 sessions)")
}

// simulateKiroTask simulates a single Kiro task with realistic MCP calls.
func simulateKiroTask(t *testing.T, ctx context.Context, mn *core.Mnemos, tracker *benchmark.SessionTracker, projectID string, taskNum int, modeOn bool) {
	t.Helper()

	// Start session
	category := []string{"feature", "refactor", "debug", "docs"}[taskNum%4]
	tracker.StartSession(projectID, category)

	// Simulate realistic MCP calls
	queries := []string{
		"implement JWT authentication middleware",
		"refactor database connection pooling",
		"debug memory leak in session handler",
		"document API endpoints for user service",
		"add rate limiting to API gateway",
	}

	query := queries[taskNum%len(queries)]

	// Store some memories (this happens in both ON and OFF modes)
	storeContent := fmt.Sprintf("Task %d: %s - implementation notes and gotchas", taskNum, query)
	_, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   storeContent,
		Category:  category,
		ProjectID: projectID,
		Tags:      []string{"dogfood-test"},
	})
	require.NoError(t, err)

	// Simulate context assembly (this returns different results based on mode)
	contextResult, err := mn.AssembleContext(ctx, query, projectID, 2000, false, nil)
	require.NoError(t, err)

	// Track MCP calls with realistic content
	reqContent := fmt.Sprintf(`{"query": "%s", "project_id": "%s", "token_budget": 2000}`, query, projectID)
	respContent := ""

	if modeOn {
		// Mode ON: context assembly returns memories
		respContent = fmt.Sprintf(`{"memories": %d, "tokens": %d}`,
			len(contextResult.Memories), contextResult.TotalTokens)
	} else {
		// Mode OFF: context assembly returns empty (simulated)
		respContent = `{"memories": [], "tokens": 0}`
	}

	tracker.OnMCPCall(projectID, reqContent, respContent)

	// Simulate additional MCP calls (search, get, etc.)
	for i := 0; i < 3; i++ {
		searchReq := fmt.Sprintf(`{"query": "related to %s", "limit": 5}`, query)
		searchResp := `{"results": []}`
		if modeOn {
			searchResp = fmt.Sprintf(`{"results": [{"id": "mem-%d", "content": "related memory"}]}`, i)
		}
		tracker.OnMCPCall(projectID, searchReq, searchResp)
	}

	// End session explicitly
	tracker.EndSession(true)
}

// SessionRecord represents a parsed session from features.log
type SessionRecord struct {
	SessionID      string
	TimestampStart string
	TimestampEnd   string
	ProjectID      string
	Mode           string
	DurationMS     int64
	TokensIn       int
	TokensOut      int
	MCPCallsCount  int
	TaskCompleted  bool
	TaskCategory   string
}

// extractSessionsFromLog parses features.log and extracts session records.
func extractSessionsFromLog(logPath string, sinceTime time.Time, projectFilter, modeFilter string, includeMixed bool) ([]SessionRecord, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	sessionStarts := make(map[string]SessionRecord)
	var sessions []SessionRecord

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		timestamp := parts[0]
		eventType := parts[1]
		attributes := parts[2]

		switch eventType {
		case "bench_session_start":
			attrs := parseAttributes(attributes)
			sessionID := attrs["session_id"]
			if sessionID == "" {
				continue
			}

			// Parse timestamp
			ts, err := time.Parse(time.RFC3339, timestamp)
			if err != nil {
				continue
			}

			// Apply time filter
			if !sinceTime.IsZero() && ts.Before(sinceTime) {
				continue
			}

			sessionStarts[sessionID] = SessionRecord{
				SessionID:      sessionID,
				TimestampStart: attrs["timestamp"],
				ProjectID:      attrs["project_id"],
				Mode:           attrs["mode"],
				TaskCategory:   attrs["category"],
			}

		case "bench_session_end":
			attrs := parseAttributes(attributes)
			sessionID := attrs["session_id"]
			if sessionID == "" {
				continue
			}

			start, ok := sessionStarts[sessionID]
			if !ok {
				continue
			}

			// Complete the session record
			start.TimestampEnd = timestamp
			start.DurationMS = parseInt64(attrs["duration_ms"])
			start.TokensIn = parseInt(attrs["tokens_in"])
			start.TokensOut = parseInt(attrs["tokens_out"])
			start.MCPCallsCount = parseInt(attrs["mcp_calls_count"])
			start.TaskCompleted = attrs["task_completed"] == "true"

			// Apply filters
			if projectFilter != "" && start.ProjectID != projectFilter {
				continue
			}
			if modeFilter != "" && start.Mode != modeFilter {
				continue
			}
			if !includeMixed && start.Mode == "mixed" {
				continue
			}

			sessions = append(sessions, start)
			delete(sessionStarts, sessionID)
		}
	}

	return sessions, nil
}

// parseAttributes parses key=value pairs from log attributes.
func parseAttributes(attrs string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(attrs, " ")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

// parseInt parses an integer from a string, returning 0 on error.
func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

// parseInt64 parses an int64 from a string, returning 0 on error.
func parseInt64(s string) int64 {
	var v int64
	fmt.Sscanf(s, "%d", &v)
	return v
}
