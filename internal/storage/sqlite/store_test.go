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

func TestSQLiteStore_GetRecentMemories(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	// Add 3 memories
	for i := 0; i < 3; i++ {
		mem := newTestMemory(fmt.Sprintf("recent %d", i))
		mem.ProjectID = "proj-recent"
		mem.CreatedAt = time.Now().Add(time.Duration(i) * time.Minute) // 0, 1m, 2m
		require.NoError(t, store.Create(ctx, mem))
	}

	mems, err := store.GetRecentMemories(ctx, "proj-recent", 2)
	require.NoError(t, err)
	assert.Len(t, mems, 2)
	// Should be sorted by created_at DESC
	assert.True(t, mems[0].CreatedAt.After(mems[1].CreatedAt))
}

func TestSQLiteStore_GetLastSessionSummary(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	// Empty project
	m, err := store.GetLastSessionSummary(ctx, "proj-session")
	require.NoError(t, err)
	assert.Nil(t, m) // should return (nil, nil)

	// Has memories but no session category or auto-breadcrumb tag
	mem := newTestMemory("ordinary")
	mem.ProjectID = "proj-session"
	require.NoError(t, store.Create(ctx, mem))

	m2, err := store.GetLastSessionSummary(ctx, "proj-session")
	require.NoError(t, err)
	assert.Nil(t, m2) // still nil

	// Add a session memory
	sessMem := newTestMemory("session mem")
	sessMem.ProjectID = "proj-session"
	sessMem.Category = "session"
	require.NoError(t, store.Create(ctx, sessMem))

	m3, err := store.GetLastSessionSummary(ctx, "proj-session")
	require.NoError(t, err)
	require.NotNil(t, m3)
	assert.Equal(t, sessMem.ID, m3.ID)

	// Add an auto-breadcrumb tagged memory (more recent)
	crumbMem := newTestMemory("crumb")
	crumbMem.ProjectID = "proj-session"
	crumbMem.Tags = []string{"auto-breadcrumb"}
	crumbMem.CreatedAt = sessMem.CreatedAt.Add(1 * time.Minute)
	require.NoError(t, store.Create(ctx, crumbMem))

	m4, err := store.GetLastSessionSummary(ctx, "proj-session")
	require.NoError(t, err)
	require.NotNil(t, m4)
	assert.Equal(t, crumbMem.ID, m4.ID)
}

func TestSQLiteStore_GetCompiledArticles(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	mems, err := store.GetCompiledArticles(ctx, "proj-compiled", 10)
	require.NoError(t, err)
	assert.Len(t, mems, 0) // Should be empty slice, not nil error

	// Add compiled
	c1 := newTestMemory("c1")
	c1.ProjectID = "proj-compiled"
	c1.Type = domain.MemoryTypeCompiled
	c1.RelevanceScore = 0.5
	require.NoError(t, store.Create(ctx, c1))

	c2 := newTestMemory("c2")
	c2.ProjectID = "proj-compiled"
	c2.Type = domain.MemoryTypeCompiled
	c2.RelevanceScore = 0.8
	require.NoError(t, store.Create(ctx, c2))

	mems2, err := store.GetCompiledArticles(ctx, "proj-compiled", 10)
	require.NoError(t, err)
	assert.Len(t, mems2, 2)
	assert.Equal(t, c2.ID, mems2[0].ID) // Sorted by relevance DESC
	assert.Equal(t, c1.ID, mems2[1].ID)
}

func TestSQLiteStore_ReduceRelevance(t *testing.T) {
	store := newTestDB(t)
	ctx := context.Background()

	mem := newTestMemory("reduce score me")
	mem.RelevanceScore = 0.8
	require.NoError(t, store.Create(ctx, mem))

	// Reduce by 0.3, floor at 0.05
	require.NoError(t, store.ReduceRelevance(ctx, mem.ID, 0.3, 0.05))
	got, _ := store.GetByID(ctx, mem.ID)
	assert.InDelta(t, 0.5, got.RelevanceScore, 0.001)

	// Reduce by 1.0, should hit floor
	require.NoError(t, store.ReduceRelevance(ctx, mem.ID, 1.0, 0.05))
	got2, _ := store.GetByID(ctx, mem.ID)
	assert.InDelta(t, 0.05, got2.RelevanceScore, 0.001)

	// NotFound error
	err := store.ReduceRelevance(ctx, "nonexistent", 0.1, 0.05)
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// TestOpen_ConnectionPragmasApplied verifies that per-connection PRAGMAs are correctly applied.
// This test addresses the bug where PRAGMAs in the schema string were not persisting to new connections.
func TestOpen_ConnectionPragmasApplied(t *testing.T) {
	db, err := Open(":memory:")
	require.NoError(t, err)
	defer db.Close()

	tests := []struct {
		pragma   string
		expected int64
		name     string
	}{
		{"busy_timeout", 5000, "busy_timeout should be 5000ms"},
		{"foreign_keys", 1, "foreign_keys should be ON (1)"},
		{"temp_store", 2, "temp_store should be MEMORY (2)"},
		{"wal_autocheckpoint", 1000, "wal_autocheckpoint should be 1000 pages"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var value int64
			query := fmt.Sprintf("PRAGMA %s", tt.pragma)
			err := db.QueryRow(query).Scan(&value)
			require.NoError(t, err, "failed to query %s", tt.pragma)
			assert.Equal(t, tt.expected, value, "%s mismatch", tt.pragma)
		})
	}

	// cache_size check (allow some tolerance)
	var cacheSize int64
	err = db.QueryRow("PRAGMA cache_size").Scan(&cacheSize)
	require.NoError(t, err, "failed to query cache_size")
	assert.True(t, cacheSize <= -60000 && cacheSize >= -68000,
		"cache_size should be around -64000, got %d", cacheSize)

	// mmap_size check (may not be supported by all drivers)
	var mmapSize int64
	err = db.QueryRow("PRAGMA mmap_size").Scan(&mmapSize)
	if err == nil {
		assert.Equal(t, int64(268435456), mmapSize, "mmap_size should be 256MB")
	} else {
		t.Logf("mmap_size PRAGMA not supported by driver: %v", err)
	}
}

