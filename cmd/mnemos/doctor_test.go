package main

import (
	"strings"
	"testing"

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
		Status:     CheckWarn,
		Summary:    "runtime needs attention",
		Version:    "test",
		DataDir:    "/tmp/mnemos",
		DBPath:     "/tmp/mnemos/mnemos.db",
		ServeCount: 1,
		Processes:  []DoctorProcess{{PID: 10, PPID: 1, Elapsed: "01:00", Command: "mnemos serve"}},
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
	assert.Contains(t, rendered, "pid=10")
	assert.Contains(t, rendered, "leader_running: true")
	assert.Contains(t, rendered, "[WARN] sample warning")
}
