package main

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core"
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

func buildEvalMnemos(t *testing.T) *core.Mnemos {
	t.Helper()
	db, err := sqlitestore.Open(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	store := sqlitestore.NewSQLiteStore(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	fts := sqlitestore.NewFTSSearcher(db)
	relStore := sqlitestore.NewRelationStore(db)
	embedder := embedding.NewNoopProvider(384)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))

	memMgr := coremem.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger, nil)
	t.Cleanup(func() { memMgr.Stop() })

	searchEng := search.NewSearchEngine(fts, embedStore, embedder, relStore, logger, 0.7, 0.0)
	relMgr := relation.NewManager(relStore, store, logger)
	lc := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, false, logger)
	return core.NewMnemos(memMgr, searchEng, relMgr, lc, store, logger)
}

func TestEval_EmptyProject(t *testing.T) {
	mn := buildEvalMnemos(t)
	cmd := newEvalCmd(mn)

	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--project", "nonexistent"})
		_ = cmd.Execute()
	})
	assert.Contains(t, out, "Total: 0 active")
}

func TestEval_WithMemories(t *testing.T) {
	mn := buildEvalMnemos(t)
	ctx := context.Background()

	memories := []struct {
		content  string
		category string
		memType  domain.MemoryType
	}{
		{"Go interfaces are defined at the point of use", "code", domain.MemoryTypeLongTerm},
		{"Use context.Context as first parameter in Go", "code", domain.MemoryTypeLongTerm},
		{"PostgreSQL connection pooling with pgx", "database", domain.MemoryTypeLongTerm},
		{"Redis caching with TTL for session management", "database", domain.MemoryTypeLongTerm},
		{"Kubernetes deployment with Helm charts", "deployment", domain.MemoryTypeEpisodic},
	}

	for _, m := range memories {
		_, err := mn.Store(ctx, &domain.StoreRequest{
			Content:   m.content,
			Category:  m.category,
			Type:      m.memType,
			ProjectID: "eval-test",
		})
		require.NoError(t, err)
	}

	cmd := newEvalCmd(mn)

	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--project", "eval-test"})
		_ = cmd.Execute()
	})

	assert.Contains(t, out, "Knowledge Base Evaluation")
	assert.Contains(t, out, "Total: 5 active")
	assert.Contains(t, out, "Quality Score Distribution")
	assert.Contains(t, out, "Memory Type Distribution")
	assert.Contains(t, out, "Category Coverage")
	assert.Contains(t, out, "Duplication")
	assert.Contains(t, out, "Staleness")
	assert.Contains(t, out, "Overall Score")
	assert.Contains(t, out, "code")
	assert.Contains(t, out, "database")
	assert.Contains(t, out, "A (")
}

func TestEval_DuplicationDetection(t *testing.T) {
	mn := buildEvalMnemos(t)
	ctx := context.Background()

	// Two memories about the same concept but worded differently enough
	// to pass fuzzy Jaccard (< 0.85 threshold) yet similar enough for the
	// eval duplication check (>= 0.75 threshold).
	_, err := mn.Store(ctx, &domain.StoreRequest{
		Content:   "JWT authentication tokens should use RS256 signing with a one hour expiration window",
		ProjectID: "eval-dup",
	})
	require.NoError(t, err)

	_, err = mn.Store(ctx, &domain.StoreRequest{
		Content:   "Configure RS256-signed JWT tokens to expire after sixty minutes for security compliance",
		ProjectID: "eval-dup",
	})
	require.NoError(t, err)

	cmd := newEvalCmd(mn)

	out := captureStdout(t, func() {
		cmd.SetArgs([]string{"--project", "eval-dup"})
		_ = cmd.Execute()
	})

	assert.Contains(t, out, "Total: 2 active")
	hasDupPair := strings.Contains(out, "duplicate pairs: 1") || strings.Contains(out, "duplicate pair")
	assert.True(t, hasDupPair, "expected duplication to be reported, got:\n%s", out)
}
