package e2e

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestMnemos(t *testing.T) *core.Mnemos {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	memStore := sqlitestore.NewSQLiteStore(db)
	fts := sqlitestore.NewFTSSearcher(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	relStore := sqlitestore.NewRelationStore(db)

	embedProvider := embedding.NewNoopProvider(384)
	mirror := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	memManager := coremem.NewManager(memStore, embedStore, embedProvider, mirror, 0.85, 0.92, logger, nil)
	searchEngine := search.NewSearchEngine(fts, embedStore, embedProvider, relStore, logger)
	relManager := relation.NewManager(relStore, memStore, logger)
	lcEngine := lifecycle.NewEngine(memStore, 24*time.Hour, 30, 0.1, logger)

	m := core.NewMnemos(memManager, searchEngine, relManager, lcEngine, memStore, logger)

	t.Cleanup(func() {
		m.Shutdown()
		db.Close()
	})
	return m
}

// E2E Test 1: Store → Search → Get round-trip
func TestE2E_StoreSearchGet(t *testing.T) {
	m := newTestMnemos(t)
	ctx := context.Background()

	// Store
	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "The authentication service uses JWT tokens with RS256 signing",
		ProjectID: "proj-e2e",
		Tags:      []string{"auth", "jwt"},
	})
	require.NoError(t, err)
	assert.True(t, result.Created)
	assert.NotEmpty(t, result.Memory.ID)
	assert.Equal(t, domain.MemoryStatusActive, result.Memory.Status)
	assert.Equal(t, 1.0, result.Memory.RelevanceScore)

	// Get
	mem, err := m.Get(ctx, result.Memory.ID)
	require.NoError(t, err)
	assert.Equal(t, result.Memory.ID, mem.ID)
	assert.Equal(t, 1, mem.AccessCount) // TouchAccess called

	// Search
	results, err := m.Search(ctx, "JWT authentication", "proj-e2e", 10)
	require.NoError(t, err)
	// FTS should find it
	found := false
	for _, r := range results {
		if r.Memory.ID == result.Memory.ID {
			found = true
			break
		}
	}
	assert.True(t, found, "stored memory should appear in search results")
}

// E2E Test 2: Exact deduplication
func TestE2E_ExactDedup(t *testing.T) {
	m := newTestMnemos(t)
	ctx := context.Background()

	content := "Use PostgreSQL for the primary database with connection pooling"

	r1, err := m.Store(ctx, &domain.StoreRequest{Content: content, ProjectID: "proj-dedup"})
	require.NoError(t, err)
	assert.True(t, r1.Created)

	r2, err := m.Store(ctx, &domain.StoreRequest{Content: content, ProjectID: "proj-dedup"})
	require.NoError(t, err)
	assert.False(t, r2.Created, "exact duplicate should not create new memory")
	assert.Equal(t, r1.Memory.ID, r2.Memory.ID)
}

// E2E Test 3: Lifecycle decay + archival
func TestE2E_LifecycleDecay(t *testing.T) {
	m := newTestMnemos(t)
	ctx := context.Background()

	// Store a memory
	result, err := m.Store(ctx, &domain.StoreRequest{
		Content: "Temporary note about the sprint planning meeting",
	})
	require.NoError(t, err)

	// Run maintenance
	err = m.Maintain(ctx, "")
	require.NoError(t, err)

	// Memory should still exist (just decayed slightly)
	mem, err := m.Get(ctx, result.Memory.ID)
	require.NoError(t, err)
	assert.NotNil(t, mem)
}

