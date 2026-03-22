package memory_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestManagerWithGate creates a Manager with the default QualityGate enabled.
func newTestManagerWithGate(t *testing.T) *memory.Manager {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	store := sqlitestore.NewSQLiteStore(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	embedder := embedding.NewNoopProvider(384)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	cfg := config.DefaultConfig().QualityGate
	gate := memory.NewQualityGate(cfg)

	m := memory.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger, gate)
	t.Cleanup(func() {
		m.Stop()
		db.Close()
	})
	return m
}

// newTestManagerWithGateConfig creates a Manager with a custom QualityGateConfig.
func newTestManagerWithGateConfig(t *testing.T, cfg config.QualityGateConfig) *memory.Manager {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	store := sqlitestore.NewSQLiteStore(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	embedder := embedding.NewNoopProvider(384)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	gate := memory.NewQualityGate(cfg)

	m := memory.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger, gate)
	t.Cleanup(func() {
		m.Stop()
		db.Close()
	})
	return m
}

// newTestManagerNoGate creates a Manager with gate=nil (disabled).
func newTestManagerNoGate(t *testing.T) *memory.Manager {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)

	store := sqlitestore.NewSQLiteStore(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	embedder := embedding.NewNoopProvider(384)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	m := memory.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger, nil)
	t.Cleanup(func() {
		m.Stop()
		db.Close()
	})
	return m
}

// TestQualityGate_TooShort verifies that a memory with fewer than min_words is rejected.
func TestQualityGate_TooShort(t *testing.T) {
	m := newTestManagerWithGate(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "fix bug",
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Created, "too-short memory should not be created")
}

// TestQualityGate_GoodMemory verifies that a high-quality memory is stored and retrievable.
func TestQualityGate_GoodMemory(t *testing.T) {
	m := newTestManagerWithGate(t)
	ctx := context.Background()

	content := "SessionStore.Close() needs mutex for thread safety in auth/session.go"
	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   content,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Created, "good memory should be created")
	assert.NotEmpty(t, result.Memory.ID)

	// Verify it can be retrieved from DB
	retrieved, err := m.Get(ctx, result.Memory.ID)
	require.NoError(t, err)
	assert.Equal(t, content, retrieved.Content)
}

// TestQualityGate_VerboseMemory verifies that a 250+ word memory gets a Summary set.
func TestQualityGate_VerboseMemory(t *testing.T) {
	m := newTestManagerWithGate(t)
	ctx := context.Background()

	// Build 260 unique tokens across multiple sentences — high density, 260+ words → triggers TooLong → FixSummarize.
	// Use sentence boundaries so first2Sentences produces a shorter summary.
	var sentences []string
	for i := 0; i < 26; i++ {
		words := make([]string, 10)
		for j := range words {
			words[j] = fmt.Sprintf("token%d_%d", i, j)
		}
		sentences = append(sentences, strings.Join(words, " ")+".")
	}
	content := strings.Join(sentences, " ")

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   content,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Created, "verbose memory should be created (with summary fix)")
	assert.NotEmpty(t, result.Memory.Summary, "summary should be set for verbose memory")
	assert.Less(t, len(result.Memory.Summary), len(content), "summary should be shorter than content")
}

// TestQualityGate_GenericLongTerm verifies that a generic long_term memory is downgraded to short_term.
func TestQualityGate_GenericLongTerm(t *testing.T) {
	m := newTestManagerWithGate(t)
	ctx := context.Background()

	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "The project has good error handling",
		Type:      domain.MemoryTypeLongTerm,
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Created, "downgraded memory should still be created")
	assert.Equal(t, domain.MemoryTypeShortTerm, result.Memory.Type, "type should be downgraded to short_term")
	assert.True(t, result.Memory.HasTag("auto-downgraded"), "auto-downgraded tag should be present")
}

// TestQualityGate_NearDuplicate verifies that a near-duplicate memory is rejected with the existing ID.
func TestQualityGate_NearDuplicate(t *testing.T) {
	cfg := config.DefaultConfig().QualityGate
	cfg.DuplicateThreshold = 0.5
	m := newTestManagerWithGateConfig(t, cfg)
	ctx := context.Background()

	// Store the first memory
	first, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "Auth uses JWT tokens for authentication",
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.True(t, first.Created)

	// Store a near-duplicate
	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "Auth uses JWT tokens for authentication and authorization",
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.Created, "near-duplicate should not be created")
	// The result should reference the existing memory ID
	require.NotNil(t, result.Memory, "result.Memory should contain the existing memory reference")
	assert.Equal(t, first.Memory.ID, result.Memory.ID, "result should contain the existing memory ID")
}

// TestQualityGate_Disabled verifies that with gate=nil, all memories pass through unchanged.
func TestQualityGate_Disabled(t *testing.T) {
	m := newTestManagerNoGate(t)
	ctx := context.Background()

	// "hi there" is 2 words — would fail the gate (min_words=5) but gate is nil
	result, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "hi there",
		ProjectID: "proj1",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Created, "with gate disabled, short memory should be created")
}

