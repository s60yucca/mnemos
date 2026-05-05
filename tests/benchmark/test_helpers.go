package benchmark_test

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
)

func buildTestMnemos(t *testing.T) *core.Mnemos {
	t.Helper()

	db, err := sqlitestore.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	store := sqlitestore.NewSQLiteStore(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	fts := sqlitestore.NewFTSSearcher(db)
	relStore := sqlitestore.NewRelationStore(db)

	embedder := embedding.NewNoopProvider(384)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	memMgr := coremem.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger, nil)
	t.Cleanup(func() { memMgr.Stop() })

	searchEng := search.NewSearchEngine(fts, embedStore, embedder, relStore, logger, 0.7, 0.0)
	relMgr := relation.NewManager(relStore, store, logger)
	lcEngine := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, logger)

	return core.NewMnemos(memMgr, searchEng, relMgr, lcEngine, store, logger)
}

