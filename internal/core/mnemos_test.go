package core_test

import (
	"log/slog"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	"github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDeps creates all real dependencies backed by an in-memory SQLite database.
func newTestDeps(t *testing.T) (
	*memory.Manager,
	*search.SearchEngine,
	*relation.Manager,
	*lifecycle.Engine,
	*sqlitestore.SQLiteStore,
	*slog.Logger,
) {
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
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	memMgr := memory.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger, nil)
	t.Cleanup(func() { memMgr.Stop() })

	searchEng := search.NewSearchEngine(fts, embedStore, embedder, relStore, logger)
	relMgr := relation.NewManager(relStore, store, logger)
	lcEngine := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, logger)

	return memMgr, searchEng, relMgr, lcEngine, store, logger
}

// TestInitMode_Light_NoWorkers verifies that NewMnemosWithMode with InitLight does NOT
// start background workers (no extra goroutine is spawned) and that Shutdown() is safe.
func TestInitMode_Light_NoWorkers(t *testing.T) {
	memMgr, searchEng, relMgr, lcEngine, store, logger := newTestDeps(t)

	// Allow any goroutines from setup to settle.
	runtime.Gosched()
	before := runtime.NumGoroutine()

	m := core.NewMnemosWithMode(
		core.InitLight,
		memMgr,
		searchEng,
		relMgr,
		lcEngine,
		store,
		nil, // no embedding provider
		logger,
	)

	require.NotNil(t, m, "Mnemos instance should be created successfully")

	// Give any potential goroutine a moment to appear.
	time.Sleep(10 * time.Millisecond)
	after := runtime.NumGoroutine()

	// InitLight must not start the lifecycle background worker goroutine.
	assert.LessOrEqual(t, after, before,
		"InitLight should not start background workers (goroutine count should not increase)")

	// Shutdown must not panic even though Start() was never called.
	assert.NotPanics(t, func() { m.Shutdown() })
}

// TestInitMode_Full_StartsWorkers verifies that NewMnemosWithMode with InitFull DOES
// start background workers (an extra goroutine is spawned by lifecycle.Start()).
func TestInitMode_Full_StartsWorkers(t *testing.T) {
	memMgr, searchEng, relMgr, lcEngine, store, logger := newTestDeps(t)

	// Allow any goroutines from setup to settle.
	runtime.Gosched()
	before := runtime.NumGoroutine()

	m := core.NewMnemosWithMode(
		core.InitFull,
		memMgr,
		searchEng,
		relMgr,
		lcEngine,
		store,
		embedding.NewNoopProvider(384),
		logger,
	)

	require.NotNil(t, m, "Mnemos instance should be created successfully")

	// Give the goroutine time to start.
	time.Sleep(10 * time.Millisecond)
	after := runtime.NumGoroutine()

	// InitFull must start at least one background worker goroutine.
	assert.Greater(t, after, before,
		"InitFull should start background workers (goroutine count should increase)")

	// Shutdown must stop the workers cleanly.
	assert.NotPanics(t, func() { m.Shutdown() })
}
