package memory_test

import (
	"context"
	"database/sql"
	"hash/fnv"
	"math"
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

func newDedupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestMemoryForDedup(content, projectID string) *domain.Memory {
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
	dedup := memory.NewContentDedup(store, nil, nil, 0.85, 0.92)

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
	dedup := memory.NewContentDedup(store, nil, nil, 0.7, 0.92)

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
	dedup := memory.NewContentDedup(store, nil, nil, 0.85, 0.92)

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
	dedup := memory.NewContentDedup(store, nil, nil, 0.85, 0.92)

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

// wordEmbedder produces a vector from the first word of the content.
// Same first word → identical embedding; different first word → orthogonal.
// This gives predictable semantic grouping without needing a real embedding model.
type wordEmbedder struct{ dims int }

func (w *wordEmbedder) Name() string                   { return "word" }
func (w *wordEmbedder) Dimensions() int                { return w.dims }
func (w *wordEmbedder) Healthy(_ context.Context) bool { return true }

func (w *wordEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, t := range texts {
		v, _ := w.Embed(context.Background(), t)
		result[i] = v
	}
	return result, nil
}

func firstWord(s string) string {
	for i, c := range s {
		if c == ' ' || c == '\t' || c == '\n' {
			return s[:i]
		}
	}
	return s
}

func (w *wordEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	v := make([]float32, w.dims)
	word := firstWord(text)
	if len(word) == 0 {
		return v, nil
	}
	hasher := fnv.New64a()
	hasher.Write([]byte(word))
	seed := hasher.Sum64()
	for i := range v {
		seed = seed*6364136223846793005 + 1442695040888963407
		v[i] = float32(int64(seed)%10001) / 5000.0 - 1.0
	}
	var sum float64
	for _, f := range v {
		sum += float64(f) * float64(f)
	}
	if norm := float32(math.Sqrt(sum)); norm > 0 {
		for i := range v {
			v[i] /= norm
		}
	}
	return v, nil
}

func TestContentDedup_SemanticMatch(t *testing.T) {
	db := newDedupDB(t)
	store := sqlitestore.NewSQLiteStore(db)
	emb := sqlitestore.NewEmbeddingStore(db)
	ctx := context.Background()

	embedder := &wordEmbedder{dims: 128}
	dedup := memory.NewContentDedup(store, emb, embedder, 0.95, 0.0)

	// Store a memory about JWT
	original := "JWT tokens use RS256 with a one-hour expiry period"
	mem := newTestMemoryForDedup(original, "proj1")
	require.NoError(t, store.Create(ctx, mem))
	vec, err := embedder.Embed(ctx, mem.Content)
	require.NoError(t, err)
	require.NoError(t, emb.StoreEmbedding(ctx, mem.ID, vec))

	// Different content but same first word "JWT" → same embedding → tier-3 match
	similar := "JWT is the authentication mechanism used across all microservices"
	req := &domain.StoreRequest{Content: similar, ProjectID: "proj1"}
	hash := util.ContentHash(similar, "proj1")
	existing, simType, score, _, err := dedup.Check(ctx, req, hash)
	require.NoError(t, err)
	assert.NotNil(t, existing)
	assert.Equal(t, "semantic", simType)
	assert.GreaterOrEqual(t, score, 0.0)
	assert.Equal(t, mem.ID, existing.ID)
}

func TestContentDedup_SemanticNoMatch(t *testing.T) {
	db := newDedupDB(t)
	store := sqlitestore.NewSQLiteStore(db)
	emb := sqlitestore.NewEmbeddingStore(db)
	ctx := context.Background()

	embedder := &wordEmbedder{dims: 128}
	dedup := memory.NewContentDedup(store, emb, embedder, 0.95, 0.0)

	mem := newTestMemoryForDedup("Redis handles session caching with TTL expiry", "proj1")
	require.NoError(t, store.Create(ctx, mem))
	vec, err := embedder.Embed(ctx, mem.Content)
	require.NoError(t, err)
	require.NoError(t, emb.StoreEmbedding(ctx, mem.ID, vec))

	// Different first word → different embedding → no match
	req := &domain.StoreRequest{Content: "Kubernetes orchestrates container deployments across nodes", ProjectID: "proj1"}
	hash := util.ContentHash(req.Content, "proj1")
	existing, _, _, _, err := dedup.Check(ctx, req, hash)
	require.NoError(t, err)
	assert.Nil(t, existing)
}

func TestContentDedup_SemanticSkipsWhenNoEmbedder(t *testing.T) {
	store := newDedupStore(t)
	ctx := context.Background()

	dedup := memory.NewContentDedup(store, nil, nil, 0.85, 0.92)

	content := "Some unique technical fact about database indexing"
	mem := newTestMemoryForDedup(content, "proj1")
	require.NoError(t, store.Create(ctx, mem))

	req := &domain.StoreRequest{Content: "Database indexing techniques for query optimization", ProjectID: "proj1"}
	hash := util.ContentHash(req.Content, "proj1")
	existing, _, _, _, err := dedup.Check(ctx, req, hash)
	require.NoError(t, err)
	assert.Nil(t, existing) // no embedder → tier 3 skipped, fuzzy below threshold
}