// E2E Test 4: Relation graph traversal
func TestE2E_RelationTraversal(t *testing.T) {
	m := newTestMnemos(t)
	ctx := context.Background()

	// Store two memories
	r1, err := m.Store(ctx, &domain.StoreRequest{Content: "Service A handles user authentication", ProjectID: "proj-graph"})
	require.NoError(t, err)

	r2, err := m.Store(ctx, &domain.StoreRequest{Content: "Service B depends on Service A for auth tokens", ProjectID: "proj-graph"})
	require.NoError(t, err)

	// Create relation
	rel, err := m.Relate(ctx, &domain.RelateRequest{
		SourceID:     r2.Memory.ID,
		TargetID:     r1.Memory.ID,
		RelationType: domain.RelationTypeDependsOn,
		Strength:     0.9,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, rel.ID)

	// Traverse from r2
	graph, err := m.Traverse(ctx, domain.GraphQuery{
		StartID:  r2.Memory.ID,
		MaxDepth: 2,
	})
	require.NoError(t, err)
	assert.NotEmpty(t, graph.Relations)
}

// E2E Test 5: Update and soft delete
func TestE2E_UpdateAndDelete(t *testing.T) {
	m := newTestMnemos(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content: "Initial content for update test",
	})
	require.NoError(t, err)

	// Update
	newContent := "Updated content with more details about the architecture decision"
	updated, err := m.Update(ctx, &domain.UpdateRequest{
		ID:      result.Memory.ID,
		Content: &newContent,
	})
	require.NoError(t, err)
	assert.Equal(t, newContent, updated.Content)

	// Soft delete
	err = m.Delete(ctx, result.Memory.ID)
	require.NoError(t, err)

	// Verify deleted status
	mem, err := m.Get(ctx, result.Memory.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.MemoryStatusDeleted, mem.Status)
}

// E2E Test 6: Stats
func TestE2E_Stats(t *testing.T) {
	m := newTestMnemos(t)
	ctx := context.Background()

	contents := []string{
		"PostgreSQL is the primary database with connection pooling enabled",
		"Redis cache layer handles session storage and rate limiting",
		"Kubernetes orchestrates container deployments across multiple nodes",
	}
	for _, content := range contents {
		_, err := m.Store(ctx, &domain.StoreRequest{
			Content:   content,
			ProjectID: "proj-stats",
		})
		require.NoError(t, err)
	}

	stats, err := m.Stats(ctx, "proj-stats")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, stats.Total, 3)
}

// E2E Test 7: Context assembly
func TestE2E_AssembleContext(t *testing.T) {
	m := newTestMnemos(t)
	ctx := context.Background()

	memories := []string{
		"The API gateway routes requests to microservices using path-based routing",
		"Authentication is handled by the auth service using OAuth2",
		"The database uses PostgreSQL with read replicas for scaling",
	}

	for _, content := range memories {
		_, err := m.Store(ctx, &domain.StoreRequest{Content: content, ProjectID: "proj-ctx"})
		require.NoError(t, err)
	}

	ctxResult, err := m.AssembleContext(ctx, "API gateway microservices", "proj-ctx", 2000, false)
	require.NoError(t, err)
	assert.NotNil(t, ctxResult)
	assert.Greater(t, ctxResult.TotalTokens, 0)
}

// E2E Test 8: List with filters
func TestE2E_ListWithFilters(t *testing.T) {
	m := newTestMnemos(t)
	ctx := context.Background()

	_, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "Long-term architecture decision about microservices",
		ProjectID: "proj-filter",
		Type:      domain.MemoryTypeLongTerm,
	})
	require.NoError(t, err)

	_, err = m.Store(ctx, &domain.StoreRequest{
		Content:   "Quick todo: fix the login bug",
		ProjectID: "proj-filter",
		Type:      domain.MemoryTypeShortTerm,
	})
	require.NoError(t, err)

	// List only long_term
	memories, err := m.List(ctx, storage.ListQuery{
		ProjectID: "proj-filter",
		Types:     []domain.MemoryType{domain.MemoryTypeLongTerm},
		Limit:     10,
	})
	require.NoError(t, err)
	for _, mem := range memories {
		assert.Equal(t, domain.MemoryTypeLongTerm, mem.Type)
	}
}
