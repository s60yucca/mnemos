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
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/spf13/cobra"
)

type DoctorReport struct {
	Status      CheckStatus          `json:"status"`
	Summary     string               `json:"summary"`
	Version     string               `json:"version"`
	DataDir     string               `json:"data_dir"`
	DBPath      string               `json:"db_path"`
	Processes   DoctorProcessReport  `json:"processes"`
	Database    DoctorDatabaseReport `json:"database"`
	Logs        DoctorLogReport      `json:"logs"`
	Findings    []DoctorFinding      `json:"findings,omitempty"`
	GeneratedAt string               `json:"generated_at"`
}

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

type DoctorDatabaseReport struct {
	Status      CheckStatus     `json:"status"`
	Path        string          `json:"path"`
	Exists      bool            `json:"exists"`
	SizeBytes   int64           `json:"size_bytes,omitempty"`
	Findings    []DoctorFinding `json:"findings,omitempty"`
	GeneratedAt string          `json:"generated_at"`
}

type DoctorLogReport struct {
	Status         CheckStatus     `json:"status"`
	ConfiguredPath string          `json:"configured_path"`
	LegacyPath     string          `json:"legacy_path,omitempty"`
	ActivePath     string          `json:"active_path"`
	Exists         bool            `json:"exists"`
	SizeBytes      int64           `json:"size_bytes,omitempty"`
	LastWrite      string          `json:"last_write,omitempty"`
	DirWritable    bool            `json:"dir_writable"`
	LegacyFallback bool            `json:"legacy_fallback"`
	Findings       []DoctorFinding `json:"findings,omitempty"`
	GeneratedAt    string          `json:"generated_at"`
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
	cmd.AddCommand(newDoctorAllCmd(cfg, buildVersion))
	cmd.AddCommand(newDoctorDatabaseCmd(cfg))
	cmd.AddCommand(newDoctorLogsCmd(cfg))
	cmd.AddCommand(newDoctorProcessesCmd(cfg, buildVersion))
	return cmd
}

