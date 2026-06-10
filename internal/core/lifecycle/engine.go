package lifecycle

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

// Engine manages memory lifecycle: decay, archival, and GC
type Engine struct {
	store            storage.IMemoryStore
	decayInterval    time.Duration
	gcRetentionDays  int
	archiveThreshold float64
	autoArchive      bool
	logger           *slog.Logger
	stopCh           chan struct{}
	stopOnce         sync.Once
}

func NewEngine(
	store storage.IMemoryStore,
	decayInterval time.Duration,
	gcRetentionDays int,
	archiveThreshold float64,
	autoArchive bool,
	logger *slog.Logger,
) *Engine {
	if decayInterval <= 0 {
		decayInterval = 24 * time.Hour
	}
	if gcRetentionDays <= 0 {
		gcRetentionDays = 30
	}
	if archiveThreshold <= 0 {
		archiveThreshold = 0.1
	}
	return &Engine{
		store:            store,
		decayInterval:    decayInterval,
		gcRetentionDays:  gcRetentionDays,
		archiveThreshold: archiveThreshold,
		autoArchive:      autoArchive,
		logger:           logger,
		stopCh:           make(chan struct{}),
	}
}

// Start begins the background lifecycle ticker
func (e *Engine) Start() {
	go func() {
		ticker := time.NewTicker(e.decayInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx := context.Background()
				if err := e.RunDecay(ctx, ""); err != nil {
					e.logger.Error("decay run failed", "err", err)
				}
				if err := e.RunGC(ctx, ""); err != nil {
					e.logger.Error("gc run failed", "err", err)
				}
			case <-e.stopCh:
				return
			}
		}
	}()
}

// Stop halts the background ticker
func (e *Engine) Stop() {
	e.stopOnce.Do(func() { close(e.stopCh) })
}

// RunDecay computes and applies decay scores for all active memories.
// Archival is only performed when autoArchive is enabled.
func (e *Engine) RunDecay(ctx context.Context, projectID string) error {
	memories, err := e.store.ListForLifecycle(ctx, storage.LifecycleQuery{
		ProjectID: projectID,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:     10000,
	})
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	var updates []storage.BulkUpdateItem
	var toArchive []string

	for _, mem := range memories {
		newScore := ComputeDecayScore(mem, now)
		updates = append(updates, storage.BulkUpdateItem{ID: mem.ID, Score: newScore})
		if newScore < e.archiveThreshold {
			if e.autoArchive {
				toArchive = append(toArchive, mem.ID)
			}
		}
	}

	if len(updates) > 0 {
		if err := e.store.BulkUpdateRelevance(ctx, updates); err != nil {
			return err
		}
	}

	if e.autoArchive && len(toArchive) > 0 {
		if err := e.store.BulkUpdateStatus(ctx, toArchive, domain.MemoryStatusArchived); err != nil {
			return err
		}
		e.logger.Info("archived memories", "count", len(toArchive))
	}

	archived := len(toArchive)
	if !e.autoArchive && archived > 0 {
		archived = 0
	}
	e.logger.Info("decay run complete", "processed", len(memories), "archived", archived)
	return nil
}

// RunArchival archives memories below the threshold
func (e *Engine) RunArchival(ctx context.Context, projectID string) error {
	memories, err := e.store.ListForLifecycle(ctx, storage.LifecycleQuery{
		ProjectID:    projectID,
		MaxRelevance: e.archiveThreshold,
		Statuses:     []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:        1000,
	})
	if err != nil {
		return err
	}

	ids := make([]string, len(memories))
	for i, m := range memories {
		ids[i] = m.ID
	}
	if len(ids) > 0 {
		return e.store.BulkUpdateStatus(ctx, ids, domain.MemoryStatusArchived)
	}
	return nil
}

// RunGC hard-deletes memories with status=deleted older than retention period,
// and purges archived autopilot reports older than retention. Autopilot reports
// are generated snapshots that get archived on every superseding write (keeping
// only the latest active); without this pass they would accumulate in the
// archived state forever, since the deleted-status GC never reaches them.
func (e *Engine) RunGC(ctx context.Context, projectID string) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -e.gcRetentionDays)
	memories, err := e.store.ListForLifecycle(ctx, storage.LifecycleQuery{
		ProjectID:     projectID,
		Statuses:      []domain.MemoryStatus{domain.MemoryStatusDeleted},
		UpdatedBefore: &cutoff,
		Limit:         1000,
	})
	if err != nil {
		return err
	}

	for _, mem := range memories {
		if err := e.store.HardDelete(ctx, mem.ID); err != nil {
			e.logger.Warn("gc hard delete failed", "id", mem.ID, "err", err)
		}
	}
	if len(memories) > 0 {
		e.logger.Info("gc complete", "deleted", len(memories))
	}

	reportsPurged, err := e.gcArchivedReports(ctx, projectID, cutoff)
	if err != nil {
		return err
	}
	if reportsPurged > 0 {
		e.logger.Info("gc archived reports complete", "deleted", reportsPurged)
	}
	return nil
}

// gcArchivedReports hard-deletes archived autopilot reports older than the
// cutoff, draining any backlog in batches. Scoped to category="autopilot" and
// the autopilot-daemon source so legitimately archived user memories are never
// touched.
func (e *Engine) gcArchivedReports(ctx context.Context, projectID string, cutoff time.Time) (int, error) {
	const batchSize = 1000
	purged := 0
	for {
		reports, err := e.store.ListForLifecycle(ctx, storage.LifecycleQuery{
			ProjectID:     projectID,
			Statuses:      []domain.MemoryStatus{domain.MemoryStatusArchived},
			Categories:    []string{"autopilot"},
			UpdatedBefore: &cutoff,
			Limit:         batchSize,
		})
		if err != nil {
			return purged, err
		}

		deletedThisBatch := 0
		for _, mem := range reports {
			if mem.Source != "autopilot-daemon" {
				continue
			}
			if err := e.store.HardDelete(ctx, mem.ID); err != nil {
				e.logger.Warn("gc report hard delete failed", "id", mem.ID, "err", err)
				continue
			}
			deletedThisBatch++
		}
		purged += deletedThisBatch

		// Stop when the batch is exhausted, or when no row in a full batch
		// matched the source filter (avoids looping on the same rows forever).
		if len(reports) < batchSize || deletedThisBatch == 0 {
			return purged, nil
		}
	}
}

// PromoteMemory resets a memory's relevance score to 1.0
func (e *Engine) PromoteMemory(ctx context.Context, id string) error {
	return e.store.BulkUpdateRelevance(ctx, []storage.BulkUpdateItem{{ID: id, Score: 1.0}})
}
