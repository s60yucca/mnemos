package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/autopilot"
	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/spf13/cobra"
)

type DoctorProcessReport struct {
	Status      CheckStatus     `json:"status"`
	Summary     string          `json:"summary"`
	Version     string          `json:"version"`
	DataDir     string          `json:"data_dir"`
	DBPath      string          `json:"db_path"`
	ServeCount  int             `json:"serve_count"`
	Processes   []DoctorProcess `json:"processes"`
	Autopilot   DoctorAutopilot `json:"autopilot"`
	Findings    []DoctorFinding `json:"findings,omitempty"`
	GeneratedAt string          `json:"generated_at"`
}

type DoctorProcess struct {
	PID     int    `json:"pid"`
	PPID    int    `json:"ppid"`
	Elapsed string `json:"elapsed"`
	Command string `json:"command"`
}

type DoctorAutopilot struct {
	StatePID      int    `json:"state_pid,omitempty"`
	StateUpdated  string `json:"state_updated,omitempty"`
	LockPID       int    `json:"lock_pid,omitempty"`
	LeaderRunning bool   `json:"leader_running"`
	LockPath      string `json:"lock_path"`
	StatePath     string `json:"state_path"`
}

type DoctorFinding struct {
	Severity CheckStatus `json:"severity"`
	Message  string      `json:"message"`
}

func newDoctorCmd(cfg *config.Config, buildVersion string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Diagnose Mnemos runtime state",
	}
	cmd.AddCommand(newDoctorProcessesCmd(cfg, buildVersion))
	return cmd
}

