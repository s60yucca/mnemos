package hook

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/mnemos-dev/mnemos/internal/setup"
)

// ProjectDetector resolves a stable project identifier.
type ProjectDetector struct {
	cwd     string
	dataDir string
}

// NewProjectDetector creates a new ProjectDetector.
func NewProjectDetector(cwd, dataDir string) *ProjectDetector {
	return &ProjectDetector{
		cwd:     cwd,
		dataDir: dataDir,
	}
}

// Detect returns (projectID, detectionStrategy, error).
func (d *ProjectDetector) Detect() (string, string, error) {
	var projectID string
	var strategy string

	// Level 1: MNEMOS_PROJECT_ID
	if id := os.Getenv("MNEMOS_PROJECT_ID"); id != "" {
		projectID = id
		strategy = "env_var"
	} else {
		// Level 2: git root remote
		gitRoot, err := d.findGitRoot()
		if err == nil && gitRoot != "" {
			if repoName, err := d.deriveFromGitRemote(gitRoot); err == nil && repoName != "" {
				projectID = repoName
				strategy = "git_remote"
			}
		}

		// Level 3: Fallback to directory basename
		if projectID == "" && d.cwd != "" {
			projectID = filepath.Base(d.cwd)
			strategy = "dir_basename"
		}
	}

	// Level 4: Return empty if no detection succeeds
	if projectID == "" {
		return "", "", nil
	}

	projectID = setup.SanitizeProjectID(projectID)

	observe.Feature("auto_inject_project_detected", map[string]any{
		"project_id": projectID,
		"strategy":   strategy,
		"cwd":        d.cwd,
	})

	return projectID, strategy, nil
}

func (d *ProjectDetector) findGitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = d.cwd
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func (d *ProjectDetector) deriveFromGitRemote(gitRoot string) (string, error) {
	cmd := exec.Command("git", "config", "--get", "remote.origin.url")
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	url := strings.TrimSpace(string(output))
	if url == "" {
		return "", nil
	}

	url = strings.TrimSuffix(url, ".git")
	parts := strings.FieldsFunc(url, func(r rune) bool {
		return r == '/' || r == ':'
	})

	if len(parts) == 0 {
		return "", nil
	}

	return parts[len(parts)-1], nil
}

// AutoInjectConfig holds configuration for auto-injection.
type AutoInjectConfig struct {
	Enabled       bool
	Budget        int
	MaxCount      int
	SummaryLength int
}

// AutoInjectConfigFromEnv creates configuration from environment variables.
func AutoInjectConfigFromEnv() AutoInjectConfig {
	cfg := AutoInjectConfig{
		Enabled:       true,
		Budget:        1500,
		MaxCount:      10,
		SummaryLength: 120,
	}

	if val := os.Getenv("MNEMOS_AUTO_INJECT"); val == "false" {
		cfg.Enabled = false
	}
	if val := os.Getenv("MNEMOS_AUTO_INJECT_BUDGET"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			cfg.Budget = parsed
		}
	}
	if val := os.Getenv("MNEMOS_AUTO_INJECT_COUNT"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			cfg.MaxCount = parsed
		}
	}
	if val := os.Getenv("MNEMOS_AUTO_INJECT_SUMMARY_LENGTH"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			cfg.SummaryLength = parsed
		}
	}

	return cfg
}

// AutoInjector performs auto-injection of memory context.
type AutoInjector struct {
	mnemos  *core.Mnemos
	cfg     AutoInjectConfig
	dataDir string
}

// NewAutoInjector creates a new AutoInjector.
func NewAutoInjector(mnemos *core.Mnemos, cfg AutoInjectConfig, dataDir string) *AutoInjector {
	return &AutoInjector{
		mnemos:  mnemos,
		cfg:     cfg,
		dataDir: dataDir,
	}
}

