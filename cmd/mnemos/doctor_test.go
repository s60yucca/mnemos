package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseMnemosServeProcesses(t *testing.T) {
	output := `  PID  PPID ELAPSED COMMAND
  100     1   01:02 /opt/homebrew/bin/mnemos serve
  101     1   00:05 mnemos check
  102     1   10:00 /Applications/Codex.app/Contents/MacOS/Codex
  103   102 1-02:03:04 mnemos serve --config /tmp/config.yaml
`

	processes := parseMnemosServeProcesses(output)

	require.Len(t, processes, 2)
	assert.Equal(t, 100, processes[0].PID)
	assert.Equal(t, 1, processes[0].PPID)
	assert.Equal(t, "01:02", processes[0].Elapsed)
	assert.Equal(t, "/opt/homebrew/bin/mnemos serve", processes[0].Command)
	assert.Equal(t, 103, processes[1].PID)
	assert.Equal(t, "mnemos serve --config /tmp/config.yaml", processes[1].Command)
	assert.Equal(t, "/Applications/Codex.app/Contents/MacOS/Codex", processes[1].ParentCommand)
}

func TestIsMnemosServeCommand(t *testing.T) {
	tests := []struct {
		command string
		want    bool
	}{
		{"mnemos serve", true},
		{"/opt/homebrew/bin/mnemos serve", true},
		{"mnemos serve --config /tmp/config.yaml", true},
		{"mnemos check", false},
		{"go run ./cmd/mnemos serve", false},
		{"notmnemos serve", false},
		{"mnemos", false},
	}

	for _, tt := range tests {
		t.Run(tt.command, func(t *testing.T) {
			assert.Equal(t, tt.want, isMnemosServeCommand(tt.command))
		})
	}
}

func TestDiagnoseProcesses_MultipleProcessesOneLeader(t *testing.T) {
	processes := []DoctorProcess{
		{PID: 10, PPID: 1, Elapsed: "01:00", Command: "mnemos serve"},
		{PID: 11, PPID: 1, Elapsed: "00:30", Command: "mnemos serve"},
	}
	info := DoctorAutopilot{LockPID: 10, LeaderRunning: true}

	findings := diagnoseProcesses(processes, info)

	assert.Equal(t, CheckWarn, findings[0].Severity)
	assert.Contains(t, findings[0].Message, "2 mnemos serve")
	assert.Equal(t, CheckPass, findings[1].Severity)
	assert.Contains(t, findings[1].Message, "PID 10")
}

func TestDiagnoseProcesses_SharedParentWarnsAboutRepeatedSpawn(t *testing.T) {
	processes := []DoctorProcess{
		{PID: 10, PPID: 2179, Elapsed: "11:00", Command: "mnemos serve", ParentCommand: "/Applications/ChatGPT.app/Contents/MacOS/ChatGPT"},
		{PID: 11, PPID: 2179, Elapsed: "10:00", Command: "mnemos serve", ParentCommand: "/Applications/ChatGPT.app/Contents/MacOS/ChatGPT"},
		{PID: 12, PPID: 2179, Elapsed: "09:00", Command: "mnemos serve", ParentCommand: "/Applications/ChatGPT.app/Contents/MacOS/ChatGPT"},
	}
	info := DoctorAutopilot{LockPID: 10, LeaderRunning: true}

	findings := diagnoseProcesses(processes, info)

	var found bool
	for _, finding := range findings {
		if strings.Contains(finding.Message, "share parent PID 2179") &&
			strings.Contains(finding.Message, "repeated servers") {
			found = true
		}
	}
	assert.True(t, found)
}

func TestDiagnoseProcesses_StaleLeaderLockFails(t *testing.T) {
	processes := []DoctorProcess{{PID: 10, PPID: 1, Elapsed: "01:00", Command: "mnemos serve"}}
	info := DoctorAutopilot{LockPID: 99, LeaderRunning: false}

	findings := diagnoseProcesses(processes, info)

	var foundFail bool
	for _, finding := range findings {
		if finding.Severity == CheckFail && strings.Contains(finding.Message, "PID 99") {
			foundFail = true
		}
	}
	assert.True(t, foundFail)
}

func TestRenderDoctorProcessReport(t *testing.T) {
	report := DoctorProcessReport{
		Status:      CheckWarn,
		Summary:     "runtime needs attention",
		Version:     "test",
		Host:        "test-host",
		ReporterPID: 1234,
		DataDir:     "/tmp/mnemos",
		DBPath:      "/tmp/mnemos/mnemos.db",
		ServeCount:  1,
		Processes:   []DoctorProcess{{PID: 10, PPID: 1, Elapsed: "01:00", Command: "mnemos serve", ParentCommand: "launchd"}},
		Autopilot: DoctorAutopilot{
			LockPID:       10,
			LeaderRunning: true,
			LockPath:      "/tmp/mnemos/autopilot.lock",
			StatePath:     "/tmp/mnemos/autopilot-state.json",
		},
		Findings: []DoctorFinding{{Severity: CheckWarn, Message: "sample warning"}},
	}
	var out strings.Builder

	renderDoctorProcessReport(&out, report)

	rendered := out.String()
	assert.Contains(t, rendered, "Mnemos Doctor - WARN")
	assert.Contains(t, rendered, "Host:     test-host")
	assert.Contains(t, rendered, "Reporter: pid=1234")
	assert.Contains(t, rendered, "pid=10")
	assert.Contains(t, rendered, "parent=launchd")
	assert.Contains(t, rendered, "leader_running: true")
	assert.Contains(t, rendered, "[WARN] sample warning")
}

