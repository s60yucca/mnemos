package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type MCPRuntimeSnapshot struct {
	Version         string `json:"version"`
	Host            string `json:"host"`
	PID             int    `json:"pid"`
	PPID            int    `json:"ppid"`
	StartedAt       string `json:"started_at"`
	UptimeSeconds   int64  `json:"uptime_seconds"`
	Executable      string `json:"executable"`
	DataDir         string `json:"data_dir"`
	CWD             string `json:"cwd"`
	ProjectID       string `json:"project_id,omitempty"`
	ProjectStrategy string `json:"project_strategy,omitempty"`
	EnvProjectID    string `json:"env_project_id,omitempty"`
}

type MCPRuntimeReport struct {
	Status      CheckStatus          `json:"status"`
	Summary     string               `json:"summary"`
	CLI         LocalRuntimeIdentity `json:"cli"`
	MCP         MCPRuntimeSnapshot   `json:"mcp,omitempty"`
	Findings    []DoctorFinding      `json:"findings,omitempty"`
	GeneratedAt string               `json:"generated_at"`
}

type LocalRuntimeIdentity struct {
	Version    string `json:"version"`
	Host       string `json:"host"`
	PID        int    `json:"pid"`
	Executable string `json:"executable"`
	DataDir    string `json:"data_dir,omitempty"`
}

func readRuntimeSnapshotArg(arg string, stdin io.Reader) (MCPRuntimeSnapshot, error) {
	var snapshot MCPRuntimeSnapshot
	arg = strings.TrimSpace(arg)
	if arg == "" {
		return snapshot, fmt.Errorf("runtime JSON is required")
	}

	var data []byte
	var err error
	switch {
	case arg == "-":
		data, err = io.ReadAll(stdin)
	case strings.HasPrefix(arg, "@"):
		data, err = os.ReadFile(strings.TrimPrefix(arg, "@"))
	case strings.HasPrefix(arg, "{"):
		data = []byte(arg)
	default:
		if _, statErr := os.Stat(arg); statErr == nil {
			data, err = os.ReadFile(arg)
		} else {
			data = []byte(arg)
		}
	}
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return snapshot, fmt.Errorf("parse runtime JSON: %w", err)
	}
	return snapshot, nil
}

func buildMCPRuntimeReport(snapshot MCPRuntimeSnapshot, buildVersion, dataDir string) MCPRuntimeReport {
	local := LocalRuntimeIdentity{
		Version: buildVersion,
		Host:    runtimeHost(),
		PID:     os.Getpid(),
		DataDir: dataDir,
	}
	if executable, err := os.Executable(); err == nil {
		local.Executable = executable
	}

	findings := validateMCPRuntime(snapshot, local)
	status := aggregateDoctorStatus(findings)
	summary := "MCP runtime identity matches this CLI runtime."
	if status == CheckWarn {
		summary = "MCP runtime identity is usable, but has mismatch warnings."
	} else if status == CheckFail {
		summary = "MCP runtime identity has a blocking mismatch."
	}
	return MCPRuntimeReport{
		Status:      status,
		Summary:     summary,
		CLI:         local,
		MCP:         snapshot,
		Findings:    findings,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func validateMCPRuntime(snapshot MCPRuntimeSnapshot, local LocalRuntimeIdentity) []DoctorFinding {
	var findings []DoctorFinding
	if snapshot.Version == "" {
		findings = append(findings, DoctorFinding{Severity: CheckFail, Message: "MCP runtime JSON is missing version."})
	} else if normalizeVersion(snapshot.Version) != normalizeVersion(local.Version) {
		findings = append(findings, DoctorFinding{
			Severity: CheckFail,
			Message:  fmt.Sprintf("MCP server version %s does not match CLI version %s.", snapshot.Version, local.Version),
		})
	} else {
		findings = append(findings, DoctorFinding{Severity: CheckPass, Message: "MCP server version matches CLI version."})
	}

	if snapshot.Host == "" {
		findings = append(findings, DoctorFinding{Severity: CheckWarn, Message: "MCP runtime JSON is missing host."})
	} else if local.Host != "" && snapshot.Host != local.Host {
		findings = append(findings, DoctorFinding{
			Severity: CheckWarn,
			Message:  fmt.Sprintf("MCP runtime host %s differs from CLI host %s.", snapshot.Host, local.Host),
		})
	} else {
		findings = append(findings, DoctorFinding{Severity: CheckPass, Message: "MCP runtime host matches this machine."})
	}

	if snapshot.PID == 0 {
		findings = append(findings, DoctorFinding{Severity: CheckWarn, Message: "MCP runtime JSON is missing pid."})
	}
	if snapshot.StartedAt == "" {
		findings = append(findings, DoctorFinding{Severity: CheckWarn, Message: "MCP runtime JSON is missing started_at."})
	} else if _, err := time.Parse(time.RFC3339, snapshot.StartedAt); err != nil {
		findings = append(findings, DoctorFinding{Severity: CheckWarn, Message: "MCP runtime started_at is not RFC3339: " + err.Error()})
	} else {
		findings = append(findings, DoctorFinding{Severity: CheckPass, Message: "MCP runtime start time is parseable."})
	}

	if snapshot.Executable == "" {
		findings = append(findings, DoctorFinding{Severity: CheckWarn, Message: "MCP runtime JSON is missing executable."})
	} else if local.Executable != "" && !sameExecutable(snapshot.Executable, local.Executable) {
		findings = append(findings, DoctorFinding{
			Severity: CheckWarn,
			Message:  fmt.Sprintf("MCP executable %s differs from CLI executable %s.", snapshot.Executable, local.Executable),
		})
	} else {
		findings = append(findings, DoctorFinding{Severity: CheckPass, Message: "MCP executable matches this CLI executable."})
	}

	return findings
}

func normalizeVersion(version string) string {
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "mnemos ")
	version = strings.TrimPrefix(version, "v")
	return version
}

func sameExecutable(a, b string) bool {
	a = resolvePathBestEffort(a)
	b = resolvePathBestEffort(b)
	return a == b
}

func resolvePathBestEffort(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

func runtimeSignalFromReport(report MCPRuntimeReport, critical bool) CheckSignal {
	value := "verified"
	if report.Status == CheckWarn {
		value = "mismatch warning"
	} else if report.Status == CheckFail {
		value = "mismatch"
	}
	explanation := report.Summary
	if len(report.Findings) > 0 {
		explanation = report.Findings[0].Message
	}
	return CheckSignal{
		Name:        "mcp runtime",
		Status:      report.Status,
		Critical:    critical,
		Scope:       "global",
		Value:       value,
		Explanation: explanation,
	}
}