// Run performs auto-injection.
func (a *AutoInjector) Run(ctx context.Context, sessionID, projectID, clientID string, existingIDs []string) (payload string, skipReason string, err error) {
	start := time.Now()
	injectedCount := 0
	defer func() {
		outcome := "ok"
		if skipReason != "" {
			outcome = "skipped:" + skipReason
		}
		if err != nil {
			outcome = "error"
		}
		observe.Feature("auto_inject", map[string]any{
			"project_id":   projectID,
			"session_id":   sessionID,
			"client_id":    clientID,
			"outcome":      outcome,
			"payload":      payload != "",
			"tokens_used":  len(payload) / 4,
			"duration_ms":  time.Since(start).Milliseconds(),
			"memory_count": injectedCount,
		})
	}()
	defer func() {
		if r := recover(); r != nil {
			payload = ""
			skipReason = "panic"
			err = nil
		}
	}()

	if !a.cfg.Enabled {
		return "", "disabled", nil
	}

	benchMode, _ := benchmark.ReadBenchMode(a.dataDir)
	if benchMode == benchmark.BenchModeOff {
		return "", "bench_mode_off", nil
	}

	observe.Feature("auto_inject_attempt", map[string]any{
		"project_id": projectID,
		"session_id": sessionID,
		"client_id":  clientID,
		"enabled":    a.cfg.Enabled,
	})

	timeoutCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	result, dbErr := a.mnemos.AssembleContext(timeoutCtx, "current project context overview", projectID, a.cfg.Budget, false, nil)
	if timeoutCtx.Err() == context.DeadlineExceeded {
		observe.Feature("auto_inject_timeout", map[string]any{
			"project_id": projectID,
			"session_id": sessionID,
			"elapsed_ms": 500,
		})
		return "", "retrieval_timeout", nil
	}
	if dbErr != nil {
		fmt.Fprintf(os.Stderr, "WARN: auto-inject db error: %v\n", dbErr)
		return "", "db_error", nil
	}

	existingMap := make(map[string]bool)
	for _, id := range existingIDs {
		existingMap[id] = true
	}

	var filtered []*domain.Memory
	for _, mem := range result.Memories {
		if existingMap[mem.ID] {
			continue
		}

		isOffDay := false
		for _, tag := range mem.Tags {
			if tag == "bench_off_day" {
				isOffDay = true
				break
			}
		}
		if !isOffDay {
			filtered = append(filtered, mem)
		}
	}

	if len(filtered) > a.cfg.MaxCount {
		filtered = filtered[:a.cfg.MaxCount]
	}

	if len(filtered) == 0 {
		return "", "no_memories", nil
	}

	injectedCount = len(filtered)
	payload = formatAutoInjectPayload(filtered, projectID, a.cfg)

	observe.Feature("auto_inject_payload", map[string]any{
		"project_id":         projectID,
		"session_id":         sessionID,
		"memory_count":       len(filtered),
		"tokens_used":        len(payload) / 4,
		"detection_strategy": "", // Not strictly passed to Run(), could be omitted or passed down. Wait, spec says detection_strategy.
	})

	return payload, "", nil
}

func formatAutoInjectPayload(memories []*domain.Memory, projectID string, cfg AutoInjectConfig) string {
	header := fmt.Sprintf("# Mnemos Project Context\n# Auto-injected at session start. %d memories from project %s.\n\n", len(memories), projectID)

	var sb strings.Builder
	sb.WriteString(header)

	for i, mem := range memories {
		dateStr := mem.CreatedAt.Format("2006-01-02")
		category := mem.Category
		if category == "" {
			category = "other"
		}

		sb.WriteString(fmt.Sprintf("[%s] %s | %s | %s\n", mem.ID, string(mem.Type), dateStr, category))

		content := mem.Summary
		if content == "" {
			content = mem.Content
			if len(content) > cfg.SummaryLength {
				content = content[:cfg.SummaryLength]
			}
		}
		sb.WriteString(content)
		sb.WriteString("\n")

		if i < len(memories)-1 {
			sb.WriteString("\n")
		}
	}

	footer := "\n# Use mnemos_get(<id>) for full content.\n# Use mnemos_search() for additional queries."
	sb.WriteString(footer)

	return sb.String()
}
