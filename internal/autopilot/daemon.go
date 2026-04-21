package autopilot

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

// reportWriter is the interface the daemon uses to persist findings.
// The real ReportWriter (Task 5) will implement this interface.
type reportWriter interface {
	Write(ctx context.Context, projectID string, findings []Finding) error
}

// noopWriter satisfies reportWriter but does nothing. Used when no writer is wired yet.
type noopWriter struct{}

func (noopWriter) Write(_ context.Context, _ string, _ []Finding) error { return nil }

// DaemonStatus holds a snapshot of the daemon's current state.
// It is also the on-disk format written to autopilot-state.json so that
// `mnemos autopilot status` can read it from outside the running process.
type DaemonStatus struct {
	Enabled          bool                 `json:"enabled"`
	LastRun          map[string]time.Time `json:"last_run"`
	LastFindingCount int                  `json:"last_finding_count"`
	NextRun          time.Time            `json:"next_run"`
	UpdatedAt        time.Time            `json:"updated_at"` // when this snapshot was written
	PID              int                  `json:"pid"`        // PID of the process that wrote it
}

// AutopilotDaemon runs the detector pipeline on a timer.
type AutopilotDaemon struct {
	mnemos    *core.Mnemos
	cfg       config.AutopilotConfig
	dataDir   string // ~/.mnemos — for persisting state
	logger    *slog.Logger
	detectors []Detector
	writer    reportWriter
	stopCh    chan struct{}
	doneCh    chan struct{}

	mu               sync.Mutex
	lastRun          map[string]time.Time // per-project, in-process only
	lastFindingCount int
	nextRun          time.Time
}

// NewAutopilotDaemon constructs an AutopilotDaemon. If writer is nil a no-op writer is used.
func NewAutopilotDaemon(
	mnemos *core.Mnemos,
	cfg config.AutopilotConfig,
	dataDir string,
	logger *slog.Logger,
	writer reportWriter,
) *AutopilotDaemon {
	if writer == nil {
		writer = noopWriter{}
	}
	return &AutopilotDaemon{
		mnemos:  mnemos,
		cfg:     cfg,
		dataDir: dataDir,
		logger:  logger,
		detectors: []Detector{
			NewStalenessDetector(mnemos, cfg),
			NewRelationDetector(mnemos, cfg),
			NewContradictionDetector(mnemos, cfg),
			NewSummaryBackfillDetector(mnemos, cfg, logger),
		},
		writer:  writer,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
		lastRun: make(map[string]time.Time),
	}
}

// Start launches the daemon goroutine. Safe to call once.
func (d *AutopilotDaemon) Start() {
	if !d.cfg.Enabled {
		d.logger.Info("autopilot daemon disabled")
		close(d.doneCh)
		return
	}

	go func() {
		defer close(d.doneCh)

		ctx := context.Background()

		// Initial delay before first run.
		select {
		case <-time.After(d.cfg.InitialDelay):
		case <-d.stopCh:
			return
		}

		d.runCycle(ctx)

		// Recurring interval.
		for {
			d.mu.Lock()
			d.nextRun = time.Now().Add(d.cfg.Interval)
			d.mu.Unlock()

			select {
			case <-time.After(d.cfg.Interval):
				d.runCycle(ctx)
			case <-d.stopCh:
				return
			}
		}
	}()
}

// Stop signals the daemon to stop and waits up to 5 seconds.
func (d *AutopilotDaemon) Stop() {
	close(d.stopCh)
	select {
	case <-d.doneCh:
	case <-time.After(5 * time.Second):
		d.logger.Warn("autopilot daemon stop timeout")
	}
}

// Status returns a snapshot of the daemon's current state.
func (d *AutopilotDaemon) Status() DaemonStatus {
	d.mu.Lock()
	defer d.mu.Unlock()

	lastRunCopy := make(map[string]time.Time, len(d.lastRun))
	for k, v := range d.lastRun {
		lastRunCopy[k] = v
	}
	return DaemonStatus{
		Enabled:          d.cfg.Enabled,
		LastRun:          lastRunCopy,
		LastFindingCount: d.lastFindingCount,
		NextRun:          d.nextRun,
		UpdatedAt:        time.Now(),
		PID:              os.Getpid(),
	}
}

// statePath returns the path to the on-disk state file.
func (d *AutopilotDaemon) statePath() string {
	return filepath.Join(d.dataDir, "autopilot-state.json")
}