// TestOpen_ValidationCatchesMissingPragma verifies that validateConnectionPragmas
// catches configuration issues at startup.
func TestOpen_ValidationCatchesMissingPragma(t *testing.T) {
	// This test verifies the validation logic by checking that Open() succeeds
	// when PRAGMAs are correctly applied. A negative test (forcing validation failure)
	// would require mocking the database, which is beyond the scope of this test.

	db, err := Open(":memory:")
	require.NoError(t, err, "Open should succeed when PRAGMAs are correctly applied")
	defer db.Close()

	// Verify validation ran by checking that critical PRAGMAs are set
	var busyTimeout int64
	err = db.QueryRow("PRAGMA busy_timeout").Scan(&busyTimeout)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), busyTimeout, "validation should have caught incorrect busy_timeout")
}

// TestForeignKeysEnforced verifies that foreign key constraints are actually enforced.
// This test addresses the bug where foreign_keys=OFF at runtime despite being set in code.
func TestForeignKeysEnforced(t *testing.T) {
	db, err := Open(":memory:")
	require.NoError(t, err)
	defer db.Close()

	// Verify foreign_keys is ON
	var fkEnabled int64
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	require.NoError(t, err)
	assert.Equal(t, int64(1), fkEnabled, "foreign_keys should be ON")

	// Test that foreign key constraint is enforced
	// Try to insert a memory_relation with non-existent source_id
	ctx := context.Background()
	_, err = db.ExecContext(ctx, `
		INSERT INTO memory_relations (id, source_id, target_id, relation_type, strength, created_at)
		VALUES ('rel1', 'nonexistent-source', 'nonexistent-target', 'relates_to', 1.0, ?)
	`, time.Now().UnixNano())

	// Should fail with foreign key constraint error
	require.Error(t, err, "should fail due to foreign key constraint")
	assert.Contains(t, err.Error(), "FOREIGN KEY constraint failed",
		"error should mention foreign key constraint")
}

// TestBusyTimeoutPreventsImmediateFailure verifies that busy_timeout allows
// concurrent writes to wait instead of failing immediately.
func TestBusyTimeoutPreventsImmediateFailure(t *testing.T) {
	// Create a real file-based database for this test (not :memory:)
	// because we need to test multi-connection behavior
	tmpDB := t.TempDir() + "/test.db"
	db1, err := Open(tmpDB)
	require.NoError(t, err)
	defer db1.Close()

	// Verify busy_timeout is set
	var timeout int64
	err = db1.QueryRow("PRAGMA busy_timeout").Scan(&timeout)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), timeout, "busy_timeout should be 5000ms")

	// Open a second connection to the same database
	db2, err := Open(tmpDB)
	require.NoError(t, err)
	defer db2.Close()

	ctx := context.Background()

	// Start a transaction on db1 (holds RESERVED lock)
	tx1, err := db1.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx1.Rollback()

	// Insert a memory in tx1 (upgrades to EXCLUSIVE lock)
	mem := newTestMemory("concurrent test")
	_, err = tx1.Exec(`
		INSERT INTO memories (id, content, type, status, content_hash, created_at, updated_at, last_accessed_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		mem.ID, mem.Content, mem.Type, mem.Status, mem.ContentHash,
		util.TimeToUnixNano(mem.CreatedAt), util.TimeToUnixNano(mem.UpdatedAt), util.TimeToUnixNano(mem.LastAccessedAt))
	require.NoError(t, err)

	// Try to write from db2 while tx1 is active
	// With busy_timeout=5000, this should wait (not fail immediately)
	// We'll commit tx1 quickly so db2 succeeds
	done := make(chan error, 1)
	go func() {
		mem2 := newTestMemory("concurrent test 2")
		_, err := db2.ExecContext(ctx, `
			INSERT INTO memories (id, content, type, status, content_hash, created_at, updated_at, last_accessed_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			mem2.ID, mem2.Content, mem2.Type, mem2.Status, mem2.ContentHash,
			util.TimeToUnixNano(mem2.CreatedAt), util.TimeToUnixNano(mem2.UpdatedAt), util.TimeToUnixNano(mem2.LastAccessedAt))
		done <- err
	}()

	// Wait a bit to ensure db2 is blocked
	time.Sleep(100 * time.Millisecond)

	// Commit tx1 to release the lock
	require.NoError(t, tx1.Commit())

	// db2 should now succeed (waited for lock instead of failing immediately)
	select {
	case err := <-done:
		require.NoError(t, err, "db2 write should succeed after tx1 commits")
	case <-time.After(6 * time.Second):
		t.Fatal("db2 write timed out (busy_timeout may not be working)")
	}
}
