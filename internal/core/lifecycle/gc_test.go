package lifecycle

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/observe"
	"github.com/mnemos-dev/mnemos/internal/storage"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/stretchr/testify/require"
)

func newGCTestStore(t *testing.T) *sqlitestore.SQLiteStore {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return sqlitestore.NewSQLiteStore(db)
}

func insertMem(t *testing.T, store *sqlitestore.SQLiteStore, m *domain.Memory) {
	t.Helper()
	if m.ContentHash == "" {
		m.ContentHash = util.NewID()
	}
	require.NoError(t, store.Create(context.Background(), m))
}

func countActive(t *testing.T, store *sqlitestore.SQLiteStore, status domain.MemoryStatus) int {
	t.Helper()
	mems, err := store.ListForLifecycle(context.Background(), storage.LifecycleQuery{
		Statuses: []domain.MemoryStatus{status},
		Limit:    100000,
	})
	require.NoError(t, err)
	return len(mems)
}

// TestRunGC_PurgesOldArchivedReports verifies archived autopilot reports older
// than retention are hard-deleted, while recent reports, the active report, and
// unrelated archived user memories are preserved.
func TestRunGC_PurgesOldArchivedReports(t *testing.T) {
	store := newGCTestStore(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	engine := NewEngine(store, 24*time.Hour, 30, 0.1, false, logger)

	now := time.Now().UTC()
	old := now.AddDate(0, 0, -40)    // beyond 30-day retention
	recent := now.AddDate(0, 0, -10) // within retention

	report := func(id string, updated time.Time, status domain.MemoryStatus, source string) *domain.Memory {
		return &domain.Memory{
			ID:             id,
			Content:        "## Autopilot Report",
			Type:           domain.MemoryTypeSemantic,
			Category:       "autopilot",
			Source:         source,
			Tags:           []string{"autopilot-report", "auto-generated"},
			ProjectID:      "p1",
			Status:         status,
			CreatedAt:      updated,
			UpdatedAt:      updated,
			LastAccessedAt: updated,
			RelevanceScore: 1.0,
		}
	}

	// Should be deleted: old + archived + daemon-sourced autopilot reports.
	insertMem(t, store, report("old-1", old, domain.MemoryStatusArchived, "autopilot-daemon"))
	insertMem(t, store, report("old-2", old, domain.MemoryStatusArchived, "autopilot-daemon"))
	// Should survive: within retention.
	insertMem(t, store, report("recent", recent, domain.MemoryStatusArchived, "autopilot-daemon"))
	// Should survive: the active (latest) report.
	insertMem(t, store, report("active", now, domain.MemoryStatusActive, "autopilot-daemon"))
	// Should survive: old archived autopilot-category memory NOT from the daemon.
	insertMem(t, store, report("not-daemon", old, domain.MemoryStatusArchived, "user"))
	// Should survive: an unrelated old archived user memory.
	insertMem(t, store, &domain.Memory{
		ID: "user-mem", Content: "real knowledge", Type: domain.MemoryTypeSemantic,
		Category: "code", Source: "user", ProjectID: "p1",
		Status: domain.MemoryStatusArchived, CreatedAt: old, UpdatedAt: old,
		LastAccessedAt: old, RelevanceScore: 1.0,
	})

	require.NoError(t, engine.RunGC(ctx, ""))

	// Only old-1 and old-2 should be gone.
	for _, id := range []string{"recent", "active", "not-daemon", "user-mem"} {
		_, err := store.GetByID(ctx, id)
		require.NoErrorf(t, err, "%s should be preserved", id)
	}
	for _, id := range []string{"old-1", "old-2"} {
		_, err := store.GetByID(ctx, id)
		require.Errorf(t, err, "%s should have been hard-deleted", id)
	}

	require.Equal(t, 1, countActive(t, store, domain.MemoryStatusActive), "active report preserved")
	require.Equal(t, 3, countActive(t, store, domain.MemoryStatusArchived), "recent report + non-daemon + user memory preserved")
}

func TestRunDecayEmitsFeature(t *testing.T) {
	featuresLog := filepath.Join(t.TempDir(), "features.log")
	observe.SetLogPath(featuresLog)
	t.Cleanup(func() { observe.SetLogPath("") })

	store := newGCTestStore(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	engine := NewEngine(store, 24*time.Hour, 30, 0.1, false, logger)

	now := time.Now().UTC()
	insertMem(t, store, &domain.Memory{
		ID: "decay-1", Content: "real knowledge", Type: domain.MemoryTypeSemantic,
		Category: "code", Source: "user", ProjectID: "p1",
		Status: domain.MemoryStatusActive, CreatedAt: now, UpdatedAt: now,
		LastAccessedAt: now, RelevanceScore: 1.0,
	})

	require.NoError(t, engine.RunDecay(ctx, "p1"))

	data, err := os.ReadFile(featuresLog)
	require.NoError(t, err)
	log := string(data)
	require.True(t, strings.Contains(log, "\tdecay\t"), log)
	require.True(t, strings.Contains(log, "project_id=p1"), log)
	require.True(t, strings.Contains(log, "processed=1"), log)
	require.True(t, strings.Contains(log, "outcome=ok"), log)
}
