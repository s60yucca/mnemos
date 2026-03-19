package memory_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func newTestManager(t *testing.T) *memory.Manager {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	store := sqlitestore.NewSQLiteStore(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	embedder := embedding.NewNoopProvider(384)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m := memory.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger)
	t.Cleanup(func() {
		m.Stop()
		db.Close()
	})
	return m
}

func newTestManagerNoEmbed(t *testing.T) *memory.Manager {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	store := sqlitestore.NewSQLiteStore(db)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m := memory.NewManager(store, nil, nil, mir, 0.85, 0.92, logger)
	t.Cleanup(func() {
		m.Stop()
		db.Close()
	})
	return m
}

func TestManager_Store_BasicPipeline(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "JWT authentication uses RS256 signing with 1h expiry",
		ProjectID: "proj1",
		Tags:      []string{"auth", "jwt"},
	})
	require.NoError(t, err)
	assert.True(t, result.Created)
	assert.NotEmpty(t, result.Memory.ID)
	assert.NotEmpty(t, result.Memory.ContentHash)
	assert.Equal(t, domain.MemoryStatusActive, result.Memory.Status)
	assert.Equal(t, 1.0, result.Memory.RelevanceScore)
}

func TestManager_Store_AutoClassify(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content: "We decided to use PostgreSQL as the primary database",
	})
	require.NoError(t, err)
	// Should auto-classify as long_term (decision keyword)
	assert.Equal(t, domain.MemoryTypeLongTerm, result.Memory.Type)
}

func TestManager_Store_ExactDedupReturnsExisting(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	content := "Redis handles session caching with 30 minute TTL"
	r1, err := m.Store(ctx, &domain.StoreRequest{Content: content, ProjectID: "proj1"})
	require.NoError(t, err)
	assert.True(t, r1.Created)

	r2, err := m.Store(ctx, &domain.StoreRequest{Content: content, ProjectID: "proj1"})
	require.NoError(t, err)
	assert.False(t, r2.Created)
	assert.Equal(t, r1.Memory.ID, r2.Memory.ID)
}

func TestManager_Store_ValidationError(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	_, err := m.Store(ctx, &domain.StoreRequest{Content: ""})
	assert.Error(t, err)
}

func TestManager_Get_TouchesAccess(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{Content: "some content"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Memory.AccessCount)

	mem, err := m.Get(ctx, result.Memory.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, mem.AccessCount)
}

func TestManager_Update_ContentTriggersReclassify(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content: "TODO: fix the login bug",
	})
	require.NoError(t, err)
	assert.Equal(t, domain.MemoryTypeShortTerm, result.Memory.Type)

	newContent := "We decided to use OAuth2 for authentication across all services"
	updated, err := m.Update(ctx, &domain.UpdateRequest{
		ID:      result.Memory.ID,
		Content: &newContent,
	})
	require.NoError(t, err)
	assert.Equal(t, newContent, updated.Content)
	// Should reclassify to long_term (decision keyword)
	assert.Equal(t, domain.MemoryTypeLongTerm, updated.Type)
}

func TestManager_Delete_SoftDelete(t *testing.T) {
	m := newTestManager(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{Content: "to be deleted"})
	require.NoError(t, err)

	require.NoError(t, m.Delete(ctx, result.Memory.ID))

	mem, err := m.Get(ctx, result.Memory.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.MemoryStatusDeleted, mem.Status)
}

func TestManager_Stop_SafeWithNilEmbedder(t *testing.T) {
	m := newTestManagerNoEmbed(t)
	// Should not panic
	assert.NotPanics(t, func() { m.Stop() })
}

func TestManager_Stop_SafeToCallOnce(t *testing.T) {
	m := newTestManager(t)
	assert.NotPanics(t, func() { m.Stop() })
}

// Property: storing same content N times always returns same ID
func TestManager_Store_IdempotentProperty(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		m := newTestManager(t)
		ctx := context.Background()

		content := rapid.StringMatching(`[a-zA-Z0-9 ]{10,100}`).Draw(rt, "content")
		projectID := "prop-test"

		r1, err := m.Store(ctx, &domain.StoreRequest{Content: content, ProjectID: projectID})
		if err != nil {
			return // validation may reject some inputs
		}

		r2, err := m.Store(ctx, &domain.StoreRequest{Content: content, ProjectID: projectID})
		if err != nil {
			return
		}

		if r1.Memory.ID != r2.Memory.ID {
			rt.Fatalf("same content produced different IDs: %s vs %s", r1.Memory.ID, r2.Memory.ID)
		}
	})
}
