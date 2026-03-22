package memory_test

import (
	"context"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/domain"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

func newDedupStore(t *testing.T) *sqlitestore.SQLiteStore {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return sqlitestore.NewSQLiteStore(db)
}

func newTestMemoryForDedup(content, projectID string) *domain.Memory {
	now := util.NewID()
	_ = now
	mem := &domain.Memory{
		ID:             util.NewID(),
		Content:        content,
		Type:           domain.MemoryTypeEpisodic,
		Category:       domain.CategoryGeneral,
		Tags:           []string{"test"},
		ProjectID:      projectID,
		RelevanceScore: 1.0,
		Status:         domain.MemoryStatusActive,
		ContentHash:    util.ContentHash(content, projectID),
	}
	return mem
}

func TestContentDedup_ExactMatch(t *testing.T) {
	store := newDedupStore(t)
	ctx := context.Background()
	dedup := memory.NewContentDedup(store, nil, 0.85, 0.92)

	content := "PostgreSQL is the primary database"
	hash := util.ContentHash(content, "proj1")
	mem := newTestMemoryForDedup(content, "proj1")
	require.NoError(t, store.Create(ctx, mem))

	req := &domain.StoreRequest{Content: content, ProjectID: "proj1"}
	existing, simType, score, _, err := dedup.Check(ctx, req, hash)
	require.NoError(t, err)
	assert.NotNil(t, existing)
	assert.Equal(t, "exact", simType)
	assert.Equal(t, 1.0, score)
	assert.Equal(t, mem.ID, existing.ID)
}

func TestContentDedup_FuzzyMatch(t *testing.T) {
	store := newDedupStore(t)
	ctx := context.Background()
	dedup := memory.NewContentDedup(store, nil, 0.7, 0.92)

	original := "PostgreSQL is the primary database with connection pooling enabled"
	mem := newTestMemoryForDedup(original, "proj1")
	require.NoError(t, store.Create(ctx, mem))

	similar := "PostgreSQL is the primary database with connection pooling"
	newHash := util.ContentHash(similar, "proj1")
	req := &domain.StoreRequest{Content: similar, ProjectID: "proj1"}
	existing, simType, score, _, err := dedup.Check(ctx, req, newHash)
	require.NoError(t, err)
	assert.NotNil(t, existing)
	assert.Equal(t, "fuzzy", simType)
	assert.GreaterOrEqual(t, score, 0.7)
}

func TestContentDedup_NoDupDifferentProject(t *testing.T) {
	store := newDedupStore(t)
	ctx := context.Background()
	dedup := memory.NewContentDedup(store, nil, 0.85, 0.92)

	content := "PostgreSQL is the primary database"
	mem := newTestMemoryForDedup(content, "proj1")
	require.NoError(t, store.Create(ctx, mem))

	req := &domain.StoreRequest{Content: content, ProjectID: "proj2"}
	hash := util.ContentHash(content, "proj2")
	existing, _, _, _, err := dedup.Check(ctx, req, hash)
	require.NoError(t, err)
	assert.Nil(t, existing)
}

func TestContentDedup_BelowThreshold(t *testing.T) {
	store := newDedupStore(t)
	ctx := context.Background()
	dedup := memory.NewContentDedup(store, nil, 0.85, 0.92)

	mem := newTestMemoryForDedup("Redis handles session caching with TTL expiry", "proj1")
	require.NoError(t, store.Create(ctx, mem))

	req := &domain.StoreRequest{Content: "Kubernetes orchestrates container deployments", ProjectID: "proj1"}
	hash := util.ContentHash(req.Content, "proj1")
	existing, _, _, _, err := dedup.Check(ctx, req, hash)
	require.NoError(t, err)
	assert.Nil(t, existing)
}

// Property: Jaccard score is always in [0, 1]
func TestContentDedup_FuzzyScoreProperty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.StringMatching(`[a-z ]{5,50}`).Draw(t, "a")
		b := rapid.StringMatching(`[a-z ]{5,50}`).Draw(t, "b")

		tokA := util.TokenSet(util.Tokenize(a))
		tokB := util.TokenSet(util.Tokenize(b))
		score := util.JaccardSimilarity(tokA, tokB)
		if score < 0 || score > 1 {
			t.Fatalf("jaccard score %f out of [0,1]", score)
		}
	})
}