func TestBuildDoctorDatabaseReportPassesForWritableDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mnemos.db")
	db, err := sqlitestore.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	report := buildDoctorDatabaseReport(dbPath)

	assert.Equal(t, CheckPass, report.Status)
	assert.True(t, report.Exists)
	assert.NotZero(t, report.SizeBytes)
	require.NotEmpty(t, report.Findings)
	assert.Equal(t, CheckPass, report.Findings[0].Severity)
}

func TestBuildDoctorDatabaseReportFailsForMissingDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "missing.db")

	report := buildDoctorDatabaseReport(dbPath)

	assert.Equal(t, CheckFail, report.Status)
	assert.False(t, report.Exists)
	require.NotEmpty(t, report.Findings)
	assert.Contains(t, report.Findings[0].Message, "does not exist")
	_, err := os.Stat(dbPath)
	assert.True(t, os.IsNotExist(err))
}

func TestRenderDoctorReport(t *testing.T) {
	report := DoctorReport{
		Status:      CheckFail,
		Summary:     "blocking issue",
		Version:     "test",
		Host:        "test-host",
		ReporterPID: 5678,
		DataDir:     "/tmp/mnemos",
		DBPath:      "/tmp/mnemos/mnemos.db",
		Processes: DoctorProcessReport{
			Status:     CheckWarn,
			ServeCount: 2,
		},
		Database: DoctorDatabaseReport{
			Status:    CheckFail,
			Exists:    true,
			SizeBytes: 123,
		},
		Logs: DoctorLogReport{
			Status:    CheckPass,
			Exists:    true,
			SizeBytes: 789,
		},
		Findings: []DoctorFinding{{Severity: CheckFail, Message: "database read-only"}},
	}
	var out strings.Builder

	renderDoctorReport(&out, report)

	rendered := out.String()
	assert.Contains(t, rendered, "Mnemos Doctor - FAIL")
	assert.Contains(t, rendered, "Host:     test-host")
	assert.Contains(t, rendered, "Reporter: pid=5678")
	assert.Contains(t, rendered, "Processes: WARN (2 mnemos serve)")
	assert.Contains(t, rendered, "Database:  FAIL (123 bytes)")
	assert.Contains(t, rendered, "Logs:      PASS (789 bytes)")
	assert.Contains(t, rendered, "[FAIL] database read-only")
}

func TestRenderDoctorDatabaseReport(t *testing.T) {
	report := DoctorDatabaseReport{
		Status:    CheckPass,
		Path:      "/tmp/mnemos/mnemos.db",
		Exists:    true,
		SizeBytes: 456,
		Findings:  []DoctorFinding{{Severity: CheckPass, Message: "writable"}},
	}
	var out strings.Builder

	renderDoctorDatabaseReport(&out, report)

	rendered := out.String()
	assert.Contains(t, rendered, "Mnemos Doctor Database - PASS")
	assert.Contains(t, rendered, "Path:   /tmp/mnemos/mnemos.db")
	assert.Contains(t, rendered, "Size:   456 bytes")
	assert.Contains(t, rendered, "[PASS] writable")
}

func TestBuildDoctorLogReportPassesForWritableRecentLog(t *testing.T) {
	dataDir := t.TempDir()
	logDir := filepath.Join(dataDir, "logs")
	require.NoError(t, os.MkdirAll(logDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(logDir, "features.log"), []byte("feature=store_call\n"), 0o600))

	report := buildDoctorLogReport(dataDir)

	assert.Equal(t, CheckPass, report.Status)
	assert.True(t, report.Exists)
	assert.True(t, report.DirWritable)
	assert.False(t, report.LegacyFallback)
	assert.NotEmpty(t, report.LastWrite)
}

func TestRenderDoctorLogReport(t *testing.T) {
	report := DoctorLogReport{
		Status:         CheckWarn,
		ConfiguredPath: "/tmp/mnemos/logs/features.log",
		ActivePath:     "/tmp/.mnemos/logs/features.log",
		LegacyPath:     "/tmp/.mnemos/logs/features.log",
		Exists:         true,
		SizeBytes:      99,
		LastWrite:      "2026-07-05T00:00:00Z",
		DirWritable:    true,
		LegacyFallback: true,
		Findings:       []DoctorFinding{{Severity: CheckWarn, Message: "legacy fallback"}},
	}
	var out strings.Builder

	renderDoctorLogReport(&out, report)

	rendered := out.String()
	assert.Contains(t, rendered, "Mnemos Doctor Logs - WARN")
	assert.Contains(t, rendered, "Configured: /tmp/mnemos/logs/features.log")
	assert.Contains(t, rendered, "Legacy fallback: true")
	assert.Contains(t, rendered, "[WARN] legacy fallback")
}