func newDoctorAllCmd(cfg *config.Config, buildVersion string) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "all",
		Short: "Inspect Mnemos processes, autopilot leadership, and database writability",
		RunE: func(cmd *cobra.Command, args []string) error {
			report, err := buildDoctorReport(cfg, buildVersion)
			if err != nil {
				return err
			}
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			renderDoctorReport(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return cmd
}

func newDoctorDatabaseCmd(cfg *config.Config) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "database",
		Short: "Inspect Mnemos database existence and writability",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := buildDoctorDatabaseReport(cfg.DBPath())
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			renderDoctorDatabaseReport(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
	return cmd
}

func newDoctorLogsCmd(cfg *config.Config) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Inspect Mnemos feature log path, writability, and freshness",
		RunE: func(cmd *cobra.Command, args []string) error {
			report := buildDoctorLogReport(cfg.DataDir)
			if asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(report)
			}
			renderDoctorLogReport(cmd.OutOrStdout(), report)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output JSON")
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

func buildDoctorReport(cfg *config.Config, buildVersion string) (DoctorReport, error) {
	processes := buildDoctorProcessReportTolerant(cfg, buildVersion)
	database := buildDoctorDatabaseReport(cfg.DBPath())
	logs := buildDoctorLogReport(cfg.DataDir)
	findings := append([]DoctorFinding{}, processes.Findings...)
	findings = append(findings, database.Findings...)
	findings = append(findings, logs.Findings...)
	status := aggregateDoctorStatus(findings)
	summary := "Mnemos runtime, database, and logs look healthy."
	if status == CheckWarn {
		summary = "Mnemos is usable, but doctor found degraded runtime, database, or log signals."
	} else if status == CheckFail {
		summary = "Mnemos has a blocking runtime, database, or log issue."
	}
	return DoctorReport{
		Status:      status,
		Summary:     summary,
		Version:     buildVersion,
		DataDir:     cfg.DataDir,
		DBPath:      cfg.DBPath(),
		Processes:   processes,
		Database:    database,
		Logs:        logs,
		Findings:    findings,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}, nil
}

func buildDoctorProcessReportTolerant(cfg *config.Config, buildVersion string) DoctorProcessReport {
	report, err := buildDoctorProcessReport(cfg, buildVersion)
	if err == nil {
		return report
	}
	finding := DoctorFinding{
		Severity: CheckUnknown,
		Message:  "Process inspection unavailable: " + err.Error(),
	}
	return DoctorProcessReport{
		Status:      CheckUnknown,
		Summary:     "Mnemos process state could not be inspected in this environment.",
		Version:     buildVersion,
		DataDir:     cfg.DataDir,
		DBPath:      cfg.DBPath(),
		Autopilot:   readDoctorAutopilot(cfg.DataDir, nil),
		Findings:    []DoctorFinding{finding},
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
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

func buildDoctorDatabaseReport(dbPath string) DoctorDatabaseReport {
	report := DoctorDatabaseReport{
		Status:      CheckPass,
		Path:        dbPath,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}
	info, err := os.Stat(dbPath)
	if err != nil {
		report.Status = CheckFail
		if os.IsNotExist(err) {
			report.Findings = append(report.Findings, DoctorFinding{Severity: CheckFail, Message: "Database file does not exist."})
		} else {
			report.Findings = append(report.Findings, DoctorFinding{Severity: CheckFail, Message: "Database stat failed: " + err.Error()})
		}
		return report
	}
	report.Exists = true
	report.SizeBytes = info.Size()
	if err := probeDatabaseWritable(dbPath); err != nil {
		report.Status = CheckFail
		report.Findings = append(report.Findings, DoctorFinding{Severity: CheckFail, Message: "Database writable probe failed: " + err.Error()})
		return report
	}
	report.Findings = append(report.Findings, DoctorFinding{Severity: CheckPass, Message: "Database accepts a rolled-back write transaction."})
	return report
}

func buildDoctorLogReport(dataDir string) DoctorLogReport {
	configured := filepath.Join(dataDir, "logs", "features.log")
	home, _ := os.UserHomeDir()
	legacy := filepath.Join(home, ".mnemos", "logs", "features.log")
	activePath, _, legacyFallback := resolveFeatureLog(dataDir)
	report := DoctorLogReport{
		Status:         CheckPass,
		ConfiguredPath: configured,
		LegacyPath:     legacy,
		ActivePath:     activePath,
		LegacyFallback: legacyFallback,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if legacyFallback {
		report.Findings = append(report.Findings, DoctorFinding{Severity: CheckWarn, Message: "Configured feature log is missing and Mnemos is reading legacy telemetry."})
	}
	logDir := filepath.Dir(configured)
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		report.Findings = append(report.Findings, DoctorFinding{Severity: CheckFail, Message: "Feature log directory cannot be created: " + err.Error()})
	} else if err := probeDirectoryWritable(logDir); err != nil {
		report.Findings = append(report.Findings, DoctorFinding{Severity: CheckFail, Message: "Feature log directory is not writable: " + err.Error()})
	} else {
		report.DirWritable = true
		report.Findings = append(report.Findings, DoctorFinding{Severity: CheckPass, Message: "Feature log directory is writable."})
	}
	info, err := os.Stat(activePath)
	if err != nil {
		if os.IsNotExist(err) {
			report.Findings = append(report.Findings, DoctorFinding{Severity: CheckWarn, Message: "Feature log does not exist yet."})
		} else {
			report.Findings = append(report.Findings, DoctorFinding{Severity: CheckFail, Message: "Feature log stat failed: " + err.Error()})
		}
	} else {
		report.Exists = true
		report.SizeBytes = info.Size()
		report.LastWrite = info.ModTime().UTC().Format(time.RFC3339)
		age := time.Since(info.ModTime())
		if age > 24*time.Hour {
			report.Findings = append(report.Findings, DoctorFinding{Severity: CheckWarn, Message: fmt.Sprintf("Feature log has not been written for %s.", age.Round(time.Hour))})
		} else {
			report.Findings = append(report.Findings, DoctorFinding{Severity: CheckPass, Message: fmt.Sprintf("Feature log was written %s ago.", age.Round(time.Minute))})
		}
	}
	report.Status = aggregateDoctorStatus(report.Findings)
	return report
}

func probeDirectoryWritable(dir string) error {
	name := filepath.Join(dir, fmt.Sprintf(".mnemos-doctor-log-probe-%d", time.Now().UTC().UnixNano()))
	if err := os.WriteFile(name, []byte("ok\n"), 0o600); err != nil {
		return err
	}
	return os.Remove(name)
}

func probeDatabaseWritable(dbPath string) error {
	db, err := sqlitestore.OpenExistingWritable(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	tableName := fmt.Sprintf("mnemos_doctor_write_probe_%d", time.Now().UTC().UnixNano())
	if _, err := tx.Exec(`CREATE TABLE ` + tableName + ` (id INTEGER PRIMARY KEY, checked_at TEXT NOT NULL)`); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO `+tableName+` (checked_at) VALUES (?)`, time.Now().UTC().Format(time.RFC3339Nano)); err != nil {
		return err
	}
	return nil
}

func aggregateDoctorStatus(findings []DoctorFinding) CheckStatus {
	status := CheckPass
	for _, finding := range findings {
		if statusRank(finding.Severity) > statusRank(status) {
			status = finding.Severity
		}
	}
	return status
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

func renderDoctorReport(w interface{ Write([]byte) (int, error) }, report DoctorReport) {
	fmt.Fprintf(w, "Mnemos Doctor - %s\n%s\n\n", report.Status, report.Summary)
	fmt.Fprintf(w, "Version:  %s\n", report.Version)
	fmt.Fprintf(w, "Data dir: %s\n", report.DataDir)
	fmt.Fprintf(w, "DB path:  %s\n\n", report.DBPath)
	fmt.Fprintf(w, "Processes: %s (%d mnemos serve)\n", report.Processes.Status, report.Processes.ServeCount)
	fmt.Fprintf(w, "Database:  %s", report.Database.Status)
	if report.Database.Exists {
		fmt.Fprintf(w, " (%d bytes)", report.Database.SizeBytes)
	}
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Logs:      %s", report.Logs.Status)
	if report.Logs.Exists {
		fmt.Fprintf(w, " (%d bytes)", report.Logs.SizeBytes)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Findings:")
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "  [%s] %s\n", finding.Severity, finding.Message)
	}
}

func renderDoctorDatabaseReport(w interface{ Write([]byte) (int, error) }, report DoctorDatabaseReport) {
	fmt.Fprintf(w, "Mnemos Doctor Database - %s\n", report.Status)
	fmt.Fprintf(w, "Path:   %s\n", report.Path)
	fmt.Fprintf(w, "Exists: %t\n", report.Exists)
	if report.Exists {
		fmt.Fprintf(w, "Size:   %d bytes\n", report.SizeBytes)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Findings:")
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "  [%s] %s\n", finding.Severity, finding.Message)
	}
}

func renderDoctorLogReport(w interface{ Write([]byte) (int, error) }, report DoctorLogReport) {
	fmt.Fprintf(w, "Mnemos Doctor Logs - %s\n", report.Status)
	fmt.Fprintf(w, "Configured: %s\n", report.ConfiguredPath)
	fmt.Fprintf(w, "Active:     %s\n", report.ActivePath)
	if report.LegacyPath != "" {
		fmt.Fprintf(w, "Legacy:     %s\n", report.LegacyPath)
	}
	fmt.Fprintf(w, "Exists:     %t\n", report.Exists)
	fmt.Fprintf(w, "Dir writable: %t\n", report.DirWritable)
	if report.Exists {
		fmt.Fprintf(w, "Size:       %d bytes\n", report.SizeBytes)
		fmt.Fprintf(w, "Last write: %s\n", report.LastWrite)
	}
	if report.LegacyFallback {
		fmt.Fprintln(w, "Legacy fallback: true")
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Findings:")
	for _, finding := range report.Findings {
		fmt.Fprintf(w, "  [%s] %s\n", finding.Severity, finding.Message)
	}
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
