package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *SQLiteStore {
	t.Helper()
	db, err := Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return NewSQLiteStore(db)
}

func newTestMemory(content string) *domain.Memory {
	now := time.Now().UTC()
	return &domain.Memory{
		ID:             util.NewID(),
		Content:        content,
		Type:           domain.MemoryTypeEpisodic,
		Category:       domain.CategoryGeneral,
		Tags:           []string{"test"},
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
		RelevanceScore: 1.0,
		Status:         domain.MemoryStatusActive,
		ContentHash:    util.ContentHash(content, ""),
	}
}

func TestSQLiteStore_CreateAndGet(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	mem := newTestMemory("hello world")
	require.NoError(t, store.Create(ctx, mem))

	got, err := store.GetByID(ctx, mem.ID)
	require.NoError(t, err)
	assert.Equal(t, mem.ID, got.ID)
	assert.Equal(t, mem.Content, got.Content)
	assert.Equal(t, mem.ContentHash, got.ContentHash)
}

func TestSQLiteStore_GetByHash(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	mem := newTestMemory("unique content for hash test")
	require.NoError(t, store.Create(ctx, mem))

	got, err := store.GetByHash(ctx, mem.ContentHash)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, mem.ID, got.ID)
}

func TestSQLiteStore_NotFound(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	_, err := store.GetByID(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSQLiteStore_SoftDelete(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	mem := newTestMemory("to be deleted")
	require.NoError(t, store.Create(ctx, mem))
	require.NoError(t, store.Delete(ctx, mem.ID))

	got, err := store.GetByID(ctx, mem.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.MemoryStatusDeleted, got.Status)
}

func TestSQLiteStore_HardDeleteMissingReturnsNotFound(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	err := store.HardDelete(ctx, "nonexistent")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

func TestSQLiteStore_List(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		mem := newTestMemory("memory " + util.NewID())
		mem.ProjectID = "proj1"
		require.NoError(t, store.Create(ctx, mem))
	}

	memories, err := store.List(ctx, storage.ListQuery{ProjectID: "proj1", Limit: 10})
	require.NoError(t, err)
	assert.Len(t, memories, 5)
}

func TestSQLiteStore_BulkUpdateRelevance(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	mem := newTestMemory("relevance test")
	require.NoError(t, store.Create(ctx, mem))

	require.NoError(t, store.BulkUpdateRelevance(ctx, []storage.BulkUpdateItem{
		{ID: mem.ID, Score: 0.5},
	}))

	got, err := store.GetByID(ctx, mem.ID)
	require.NoError(t, err)
	assert.InDelta(t, 0.5, got.RelevanceScore, 0.001)
}

func TestSQLiteStore_Stats(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		mem := newTestMemory("stats test " + util.NewID())
		require.NoError(t, store.Create(ctx, mem))
	}

	stats, err := store.Stats(ctx, "")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.Total, 3)
}
