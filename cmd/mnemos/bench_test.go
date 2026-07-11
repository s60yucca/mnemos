package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBenchMode_Toggle verifies that "mnemos bench mode" writes the mode file
// and emits a mode change event.
func TestBenchMode_Toggle(t *testing.T) {
	tmpDir := t.TempDir()

	// Test setting mode to ON
	cmd := newBenchModeCmd(tmpDir)
	cmd.SetArgs([]string{"on"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("failed to set mode to on: %v", err)
	}

	// Verify mode file was written
	mode, err := benchmark.ReadBenchMode(tmpDir)
	if err != nil {
		t.Fatalf("failed to read bench mode: %v", err)
	}
	if mode != benchmark.BenchModeOn {
		t.Errorf("expected mode 'on', got %q", mode)
	}

	// Test setting mode to OFF
	cmd = newBenchModeCmd(tmpDir)
	cmd.SetArgs([]string{"off"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("failed to set mode to off: %v", err)
	}

	// Verify mode file was updated
	mode, err = benchmark.ReadBenchMode(tmpDir)
	if err != nil {
		t.Fatalf("failed to read bench mode: %v", err)
	}
	if mode != benchmark.BenchModeOff {
		t.Errorf("expected mode 'off', got %q", mode)
	}
}

func TestBenchMode_Aliases(t *testing.T) {
	tmpDir := t.TempDir()
	cmd := newBenchCmd(tmpDir)
	cmd.SetArgs([]string{"off"})
	require.NoError(t, cmd.Execute())

	mode, err := benchmark.ReadBenchMode(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, benchmark.BenchModeOff, mode)

	cmd = newBenchCmd(tmpDir)
	cmd.SetArgs([]string{"on"})
	require.NoError(t, cmd.Execute())

	mode, err = benchmark.ReadBenchMode(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, benchmark.BenchModeOn, mode)
}

// TestBenchMode_InvalidMode verifies that invalid mode arguments are rejected.
func TestBenchMode_InvalidMode(t *testing.T) {
	tmpDir := t.TempDir()

	cmd := newBenchModeCmd(tmpDir)
	cmd.SetArgs([]string{"invalid"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for invalid mode, got nil")
	}
	if !strings.Contains(err.Error(), "invalid mode") {
		t.Errorf("expected 'invalid mode' error, got: %v", err)
	}
}

// TestBenchExport_EmptyLog verifies that export handles missing log files gracefully.
func TestBenchExport_EmptyLog(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a temporary home directory for the test
	oldHome := os.Getenv("HOME")
	testHome := t.TempDir()
	os.Setenv("HOME", testHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	cmd := newBenchExportCmd(tmpDir)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export should not error on missing log: %v", err)
	}
}

// TestBenchExport_ValidSessions verifies that export produces correct CSV output
// from synthetic features.log data.
func TestBenchExport_ValidSessions(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a temporary home directory with features.log
	testHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", testHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	logPath := filepath.Join(logDir, "features.log")

	// Write synthetic session events
	baseTime := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	content := baseTime.Format(time.RFC3339) + "\tbench_session_start\tsession_id=sess-1 project_id=proj-1 mode=on category=feature timestamp=" + baseTime.Format(time.RFC3339) + "\n"
	endTime := baseTime.Add(15 * time.Minute)
	content += endTime.Format(time.RFC3339) + "\tbench_session_end\tsession_id=sess-1 duration_ms=900000 tokens_in=1500 tokens_out=3200 mcp_calls_count=8 task_completed=true\n"

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write log: %v", err)
	}

	// Export to temporary file
	outputPath := filepath.Join(tmpDir, "export.csv")
	cmd := newBenchExportCmd(tmpDir)
	cmd.SetArgs([]string{"--output", outputPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	// Read and verify CSV output
	csvData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	csvStr := string(csvData)
	lines := strings.Split(strings.TrimSpace(csvStr), "\n")

	// Verify header
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines (header + 1 session), got %d", len(lines))
	}

	header := lines[0]
	expectedHeader := "session_id,timestamp_start,timestamp_end,project_id,mode,duration_ms,tokens_in,tokens_out,mcp_calls_count,task_completed,task_category,provenance,auto_inject_fired"
	if header != expectedHeader {
		t.Errorf("header mismatch:\nexpected: %s\ngot:      %s", expectedHeader, header)
	}

	// Verify session row
	row := lines[1]
	if !strings.Contains(row, "sess-1") {
		t.Errorf("expected session_id 'sess-1' in row, got: %s", row)
	}
	if !strings.Contains(row, "proj-1") {
		t.Errorf("expected project_id 'proj-1' in row, got: %s", row)
	}
	if !strings.Contains(row, "on") {
		t.Errorf("expected mode 'on' in row, got: %s", row)
	}
	if !strings.Contains(row, "900000") {
		t.Errorf("expected duration_ms '900000' in row, got: %s", row)
	}
	if !strings.Contains(row, "1500") {
		t.Errorf("expected tokens_in '1500' in row, got: %s", row)
	}
	if !strings.Contains(row, "3200") {
		t.Errorf("expected tokens_out '3200' in row, got: %s", row)
	}
	if !strings.Contains(row, "8") {
		t.Errorf("expected mcp_calls_count '8' in row, got: %s", row)
	}
	if !strings.Contains(row, "true") {
		t.Errorf("expected task_completed 'true' in row, got: %s", row)
	}
	if !strings.Contains(row, "feature") {
		t.Errorf("expected task_category 'feature' in row, got: %s", row)
	}
}

// TestBenchExport_Filtering verifies that export filters work correctly.
func TestBenchExport_Filtering(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a temporary home directory with features.log
	testHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", testHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	logPath := filepath.Join(logDir, "features.log")

	// Write multiple sessions with different modes and projects
	baseTime := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	var content string

	// Session 1: proj-1, mode=on
	content += baseTime.Format(time.RFC3339) + "\tbench_session_start\tsession_id=sess-1 project_id=proj-1 mode=on category=feature timestamp=" + baseTime.Format(time.RFC3339) + "\n"
	content += baseTime.Add(10*time.Minute).Format(time.RFC3339) + "\tbench_session_end\tsession_id=sess-1 duration_ms=600000 tokens_in=1000 tokens_out=2000 mcp_calls_count=5 task_completed=true\n"

	// Session 2: proj-2, mode=off
	baseTime2 := baseTime.Add(1 * time.Hour)
	content += baseTime2.Format(time.RFC3339) + "\tbench_session_start\tsession_id=sess-2 project_id=proj-2 mode=off category=debug timestamp=" + baseTime2.Format(time.RFC3339) + "\n"
	content += baseTime2.Add(15*time.Minute).Format(time.RFC3339) + "\tbench_session_end\tsession_id=sess-2 duration_ms=900000 tokens_in=1500 tokens_out=3000 mcp_calls_count=7 task_completed=true\n"

	// Session 3: proj-1, mode=off
	baseTime3 := baseTime.Add(2 * time.Hour)
	content += baseTime3.Format(time.RFC3339) + "\tbench_session_start\tsession_id=sess-3 project_id=proj-1 mode=off category=refactor timestamp=" + baseTime3.Format(time.RFC3339) + "\n"
	content += baseTime3.Add(20*time.Minute).Format(time.RFC3339) + "\tbench_session_end\tsession_id=sess-3 duration_ms=1200000 tokens_in=2000 tokens_out=4000 mcp_calls_count=10 task_completed=true\n"

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write log: %v", err)
	}

	// Test project filter
	outputPath := filepath.Join(tmpDir, "export_proj1.csv")
	cmd := newBenchExportCmd(tmpDir)
	cmd.SetArgs([]string{"--project", "proj-1", "--output", outputPath})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	csvData, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	lines := strings.Split(strings.TrimSpace(string(csvData)), "\n")
	// Should have header + 2 sessions (sess-1 and sess-3)
	if len(lines) != 3 {
		t.Errorf("expected 3 lines (header + 2 sessions), got %d", len(lines))
	}

	// Test mode filter
	outputPath2 := filepath.Join(tmpDir, "export_off.csv")
	cmd = newBenchExportCmd(tmpDir)
	cmd.SetArgs([]string{"--mode", "off", "--output", outputPath2})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	csvData2, err := os.ReadFile(outputPath2)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	lines2 := strings.Split(strings.TrimSpace(string(csvData2)), "\n")
	// Should have header + 2 sessions (sess-2 and sess-3)
	if len(lines2) != 3 {
		t.Errorf("expected 3 lines (header + 2 sessions), got %d", len(lines2))
	}
}

func TestExtractSessions_UsesFinalModeAndAutoInjectEvidence(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "features.log")
	start := time.Date(2026, 4, 15, 10, 0, 0, 0, time.UTC)
	content := start.Format(time.RFC3339) + "\tbench_session_start\tsession_id=sess-mixed project_id=proj-1 mode=on category=feature timestamp=" + start.Format(time.RFC3339) + " provenance=production\n"
	content += start.Add(time.Minute).Format(time.RFC3339) + "\tauto_inject\tproject_id=proj-1 outcome=ok payload=true\n"
	content += start.Add(2*time.Minute).Format(time.RFC3339) + "\tbench_session_end\tsession_id=sess-mixed mode=mode_mixed duration_ms=120000 tokens_in=100 tokens_out=200 mcp_calls_count=2 task_completed=true\n"
	require.NoError(t, os.WriteFile(logPath, []byte(content), 0o644))

	excluded, err := extractSessions(logPath, time.Time{}, "", "", false)
	require.NoError(t, err)
	assert.Empty(t, excluded)

	included, err := extractSessions(logPath, time.Time{}, "", "", true)
	require.NoError(t, err)
	require.Len(t, included, 1)
	assert.Equal(t, "mode_mixed", included[0].Mode)
	assert.True(t, included[0].AutoInjectFired)
	assert.Equal(t, "production", included[0].Provenance)
}

// TestBenchStatus_CurrentMode verifies that status displays the current mode.
func TestBenchStatus_CurrentMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Set mode to OFF
	if err := benchmark.WriteBenchMode(tmpDir, benchmark.BenchModeOff); err != nil {
		t.Fatalf("failed to write bench mode: %v", err)
	}

	// Create a temporary home directory (no log file needed for mode display)
	testHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", testHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	// Capture output using a pipe
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBenchStatusCmd(tmpDir)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var output strings.Builder
	io.Copy(&output, r)
	out := output.String()

	if !strings.Contains(out, "Current benchmark mode: OFF") {
		t.Errorf("expected 'Current benchmark mode: OFF' in output, got:\n%s", out)
	}
}

