package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildMCPRuntimeReportPassesForMatchingRuntime(t *testing.T) {
	host := runtimeHost()
	executable, err := os.Executable()
	require.NoError(t, err)

	report := buildMCPRuntimeReport(MCPRuntimeSnapshot{
		Version:    "v1.1.22",
		Host:       host,
		PID:        123,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
		Executable: executable,
	}, "1.1.22", t.TempDir())

	assert.Equal(t, CheckPass, report.Status)
	assert.Contains(t, report.Findings[0].Message, "version matches")
}

func TestBuildMCPRuntimeReportFailsForVersionMismatch(t *testing.T) {
	report := buildMCPRuntimeReport(MCPRuntimeSnapshot{
		Version:    "1.1.21",
		Host:       runtimeHost(),
		PID:        123,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
		Executable: mustExecutable(t),
	}, "1.1.22", t.TempDir())

	assert.Equal(t, CheckFail, report.Status)
	assert.Contains(t, report.Findings[0].Message, "does not match")
}

func TestBuildMCPRuntimeReportWarnsForHostMismatch(t *testing.T) {
	report := buildMCPRuntimeReport(MCPRuntimeSnapshot{
		Version:    "1.1.22",
		Host:       "other-host",
		PID:        123,
		StartedAt:  time.Now().UTC().Format(time.RFC3339),
		Executable: mustExecutable(t),
	}, "1.1.22", t.TempDir())

	assert.Equal(t, CheckWarn, report.Status)
	assert.Contains(t, report.Findings[1].Message, "differs from CLI host")
}

func TestReadRuntimeSnapshotArgReadsRawJSONAndFile(t *testing.T) {
	raw := `{"version":"1.1.22","host":"test-host","pid":123}`
	snapshot, err := readRuntimeSnapshotArg(raw, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, "1.1.22", snapshot.Version)
	assert.Equal(t, "test-host", snapshot.Host)

	path := filepath.Join(t.TempDir(), "runtime.json")
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))
	snapshot, err = readRuntimeSnapshotArg("@"+path, strings.NewReader(""))
	require.NoError(t, err)
	assert.Equal(t, 123, snapshot.PID)
}

func mustExecutable(t *testing.T) string {
	t.Helper()
	executable, err := os.Executable()
	require.NoError(t, err)
	return executable
}
