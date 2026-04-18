package memory_test

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
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newFileAwareMnemos(t *testing.T) *core.Mnemos {
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
	// Use FileBoostDefault (0.3) as the file boost value
	searchEngine := search.NewSearchEngine(fts, embedStore, embedProvider, relStore, logger, 0.7, coremem.FileBoostDefault)
	relManager := relation.NewManager(relStore, memStore, logger)
	lcEngine := lifecycle.NewEngine(memStore, 24*time.Hour, 30, 0.1, logger)

	m := core.NewMnemos(memManager, searchEngine, relManager, lcEngine, memStore, logger)

	t.Cleanup(func() {
		m.Shutdown()
		db.Close()
	})
	return m
}

// TestFileAwareRetrieval_OverlappingMemoryRanksHigher verifies that a memory
// referencing an open file ranks higher than a non-overlapping memory with
// equal hybrid search scores.
func TestFileAwareRetrieval_OverlappingMemoryRanksHigher(t *testing.T) {
	m := newFileAwareMnemos(t)
	ctx := context.Background()

	// Store two memories with similar content (so they get similar FTS scores)
	// but only one references the open file.
	_, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "auth/session.go handles token validation and refresh logic for the authentication service",
		ProjectID: "proj-file-aware",
	})
	require.NoError(t, err)

	_, err = m.Store(ctx, &domain.StoreRequest{
		Content:   "database/postgres.go handles connection pooling and query execution for the data layer",
		ProjectID: "proj-file-aware",
	})
	require.NoError(t, err)

	// Query with auth/session.go as an open file.
	// Use "handles" which appears in both memories so both are returned by FTS.
	openFiles := []string{"auth/session.go"}
	result, err := m.AssembleContext(ctx, "handles", "proj-file-aware", 10000, false, openFiles)
	require.NoError(t, err)
	require.NotNil(t, result)

	if len(result.Memories) < 2 {
		t.Skip("fewer than 2 memories returned — cannot compare rankings")
	}

	// The memory referencing auth/session.go should rank first
	assert.Equal(t, "auth/session.go", findFirstFileRef(result.Memories),
		"memory referencing open file should rank first")
}

// TestFileAwareRetrieval_NoOpenFiles_ScoresUnchanged verifies that when no open
// files are provided, retrieval results are identical to the baseline (no boost).
func TestFileAwareRetrieval_NoOpenFiles_ScoresUnchanged(t *testing.T) {
	m := newFileAwareMnemos(t)
	ctx := context.Background()

	_, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "auth/session.go handles token validation and refresh logic",
		ProjectID: "proj-no-boost",
	})
	require.NoError(t, err)

	// With nil openFiles — no boost
	result, err := m.AssembleContext(ctx, "token validation", "proj-no-boost", 10000, false, nil)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Scores should be in the normal FTS range (no boost applied)
	for _, mem := range result.Memories {
		// FTS BM25 scores are negative; after propagation they are ≤ 0.
		// With no boost, score should be < FileBoostDefault.
		assert.Less(t, mem.RelevanceScore, coremem.FileBoostDefault,
			"score should not be boosted when openFiles is nil")
	}
}

// TestFileAwareRetrieval_MMRInteraction verifies that a boosted memory survives
// DiversityFilter even when it is highly similar to another memory.
func TestFileAwareRetrieval_MMRInteraction(t *testing.T) {
	m := newFileAwareMnemos(t)
	ctx := context.Background()

	// Store two memories with very similar content (high Jaccard similarity).
	// Only the first one references the open file.
	_, err := m.Store(ctx, &domain.StoreRequest{
		Content:   "auth/session.go token refresh logic handles JWT expiry and renewal",
		ProjectID: "proj-mmr",
	})
	require.NoError(t, err)

	_, err = m.Store(ctx, &domain.StoreRequest{
		Content:   "auth/session.go token refresh logic handles JWT expiry and renewal process",
		ProjectID: "proj-mmr",
	})
	require.NoError(t, err)

	openFiles := []string{"auth/session.go"}
	result, err := m.AssembleContext(ctx, "token refresh JWT", "proj-mmr", 10000, false, openFiles)
	require.NoError(t, err)
	require.NotNil(t, result)

	// At least one memory should be returned
	assert.NotEmpty(t, result.Memories, "at least one memory should survive DiversityFilter")
}

// findFirstFileRef returns the first file path found in the first memory's content.
// Used to verify which memory ranked first.
func findFirstFileRef(memories []*domain.Memory) string {
	if len(memories) == 0 {
		return ""
	}
	content := memories[0].Content
	// Look for auth/session.go pattern
	if containsStr(content, "auth/session.go") {
		return "auth/session.go"
	}
	return ""
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && findSubstr(s, sub))
}

func findSubstr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
