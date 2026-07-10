package mcp

import (
	"os"
	"time"

	"github.com/mnemos-dev/mnemos/internal/hook"
)

type RuntimeInfo struct {
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

func (s *Server) runtimeInfo() RuntimeInfo {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "unknown"
	}
	executable, err := os.Executable()
	if err != nil {
		executable = ""
	}
	cwd, err := os.Getwd()
	if err != nil {
		cwd = ""
	}
	projectID, strategy, _ := hook.NewProjectDetector(cwd, s.dataDir).Detect()

	return RuntimeInfo{
		Version:         s.version,
		Host:            host,
		PID:             os.Getpid(),
		PPID:            os.Getppid(),
		StartedAt:       s.startedAt.Format(time.RFC3339),
		UptimeSeconds:   int64(time.Since(s.startedAt).Seconds()),
		Executable:      executable,
		DataDir:         s.dataDir,
		CWD:             cwd,
		ProjectID:       projectID,
		ProjectStrategy: strategy,
		EnvProjectID:    os.Getenv("MNEMOS_PROJECT_ID"),
	}
}