// TestQualityGate_EndToEnd stores 10 memories of varying quality and verifies the expected distribution.
func TestQualityGate_EndToEnd(t *testing.T) {
	cfg := config.DefaultConfig().QualityGate
	cfg.DuplicateThreshold = 0.5
	m := newTestManagerWithGateConfig(t, cfg)
	ctx := context.Background()

	type testCase struct {
		req           *domain.StoreRequest
		expectCreated bool
		expectSummary bool
		expectType    domain.MemoryType
		expectTag     string
	}

	// Build verbose content (260 words across multiple sentences for proper summarization)
	var verboseSentences []string
	for i := 0; i < 26; i++ {
		words := make([]string, 10)
		for j := range words {
			words[j] = fmt.Sprintf("verboseToken%d_%d", i, j)
		}
		verboseSentences = append(verboseSentences, strings.Join(words, " ")+".")
	}
	verboseContent := strings.Join(verboseSentences, " ")

	var verboseSentences2 []string
	for i := 0; i < 26; i++ {
		words := make([]string, 10)
		for j := range words {
			words[j] = fmt.Sprintf("longToken%d_%d", i, j)
		}
		verboseSentences2 = append(verboseSentences2, strings.Join(words, " ")+".")
	}
	verboseContent2 := strings.Join(verboseSentences2, " ")

	cases := []testCase{
		// 2 good memories — specific identifiers, 7+ words
		{
			req:           &domain.StoreRequest{Content: "SessionStore.Close() needs mutex for thread safety in auth/session.go", ProjectID: "proj1"},
			expectCreated: true,
		},
		{
			req:           &domain.StoreRequest{Content: "JWT_SECRET rotation in config/auth.go requires zero-downtime deployment", ProjectID: "proj1"},
			expectCreated: true,
		},
		// 2 too-short memories
		{
			req:           &domain.StoreRequest{Content: "fix bug", ProjectID: "proj1"},
			expectCreated: false,
		},
		{
			req:           &domain.StoreRequest{Content: "todo later", ProjectID: "proj1"},
			expectCreated: false,
		},
		// 2 generic long_term memories → downgraded to short_term
		{
			req:           &domain.StoreRequest{Content: "The project has good error handling overall", Type: domain.MemoryTypeLongTerm, ProjectID: "proj1"},
			expectCreated: true,
			expectType:    domain.MemoryTypeShortTerm,
			expectTag:     "auto-downgraded",
		},
		{
			req:           &domain.StoreRequest{Content: "We have a good testing strategy in place", Type: domain.MemoryTypeLongTerm, ProjectID: "proj1"},
			expectCreated: true,
			expectType:    domain.MemoryTypeShortTerm,
			expectTag:     "auto-downgraded",
		},
		// 2 verbose memories → created with summary
		{
			req:           &domain.StoreRequest{Content: verboseContent, ProjectID: "proj1"},
			expectCreated: true,
			expectSummary: true,
		},
		{
			req:           &domain.StoreRequest{Content: verboseContent2, ProjectID: "proj1"},
			expectCreated: true,
			expectSummary: true,
		},
		// 2 near-duplicates of the good memories (stored after the originals)
		{
			req:           &domain.StoreRequest{Content: "SessionStore.Close() needs mutex for thread safety in auth/session.go and more", ProjectID: "proj1"},
			expectCreated: false,
		},
		{
			req:           &domain.StoreRequest{Content: "JWT_SECRET rotation in config/auth.go requires zero-downtime deployment strategy", ProjectID: "proj1"},
			expectCreated: false,
		},
	}

	createdCount := 0
	rejectedCount := 0

	for i, tc := range cases {
		result, err := m.Store(ctx, tc.req)
		require.NoError(t, err, "case %d should not error", i)
		require.NotNil(t, result, "case %d result should not be nil", i)

		if tc.expectCreated {
			assert.True(t, result.Created, "case %d: expected Created=true", i)
			createdCount++

			if tc.expectSummary {
				assert.NotEmpty(t, result.Memory.Summary, "case %d: expected non-empty summary", i)
				assert.Less(t, len(result.Memory.Summary), len(tc.req.Content), "case %d: summary should be shorter than content", i)
			}
			if tc.expectType != "" {
				assert.Equal(t, tc.expectType, result.Memory.Type, "case %d: unexpected memory type", i)
			}
			if tc.expectTag != "" {
				assert.True(t, result.Memory.HasTag(tc.expectTag), "case %d: expected tag %q", i, tc.expectTag)
			}
		} else {
			assert.False(t, result.Created, "case %d: expected Created=false", i)
			rejectedCount++
		}
	}

	// 6 created (2 good + 2 generic downgraded + 2 verbose), 4 rejected (2 short + 2 near-dup)
	assert.Equal(t, 6, createdCount, "expected 6 memories created")
	assert.Equal(t, 4, rejectedCount, "expected 4 memories rejected/not created")
}