// TestBenchStatus_SessionCounts verifies that status displays session counts.
func TestBenchStatus_SessionCounts(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a temporary home directory with features.log
	testHome := t.TempDir()
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", testHome)
	t.Cleanup(func() { os.Setenv("HOME", oldHome) })

	logDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("failed to create log dir: %v", err)
	}
	logPath := filepath.Join(logDir, "features.log")

	// Write sessions within last 7 days
	now := time.Now().UTC()
	var content string

	// 2 ON sessions
	for i := 0; i < 2; i++ {
		sessionTime := now.Add(-time.Duration(i) * 24 * time.Hour)
		sessionID := "sess-on-" + string(rune('1'+i))
		content += sessionTime.Format(time.RFC3339) + "\tbench_session_start\tsession_id=" + sessionID + " project_id=proj-1 mode=on category=feature timestamp=" + sessionTime.Format(time.RFC3339) + "\n"
		content += sessionTime.Add(10*time.Minute).Format(time.RFC3339) + "\tbench_session_end\tsession_id=" + sessionID + " duration_ms=600000 tokens_in=1000 tokens_out=2000 mcp_calls_count=5 task_completed=true\n"
	}

	// 3 OFF sessions
	for i := 0; i < 3; i++ {
		sessionTime := now.Add(-time.Duration(i+2) * 24 * time.Hour)
		sessionID := "sess-off-" + string(rune('1'+i))
		content += sessionTime.Format(time.RFC3339) + "\tbench_session_start\tsession_id=" + sessionID + " project_id=proj-1 mode=off category=debug timestamp=" + sessionTime.Format(time.RFC3339) + "\n"
		content += sessionTime.Add(15*time.Minute).Format(time.RFC3339) + "\tbench_session_end\tsession_id=" + sessionID + " duration_ms=900000 tokens_in=1500 tokens_out=3000 mcp_calls_count=7 task_completed=true\n"
	}

	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write log: %v", err)
	}

	// Capture output using a pipe
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBenchStatusCmd(tmpDir)
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var output strings.Builder
	io.Copy(&output, r)
	out := output.String()

	if !strings.Contains(out, "ON:  2") {
		t.Errorf("expected 'ON:  2' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "OFF: 3") {
		t.Errorf("expected 'OFF: 3' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "Total: 5") {
		t.Errorf("expected 'Total: 5' in output, got:\n%s", out)
	}
}

// TestBenchSessionStart_EventEmission verifies that session-start emits an event.
func TestBenchSessionStart_EventEmission(t *testing.T) {
	tmpDir := t.TempDir()
	observe.SetDataDir(tmpDir)
	t.Cleanup(func() { observe.SetLogPath("") })

	// Capture output using a pipe
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBenchSessionStartCmd(tmpDir)
	cmd.SetArgs([]string{"--category", "refactor", "--project", "manual-project", "--session-id", "manual-test"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("session-start failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var output strings.Builder
	io.Copy(&output, r)
	out := output.String()

	if !strings.Contains(out, "Session started:") {
		t.Errorf("expected 'Session started:' in output, got:\n%s", out)
	}
	state, err := readManualBenchSessionState(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, "manual-test", state.SessionID)
	assert.Equal(t, "manual-project", state.ProjectID)
}

// TestBenchSessionEnd_EventEmission verifies that session-end emits an event.
func TestBenchSessionEnd_EventEmission(t *testing.T) {
	tmpDir := t.TempDir()
	observe.SetDataDir(tmpDir)
	t.Cleanup(func() { observe.SetLogPath("") })
	require.NoError(t, benchmark.WriteBenchMode(tmpDir, benchmark.BenchModeOff))
	startCmd := newBenchSessionStartCmd(tmpDir)
	startCmd.SetArgs([]string{"--category", "debug", "--project", "manual-project", "--session-id", "manual-test"})
	require.NoError(t, startCmd.Execute())

	// Capture output using a pipe
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	cmd := newBenchSessionEndCmd(tmpDir)
	cmd.SetArgs([]string{})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("session-end failed: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var output strings.Builder
	io.Copy(&output, r)
	out := output.String()

	if !strings.Contains(out, "Session ended: manual-test") {
		t.Errorf("expected 'Session ended: manual-test' in output, got:\n%s", out)
	}
	_, err := os.Stat(manualBenchSessionStatePath(tmpDir))
	assert.True(t, os.IsNotExist(err))
	sessions, err := extractSessions(benchmarkLogPath(tmpDir), time.Now().UTC().Add(-time.Hour), "", "off", false)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "manual-test", sessions[0].SessionID)
	assert.Equal(t, "off", sessions[0].Mode)
	assert.Equal(t, "production", sessions[0].Provenance)
}
