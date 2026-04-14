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

// newTestMemoryWithProject creates a test memory assigned to a project.
func newTestMemoryWithProject(content, projectID string) *domain.Memory {
	m := newTestMemory(content)
	m.ProjectID = projectID
	return m
}

// TestMaxCreatedAt_NoMemories verifies that MaxCreatedAt returns zero time when
// no active memories exist for the project.
func TestMaxCreatedAt_NoMemories(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	got, err := store.MaxCreatedAt(ctx, "empty-project")
	require.NoError(t, err)
	assert.True(t, got.IsZero(), "expected zero time for empty project")
}

// TestMaxCreatedAt_ReturnsLatest verifies that MaxCreatedAt returns the most
// recent created_at among active memories for the project.
func TestMaxCreatedAt_ReturnsLatest(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	older := base.Add(-10 * time.Minute)
	newer := base.Add(5 * time.Minute)

	m1 := newTestMemoryWithProject("older memory", "proj-max")
	m1.CreatedAt = older
	m1.UpdatedAt = older
	m1.LastAccessedAt = older
	require.NoError(t, store.Create(ctx, m1))

	m2 := newTestMemoryWithProject("newer memory", "proj-max")
	m2.CreatedAt = newer
	m2.UpdatedAt = newer
	m2.LastAccessedAt = newer
	require.NoError(t, store.Create(ctx, m2))

	// Deleted memory with even newer timestamp — should not count
	m3 := newTestMemoryWithProject("deleted newer", "proj-max")
	m3.CreatedAt = newer.Add(1 * time.Minute)
	m3.UpdatedAt = newer.Add(1 * time.Minute)
	m3.LastAccessedAt = newer.Add(1 * time.Minute)
	require.NoError(t, store.Create(ctx, m3))
	require.NoError(t, store.Delete(ctx, m3.ID))

	got, err := store.MaxCreatedAt(ctx, "proj-max")
	require.NoError(t, err)
	assert.Equal(t, newer.UnixNano(), got.UnixNano())
}

// TestGetLatestAutopilotReport_None verifies that GetLatestAutopilotReport returns
// nil (not error) when no autopilot report exists for the project.
func TestGetLatestAutopilotReport_None(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	got, err := store.GetLatestAutopilotReport(ctx, "no-report-project")
	require.NoError(t, err)
	assert.Nil(t, got)
}

// TestGetLatestAutopilotReport_ReturnsNewest verifies that when multiple autopilot
// reports exist, the most recently created one is returned.
func TestGetLatestAutopilotReport_ReturnsNewest(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)

	older := newTestMemoryWithProject("## Autopilot Report (old)", "proj-report")
	older.Category = "autopilot"
	older.Type = domain.MemoryTypeSemantic
	older.CreatedAt = base.Add(-1 * time.Hour)
	older.UpdatedAt = base.Add(-1 * time.Hour)
	older.LastAccessedAt = base.Add(-1 * time.Hour)
	require.NoError(t, store.Create(ctx, older))

	newer := newTestMemoryWithProject("## Autopilot Report (new)", "proj-report")
	newer.Category = "autopilot"
	newer.Type = domain.MemoryTypeSemantic
	newer.CreatedAt = base
	newer.UpdatedAt = base
	newer.LastAccessedAt = base
	require.NoError(t, store.Create(ctx, newer))

	got, err := store.GetLatestAutopilotReport(ctx, "proj-report")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, newer.ID, got.ID)
}

// TestListQuery_ExcludeCategories verifies that memories with excluded categories
// are not returned by List.
func TestListQuery_ExcludeCategories(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	m1 := newTestMemoryWithProject("normal memory", "proj-excl")
	m1.Category = "general"
	require.NoError(t, store.Create(ctx, m1))

	m2 := newTestMemoryWithProject("autopilot report", "proj-excl")
	m2.Category = "autopilot"
	require.NoError(t, store.Create(ctx, m2))

	results, err := store.List(ctx, storage.ListQuery{
		ProjectID:         "proj-excl",
		ExcludeCategories: []string{"autopilot"},
		Limit:             10,
	})
	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, "autopilot", r.Category, "excluded category should not appear")
	}
	assert.Len(t, results, 1)
	assert.Equal(t, m1.ID, results[0].ID)
}

// TestListQuery_ExcludeTypes verifies that memories with excluded types are not
// returned by List.
func TestListQuery_ExcludeTypes(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	m1 := newTestMemoryWithProject("episodic memory", "proj-excl-type")
	m1.Type = domain.MemoryTypeEpisodic
	require.NoError(t, store.Create(ctx, m1))

	m2 := newTestMemoryWithProject("compiled article", "proj-excl-type")
	m2.Type = domain.MemoryTypeCompiled
	require.NoError(t, store.Create(ctx, m2))

	results, err := store.List(ctx, storage.ListQuery{
		ProjectID:    "proj-excl-type",
		ExcludeTypes: []string{"compiled"},
		Limit:        10,
	})
	require.NoError(t, err)
	for _, r := range results {
		assert.NotEqual(t, domain.MemoryTypeCompiled, r.Type, "excluded type should not appear")
	}
	assert.Len(t, results, 1)
	assert.Equal(t, m1.ID, results[0].ID)
}

