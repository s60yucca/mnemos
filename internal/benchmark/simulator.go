//go:build benchmark

package benchmark

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/storage/sqlite"
)

// SessionSimulator runs benchmark scenarios against a real in-memory SQLite instance.
type SessionSimulator struct {
	db     *sql.DB
	store  *sqlite.SQLiteStore
	fts    *sqlite.FTSSearcher
	engine *search.SearchEngine
	eval   MetricsEvaluator
}

// NewSessionSimulator opens an in-memory SQLite instance and wires a real
// SearchEngine with a NoopProvider (dims=384).
func NewSessionSimulator() (*SessionSimulator, error) {
	db, err := sqlite.Open(":memory:")
	if err != nil {
		return nil, fmt.Errorf("init benchmark sqlite: %w", err)
	}

	store := sqlite.NewSQLiteStore(db)
	fts := sqlite.NewFTSSearcher(db)
	embedStore := sqlite.NewEmbeddingStore(db)
	relations := sqlite.NewRelationStore(db)

	noop := embedding.NewNoopProvider(384)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	engine := search.NewSearchEngine(fts, embedStore, noop, relations, logger)

	return &SessionSimulator{
		db:     db,
		store:  store,
		fts:    fts,
		engine: engine,
		eval:   MetricsEvaluator{},
	}, nil
}

// Close releases the underlying database connection.
func (s *SessionSimulator) Close() error {
	return s.db.Close()
}

// RunScenario runs all sessions with an accumulating store.
// Memories are seeded incrementally (AvailableFromSession == sessionNum, equality).
// The store is NOT reset between sessions — it accumulates across sessions.
func (s *SessionSimulator) RunScenario(ctx context.Context, scenario BenchmarkScenario) ([]SessionMetrics, error) {
	var allMetrics []SessionMetrics

	for sessionNum := 1; sessionNum <= scenario.Sessions; sessionNum++ {
		// Seed memories that become available exactly this session.
		for _, mem := range scenario.Memories {
			if mem.AvailableFromSession != sessionNum {
				continue
			}
			now := time.Now()
			dm := &domain.Memory{
				ID:             mem.ID,
				Content:        mem.Content,
				Type:           mem.Type,
				Category:       mem.Category,
				Tags:           mem.Tags,
				ProjectID:      scenario.ProjectID,
				Status:         domain.MemoryStatusActive,
				ContentHash:    mem.ID, // use ID as hash to avoid duplicates
				CreatedAt:      now,
				UpdatedAt:      now,
				LastAccessedAt: now,
				RelevanceScore: 1.0,
			}
			if err := s.store.Create(ctx, dm); err != nil {
				// Ignore duplicate errors — idempotent seeding.
				if err != domain.ErrDuplicate {
					return nil, fmt.Errorf("seed memory %s: %w", mem.ID, err)
				}
			}
		}

		// Snapshot all stored memories for TokenEfficiency calculation.
		allStored, err := s.store.List(ctx, storage.ListQuery{
			ProjectID: scenario.ProjectID,
			Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
			Limit:     10000,
		})
		if err != nil {
			return nil, fmt.Errorf("list stored memories session %d: %w", sessionNum, err)
		}

		// Run each task in the session.
		for _, task := range scenario.Tasks {
			result, err := s.engine.AssembleContext(ctx, task.Query, scenario.ProjectID, task.TokenBudget, false)
			if err != nil {
				// Log and record zero metrics rather than aborting.
				result = &search.ContextResult{}
			}

			metrics := s.eval.Evaluate(result.Memories, allStored, task)
			metrics.SessionNumber = sessionNum
			allMetrics = append(allMetrics, metrics)
		}
	}

	return allMetrics, nil
}

// resetForNextScenario wipes all data between scenarios (not between sessions).
// This is faster than DROP/CREATE for in-memory SQLite since the schema already exists.
// Order matters: relations and embeddings first (FK references memories), then memories
// (triggers handle FTS cleanup automatically).
func (s *SessionSimulator) resetForNextScenario(ctx context.Context) error {
	stmts := []string{
		`DELETE FROM memory_relations`,
		`DELETE FROM memory_embeddings`,
		`DELETE FROM memories`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("reset: %s: %w", stmt, err)
		}
	}
	return nil
}
