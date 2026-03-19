package lifecycle_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	"github.com/mnemos-dev/mnemos/internal/domain"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestEngine(t *testing.T) (*lifecycle.Engine, *sqlitestore.SQLiteStore) {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)
	store := sqlitestore.NewSQLiteStore(db)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	engine := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, logger)
	t.Cleanup(func() { db.Close() })
	return engine, store
}

func createMemoryWithAge(t *testing.T, store *sqlitestore.SQLiteStore, content, projectID string, hoursOld float64, status domain.MemoryStatus) *domain.Memory {
	t.Helper()
	ctx := context.Background()
	now := time.Now().UTC()
	lastAccess := now.Add(-time.Duration(hoursOld * float64(time.Hour)))
	mem := &domain.Memory{
		ID:             util.NewID(),
		Content:        content,
		Type:           domain.MemoryTypeShortTerm,
		Category:       domain.CategoryGeneral,
		ProjectID:      projectID,
		CreatedAt:      lastAccess,
		UpdatedAt:      lastAccess,
		LastAccessedAt: lastAccess,
		RelevanceScore: 1.0,
		Status:         status,
		ContentHash:    util.ContentHash(content, projectID),
	}
	require.NoError(t, store.Create(ctx, mem))
	return mem
}

func TestEngine_RunDecay_ReducesScore(t *testing.T) {
	engine, store := newTestEngine(t)
	ctx := context.Background()
	mem := createMemoryWithAge(t, store, "old short term memory", "", 100, domain.MemoryStatusActive)
	require.NoError(t, engine.RunDecay(ctx, ""))
	updated, err := store.GetByID(ctx, mem.ID)
	require.NoError(t, err)
	assert.Less(t, updated.RelevanceScore, 1.0, "score should have decayed")
}

func TestEngine_RunDecay_ArchivesLowScore(t *testing.T) {
	engine, store := newTestEngine(t)
	ctx := context.Background()
	mem := createMemoryWithAge(t, store, "very old memory", "", 10000, domain.MemoryStatusActive)
	require.NoError(t, engine.RunDecay(ctx, ""))
	updated, err := store.GetByID(ctx, mem.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.MemoryStatusArchived, updated.Status)
}

func TestEngine_RunGC_HardDeletesOldDeleted(t *testing.T) {
	engine, store := newTestEngine(t)
	ctx := context.Background()
	mem := createMemoryWithAge(t, store, "deleted old memory", "", 24*40, domain.MemoryStatusDeleted)
	require.NoError(t, engine.RunGC(ctx, ""))
	_, err := store.GetByID(ctx, mem.ID)
	assert.Error(t, err, "memory should have been hard deleted by GC")
}

func TestEngine_RunGC_KeepsRecentDeleted(t *testing.T) {
	engine, store := newTestEngine(t)
	ctx := context.Background()
	mem := createMemoryWithAge(t, store, "recently deleted", "", 1, domain.MemoryStatusDeleted)
	require.NoError(t, engine.RunGC(ctx, ""))
	got, err := store.GetByID(ctx, mem.ID)
	require.NoError(t, err)
	assert.Equal(t, mem.ID, got.ID)
}

func TestEngine_Stop_SafeMultipleCalls(t *testing.T) {
	engine, _ := newTestEngine(t)
	engine.Start()
	assert.NotPanics(t, func() {
		engine.Stop()
		engine.Stop()
	})
}

func TestEngine_PromoteMemory(t *testing.T) {
	engine, store := newTestEngine(t)
	ctx := context.Background()
	mem := createMemoryWithAge(t, store, "memory to promote", "", 100, domain.MemoryStatusActive)
	require.NoError(t, engine.RunDecay(ctx, ""))
	decayed, err := store.GetByID(ctx, mem.ID)
	require.NoError(t, err)
	assert.Less(t, decayed.RelevanceScore, 1.0)
	require.NoError(t, engine.PromoteMemory(ctx, mem.ID))
	promoted, err := store.GetByID(ctx, mem.ID)
	require.NoError(t, err)
	assert.InDelta(t, 1.0, promoted.RelevanceScore, 0.001)
}

func TestEngine_RunDecay_ProjectFilter(t *testing.T) {
	engine, store := newTestEngine(t)
	ctx := context.Background()
	// Create memories with project IDs set at creation time
	mem1 := createMemoryWithAge(t, store, "proj1 memory", "proj1", 100, domain.MemoryStatusActive)
	mem2 := createMemoryWithAge(t, store, "proj2 memory", "proj2", 100, domain.MemoryStatusActive)
	// Decay only proj1
	require.NoError(t, engine.RunDecay(ctx, "proj1"))
	updated1, err := store.GetByID(ctx, mem1.ID)
	require.NoError(t, err)
	updated2, err := store.GetByID(ctx, mem2.ID)
	require.NoError(t, err)
	assert.Less(t, updated1.RelevanceScore, 1.0, "proj1 should have decayed")
	assert.InDelta(t, 1.0, updated2.RelevanceScore, 0.001, "proj2 should be untouched")
}

func TestEngine_Defaults(t *testing.T) {
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)
	defer db.Close()
	store := sqlitestore.NewSQLiteStore(db)
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	engine := lifecycle.NewEngine(store, 0, 0, 0, logger)
	assert.NotNil(t, engine)
	require.NoError(t, engine.RunDecay(context.Background(), ""))
	require.NoError(t, engine.RunGC(context.Background(), ""))
}