// TestGetByIDs_BatchFetch verifies that GetByIDs returns all matching active
// memories and omits missing or inactive IDs.
func TestGetByIDs_BatchFetch(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	m1 := newTestMemoryWithProject("batch one", "proj-batch")
	m2 := newTestMemoryWithProject("batch two", "proj-batch")
	m3 := newTestMemoryWithProject("batch deleted", "proj-batch")
	require.NoError(t, store.Create(ctx, m1))
	require.NoError(t, store.Create(ctx, m2))
	require.NoError(t, store.Create(ctx, m3))
	require.NoError(t, store.Delete(ctx, m3.ID)) // soft-delete

	result, err := store.GetByIDs(ctx, []string{m1.ID, m2.ID, m3.ID, "nonexistent-id"})
	require.NoError(t, err)
	assert.Len(t, result, 2, "should return only active, existing memories")
	assert.Contains(t, result, m1.ID)
	assert.Contains(t, result, m2.ID)
	assert.NotContains(t, result, m3.ID)
	assert.NotContains(t, result, "nonexistent-id")
}

// TestGetByIDs_EmptyInput verifies that GetByIDs returns an empty map (not error)
// when given an empty slice.
func TestGetByIDs_EmptyInput(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	result, err := store.GetByIDs(ctx, []string{})
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result, 0)
}

// TestGetRelationBetween_BidirectionalCheck verifies that GetRelationBetween finds
// a relation regardless of which direction it was stored in.
func TestGetRelationBetween_BidirectionalCheck(t *testing.T) {
	db, err := Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	memStore := NewSQLiteStore(db)
	relStore := NewRelationStore(db)
	ctx := context.Background()

	// Create the memories so FK constraints are satisfied
	mA := newTestMemoryWithProject("memory A", "proj-rel")
	mB := newTestMemoryWithProject("memory B", "proj-rel")
	require.NoError(t, memStore.Create(ctx, mA))
	require.NoError(t, memStore.Create(ctx, mB))

	// Create relation A → B
	rel := &domain.MemoryRelation{
		ID:             util.NewID(),
		SourceMemoryID: mA.ID,
		TargetMemoryID: mB.ID,
		RelationType:   domain.RelationTypeRelatesTo,
		Strength:       0.5,
		CreatedAt:      time.Now().UTC(),
	}
	require.NoError(t, relStore.CreateRelation(ctx, rel))

	// Query as A → B (same direction)
	found, err := relStore.GetRelationBetween(ctx, mA.ID, mB.ID, domain.RelationTypeRelatesTo)
	require.NoError(t, err)
	require.NotNil(t, found, "should find relation in forward direction")
	assert.Equal(t, rel.ID, found.ID)

	// Query as B → A (reverse direction) — bidirectional check
	foundReverse, err := relStore.GetRelationBetween(ctx, mB.ID, mA.ID, domain.RelationTypeRelatesTo)
	require.NoError(t, err)
	require.NotNil(t, foundReverse, "should find relation in reverse direction")
	assert.Equal(t, rel.ID, foundReverse.ID)

	// Query with different relation type — should return nil
	notFound, err := relStore.GetRelationBetween(ctx, mA.ID, mB.ID, domain.RelationTypeCompiledFrom)
	require.NoError(t, err)
	assert.Nil(t, notFound)
}

// TestListDistinctProjectIDs_ReturnsActiveProjects verifies that only projects with
// active memories are returned, and deleted memories do not contribute.
func TestListDistinctProjectIDs_ReturnsActiveProjects(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	// Active memories for two projects
	m1 := newTestMemoryWithProject("active proj-a", "proj-a")
	m2 := newTestMemoryWithProject("active proj-b", "proj-b")
	require.NoError(t, store.Create(ctx, m1))
	require.NoError(t, store.Create(ctx, m2))

	// Deleted memory for proj-c — should not appear
	m3 := newTestMemoryWithProject("deleted proj-c", "proj-c")
	require.NoError(t, store.Create(ctx, m3))
	require.NoError(t, store.Delete(ctx, m3.ID))

	// Memory with empty project_id — should not appear
	m4 := newTestMemory("no project")
	m4.ProjectID = ""
	require.NoError(t, store.Create(ctx, m4))

	ids, err := store.ListDistinctProjectIDs(ctx)
	require.NoError(t, err)
	assert.Contains(t, ids, "proj-a")
	assert.Contains(t, ids, "proj-b")
	assert.NotContains(t, ids, "proj-c")
	assert.NotContains(t, ids, "")
}

// TestListDistinctProjectIDs_EmptyStore verifies that an empty slice (not nil, not
// error) is returned when no active memories exist.
func TestListDistinctProjectIDs_EmptyStore(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	ids, err := store.ListDistinctProjectIDs(ctx)
	require.NoError(t, err)
	assert.NotNil(t, ids)
	assert.Len(t, ids, 0)
}

// TestGetByIDs_ChunkingLargeInput verifies that GetByIDs correctly handles more
// than 900 IDs by chunking queries to stay under SQLite's SQLITE_MAX_VARIABLE_NUMBER.
func TestGetByIDs_ChunkingLargeInput(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	// Create 950 memories with UNIQUE content to avoid dedup collisions.
	// The chunking logic is about SQL parameter limits, not content — but unique
	// content ensures each memory gets a distinct ID and content hash.
	const total = 950
	ids := make([]string, total)
	for i := 0; i < total; i++ {
		m := newTestMemoryWithProject(
			fmt.Sprintf("chunking test memory %d with unique content hash seed %d", i, i),
			"proj-chunk",
		)
		require.NoError(t, store.Create(ctx, m))
		ids[i] = m.ID
	}

	result, err := store.GetByIDs(ctx, ids)
	require.NoError(t, err, "GetByIDs must not fail — chunking should handle >900 IDs transparently")
	assert.Len(t, result, total, "all %d memories should be returned across chunks", total)
}