func newDoctorProcessesCmd(cfg *config.Config, buildVersion string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "processes",
		Short: "Inspect mnemos serve processes and autopilot leadership",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := buildDoctorProcessReport(cfg, buildVersion)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			renderDoctorProcessReport(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return cmd
}

func buildDoctorProcessReport(cfg *config.Config, buildVersion string) (DoctorProcessReport, error) {
	processes, err := listMnemosServeProcesses()
	if err != nil {
		return DoctorProcessReport{}, err
	}
	autopilotInfo := readDoctorAutopilot(cfg.DataDir, processes)
	findings := diagnoseProcesses(processes, autopilotInfo)
	status := CheckPass
	for _, finding := range findings {
		if finding.Severity == CheckFail {
			status = CheckFail
			break
		}
		if finding.Severity == CheckWarn && status != CheckFail {
			status = CheckWarn
		}
	}
	summary := "Mnemos runtime process state looks healthy."
	if status == CheckWarn {
		summary = "Mnemos runtime is usable, but process state needs attention."
	} else if status == CheckFail {
		summary = "Mnemos runtime has a blocking process or autopilot issue."
	}
	return DoctorProcessReport{
		Status:      status,
		Summary:     summary,
		Version:     buildVersion,
		DataDir:     cfg.DataDir,
		DBPath:      cfg.DBPath(),
		ServeCount:  len(processes),
		Processes:   processes,
		Autopilot:   autopilotInfo,
		Findings:    findings,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func listMnemosServeProcesses() ([]DoctorProcess, error) {
	out, err := exec.Command("ps", "-axo", "pid,ppid,etime,command").Output()
	if err != nil {
		return nil, fmt.Errorf("list processes: %w", err)
	}
	return parseMnemosServeProcesses(string(out)), nil
}

func parseMnemosServeProcesses(output string) []DoctorProcess {
	var processes []DoctorProcess
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PID ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		command := strings.Join(fields[3:], " ")
		if !isMnemosServeCommand(command) {
			continue
		}
		pid, err1 := strconv.Atoi(fields[0])
		ppid, err2 := strconv.Atoi(fields[1])
		if err1 != nil || err2 != nil {
			continue
		}
		processes = append(processes, DoctorProcess{
			PID:     pid,
			PPID:    ppid,
			Elapsed: fields[2],
			Command: command,
		})
	}
	return processes
}

func isMnemosServeCommand(command string) bool {
	fields := strings.Fields(command)
	if len(fields) < 2 {
		return false
	}
	bin := filepath.Base(fields[0])
	return bin == "mnemos" && fields[1] == "serve"
}

func readDoctorAutopilot(dataDir string, processes []DoctorProcess) DoctorAutopilot {
	info := DoctorAutopilot{
		LockPath:  filepath.Join(dataDir, "autopilot.lock"),
		StatePath: filepath.Join(dataDir, "autopilot-state.json"),
	}
	if status, err := autopilot.ReadStateFile(dataDir); err == nil && status != nil {
		info.StatePID = status.PID
		if !status.UpdatedAt.IsZero() {
			info.StateUpdated = status.UpdatedAt.Format(time.RFC3339)
		}
	}
	if data, err := os.ReadFile(info.LockPath); err == nil {
		if pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil {
			info.LockPID = pid
			info.LeaderRunning = processExists(processes, pid)
		}
	}
	return info
}

func processExists(processes []DoctorProcess, pid int) bool {
	for _, process := range processes {
		if process.PID == pid {
			return true
		}
	}
	return false
}

func diagnoseProcesses(processes []DoctorProcess, autopilotInfo DoctorAutopilot) []DoctorFinding {
	var findings []DoctorFinding
	switch len(processes) {
	case 0:
		findings = append(findings, DoctorFinding{
			Severity: CheckWarn,
			Message:  "No mnemos serve process is running. This is expected if no MCP client is currently using Mnemos.",
		})
	case 1:
		findings = append(findings, DoctorFinding{
			Severity: CheckPass,
			Message:  "One mnemos serve process is running.",
		})
	default:
		findings = append(findings, DoctorFinding{
			Severity: CheckWarn,
			Message:  fmt.Sprintf("%d mnemos serve processes are running. This can be normal with multiple MCP client sessions, but only one should own autopilot.", len(processes)),
		})
	}
	if autopilotInfo.LockPID == 0 {
		findings = append(findings, DoctorFinding{
			Severity: CheckWarn,
			Message:  "No autopilot leader lock was found. The daemon may not have started yet, or no serve process has leadership.",
		})
	} else if !autopilotInfo.LeaderRunning {
		findings = append(findings, DoctorFinding{
			Severity: CheckFail,
			Message:  fmt.Sprintf("Autopilot lock points at PID %d, but that PID is not a running mnemos serve process.", autopilotInfo.LockPID),
		})
	} else {
		findings = append(findings, DoctorFinding{
			Severity: CheckPass,
			Message:  fmt.Sprintf("Autopilot leader lock is owned by running PID %d.", autopilotInfo.LockPID),
		})
	}
	if autopilotInfo.StatePID != 0 && !processExists(processes, autopilotInfo.StatePID) {
		findings = append(findings, DoctorFinding{
			Severity: CheckWarn,
			Message:  fmt.Sprintf("Autopilot state was last written by PID %d, which is not currently running. State may be stale until the next cycle.", autopilotInfo.StatePID),
		})
	}
	return findings
}

func renderDoctorProcessReport(w interface{ Write([]byte) (int, error) }, report DoctorProcessReport) {
	fmt.Fprintf(w, "Mnemos Doctor - %s\n%s\n\n", report.Status, report.Summary)
	fmt.Fprintf(w, "Version:  %s\n", report.Version)
	fmt.Fprintf(w, "Data dir: %s\n", report.DataDir)
	fmt.Fprintf(w, "DB path:  %s\n\n", report.DBPath)
	fmt.Fprintf(w, "mnemos serve processes: %d\n", report.ServeCount)
	if len(report.Processes) == 0 {
		fmt.Fprintln(w, "  (none)")
	} else {
		for _, process := range report.Processes {
			fmt.Fprintf(w, "  pid=%d ppid=%d age=%s cmd=%s\n", process.PID, process.PPID, process.Elapsed, process.Command)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Autopilot:")
	fmt.Fprintf(w, "  lock_path:  %s\n", report.Autopilot.LockPath)
	if report.Autopilot.LockPID == 0 {
		fmt.Fprintln(w, "  lock_pid:   (none)")
	} else {
		fmt.Fprintf(w, "  lock_pid:   %d\n", report.Autopilot.LockPID)
	}
	fmt.Fprintf(w, "  leader_running: %t\n", report.Autopilot.LeaderRunning)
	fmt.Fprintf(w, "  state_path: %s\n", report.Autopilot.StatePath)
	if report.Autopilot.StatePID == 0 {
		fmt.Fprintln(w, "  state_pid:  (none)")
	} else {
		fmt.Fprintf(w, "  state_pid:  %d\n", report.Autopilot.StatePID)
	}
	if report.Autopilot.StateUpdated != "" {
		fmt.Fprintf(w, "  state_updated: %s\n", report.Autopilot.StateUpdated)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Findings:")
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "  [%s] %s\n", finding.Severity, finding.Message)
	}
}