// persistState writes the current daemon status to disk atomically.
// Called after every runCycle so `mnemos autopilot status` can read it
// from outside the running process.
func (d *AutopilotDaemon) persistState() {
	s := d.Status()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		d.logger.Warn("autopilot: failed to marshal state", "err", err)
		return
	}
	path := d.statePath()
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		d.logger.Warn("autopilot: failed to write state file", "err", err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		d.logger.Warn("autopilot: failed to rename state file", "err", err)
		os.Remove(tmp)
	}
}

// ReadStateFile reads the persisted daemon status from disk.
// Returns nil if the file does not exist (daemon has never run).
// Used by `mnemos autopilot status` to show real state across processes.
func ReadStateFile(dataDir string) (*DaemonStatus, error) {
	path := filepath.Join(dataDir, "autopilot-state.json")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var s DaemonStatus
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

// RunOnce executes the full detector pipeline for a single project synchronously.
// dryRun=true skips writing the report and creating relations.
func (d *AutopilotDaemon) RunOnce(ctx context.Context, projectID string, dryRun bool) ([]Finding, error) {
	memories, err := d.mnemos.List(ctx, storage.ListQuery{
		ProjectID:         projectID,
		Statuses:          []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:             d.cfg.MaxMemoriesPerRun,
		SortBy:            "created_at",
		SortDesc:          true,
		ExcludeCategories: []string{"autopilot"},
		ExcludeTypes:      []string{"compiled"},
	})
	if err != nil {
		return nil, err
	}

	maps := BuildEntityMaps(memories)

	var findings []Finding
	for _, det := range d.detectors {
		if dryRun {
			// For dry run, wrap with a dry-run-aware detector that skips side effects.
			// The detectors themselves handle side effects (relation creation), so we
			// need to use a dry-run context or skip the relation detector.
			// Since RelationDetector creates relations as a side effect, we skip it in dry run.
			if det.Name() == "relations" {
				continue
			}
		}
		findings = append(findings, d.safeDetect(ctx, det, projectID, maps)...)
	}

	if !dryRun && len(findings) > 0 {
		if err := d.writer.Write(ctx, projectID, findings); err != nil {
			d.logger.Error("autopilot report write failed", "project_id", projectID, "err", err)
		}
	}

	return findings, nil
}

// safeDetect calls det.Detect and recovers from any panic, logging a Warn.
// Must be a separate function (not an inline defer) so that the deferred
// recover scopes to this call, not to the enclosing loop.
func (d *AutopilotDaemon) safeDetect(ctx context.Context, det Detector, projectID string, maps EntityMaps) (findings []Finding) {
	defer func() {
		if r := recover(); r != nil {
			d.logger.Warn("detector panic", "detector", det.Name(), "panic", r)
		}
	}()
	f, err := det.Detect(ctx, projectID, maps)
	if err != nil {
		d.logger.Warn("detector error", "detector", det.Name(), "err", err)
		return nil
	}
	return f
}

// runCycle discovers active projects and runs the pipeline for each that has new activity.
func (d *AutopilotDaemon) runCycle(ctx context.Context) {
	start := time.Now()

	projects, err := d.mnemos.ListDistinctProjectIDs(ctx)
	if err != nil {
		d.logger.Warn("autopilot: failed to list projects", "err", err)
		return
	}

	total := 0
	for _, project := range projects {
		newest, err := d.mnemos.MaxCreatedAt(ctx, project)
		if err != nil {
			d.logger.Warn("autopilot: MaxCreatedAt failed", "project_id", project, "err", err)
			continue
		}

		d.mu.Lock()
		lastRun := d.lastRun[project]
		d.mu.Unlock()

		// Idle skip: no new memories since last run.
		if !newest.IsZero() && !newest.After(lastRun) {
			d.logger.Debug("autopilot idle", "project_id", project)
			continue
		}

		findings, err := d.RunOnce(ctx, project, false)
		if err != nil {
			d.logger.Warn("autopilot: RunOnce failed", "project_id", project, "err", err)
			continue
		}

		d.mu.Lock()
		d.lastRun[project] = time.Now()
		d.mu.Unlock()

		total += len(findings)
	}

	d.mu.Lock()
	d.lastFindingCount = total
	d.mu.Unlock()

	d.logger.Info("autopilot run", "findings", total, "duration_ms", time.Since(start).Milliseconds(), "skipped", false)

	// Persist state to disk so `mnemos autopilot status` can read it cross-process.
	d.persistState()
}
