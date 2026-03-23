package sqlite

import (
	"context"
	"fmt"
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

// TestProp_CountMemoriesSinceCorrect verifies Property 14:
// CountMemoriesSince returns exactly the count of active memories for a project
// created at or after the given time.
//
// Feature: mnemos-autopilot, Property 14: CountMemoriesSince counts correctly
func TestProp_CountMemoriesSinceCorrect(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	projectID := "prop-count-project"
	otherProject := "other-project"

	// Create memories at known times
	past := time.Now().Add(-2 * time.Hour)
	recent := time.Now().Add(-30 * time.Minute)

	// 3 old memories (before cutoff)
	for i := 0; i < 3; i++ {
		mem := newTestMemory(fmt.Sprintf("old memory %d", i))
		mem.ProjectID = projectID
		mem.CreatedAt = past
		mem.UpdatedAt = past
		mem.LastAccessedAt = past
		require.NoError(t, store.Create(ctx, mem))
	}

	// 2 recent memories (after cutoff)
	for i := 0; i < 2; i++ {
		mem := newTestMemory(fmt.Sprintf("recent memory %d", i))
		mem.ProjectID = projectID
		mem.CreatedAt = recent
		mem.UpdatedAt = recent
		mem.LastAccessedAt = recent
		require.NoError(t, store.Create(ctx, mem))
	}

	// 1 recent memory for a different project (should not be counted)
	otherMem := newTestMemory("other project memory")
	otherMem.ProjectID = otherProject
	otherMem.CreatedAt = recent
	otherMem.UpdatedAt = recent
	otherMem.LastAccessedAt = recent
	require.NoError(t, store.Create(ctx, otherMem))

	// 1 deleted recent memory (should not be counted)
	deletedMem := newTestMemory("deleted memory")
	deletedMem.ProjectID = projectID
	deletedMem.CreatedAt = recent
	deletedMem.UpdatedAt = recent
	deletedMem.LastAccessedAt = recent
	require.NoError(t, store.Create(ctx, deletedMem))
	require.NoError(t, store.Delete(ctx, deletedMem.ID))

	// Cutoff: 1 hour ago — should count only the 2 recent active memories
	cutoff := time.Now().Add(-1 * time.Hour)
	count, err := store.CountMemoriesSince(ctx, projectID, cutoff)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "should count only recent active memories for the project")

	// Cutoff far in the past — should count all 5 active memories for the project
	allCount, err := store.CountMemoriesSince(ctx, projectID, past.Add(-1*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 5, allCount, "should count all active memories when cutoff is before all")

	// Cutoff in the future — should count 0
	futureCount, err := store.CountMemoriesSince(ctx, projectID, time.Now().Add(1*time.Hour))
	require.NoError(t, err)
	assert.Equal(t, 0, futureCount, "should count 0 when cutoff is in the future")
}
