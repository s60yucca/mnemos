package lifecycle

import (
	"context"
	"log/slog"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

// Engine manages memory lifecycle: decay, archival, and GC
type Engine struct {
	store           storage.IMemoryStore
	decayInterval   time.Duration
	gcRetentionDays int
	archiveThreshold float64
	logger          *slog.Logger
	stopCh          chan struct{}
}

func NewEngine(
	store storage.IMemoryStore,
	decayInterval time.Duration,
	gcRetentionDays int,
	archiveThreshold float64,
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
	close(e.stopCh)
}

// RunDecay computes and applies decay scores for all active memories
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
			toArchive = append(toArchive, mem.ID)
		}
	}

	if len(updates) > 0 {
		if err := e.store.BulkUpdateRelevance(ctx, updates); err != nil {
			return err
		}
	}

	if len(toArchive) > 0 {
		if err := e.store.BulkUpdateStatus(ctx, toArchive, domain.MemoryStatusArchived); err != nil {
			return err
		}
		e.logger.Info("archived memories", "count", len(toArchive))
	}

	e.logger.Info("decay run complete", "processed", len(memories), "archived", len(toArchive))
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

// RunGC hard-deletes memories with status=deleted older than retention period
func (e *Engine) RunGC(ctx context.Context, projectID string) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -e.gcRetentionDays)
	memories, err := e.store.ListForLifecycle(ctx, storage.LifecycleQuery{
		ProjectID:        projectID,
		Statuses:         []domain.MemoryStatus{domain.MemoryStatusDeleted},
		LastAccessBefore: &cutoff,
		Limit:            1000,
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
	return nil
}

// PromoteMemory resets a memory's relevance score to 1.0
func (e *Engine) PromoteMemory(ctx context.Context, id string) error {
	return e.store.BulkUpdateRelevance(ctx, []storage.BulkUpdateItem{{ID: id, Score: 1.0}})
}
