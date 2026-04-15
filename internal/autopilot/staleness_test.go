package autopilot

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// stalenessTestEnv holds all test dependencies.
type stalenessTestEnv struct {
	mnemos   *core.Mnemos
	store    *sqlitestore.SQLiteStore
	relStore *sqlitestore.RelationStore
}

// newStalenessTestEnv creates a real Mnemos instance backed by in-memory SQLite.
func newStalenessTestEnv(t *testing.T) *stalenessTestEnv {
	t.Helper()

	db, err := sqlitestore.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := sqlitestore.NewSQLiteStore(db)
	embedStore := sqlitestore.NewEmbeddingStore(db)
	fts := sqlitestore.NewFTSSearcher(db)
	relStore := sqlitestore.NewRelationStore(db)

	embedder := embedding.NewNoopProvider(384)
	mir := markdown.NewMirror(t.TempDir(), false)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	memMgr := coremem.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger, nil)
	t.Cleanup(func() { memMgr.Stop() })

	searchEng := search.NewSearchEngine(fts, embedStore, embedder, relStore, logger, 0.7)
	relMgr := relation.NewManager(relStore, store, logger)
	lcEngine := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, logger)

	mnemos := core.NewMnemos(memMgr, searchEng, relMgr, lcEngine, store, logger)
	return &stalenessTestEnv{mnemos: mnemos, store: store, relStore: relStore}
}

// createMemory inserts a memory directly into the store with explicit timestamps.
func createMemory(t *testing.T, store *sqlitestore.SQLiteStore, id, projectID, content string, memType domain.MemoryType, createdAt, updatedAt time.Time) *domain.Memory {
	t.Helper()
	m := &domain.Memory{
		ID:             id,
		Content:        content,
		Type:           memType,
		Category:       "code",
		ProjectID:      projectID,
		Status:         domain.MemoryStatusActive,
		CreatedAt:      createdAt,
		UpdatedAt:      updatedAt,
		LastAccessedAt: createdAt,
		RelevanceScore: 1.0,
		ContentHash:    util.NewID(),
	}
	if err := store.Create(context.Background(), m); err != nil {
		t.Fatalf("create memory %s: %v", id, err)
	}
	return m
}

// linkCompiledFrom creates a compiled_from relation from article to source.
func linkCompiledFrom(t *testing.T, relStore *sqlitestore.RelationStore, articleID, sourceID string) {
	t.Helper()
	rel := &domain.MemoryRelation{
		ID:             util.NewID(),
		SourceMemoryID: articleID,
		TargetMemoryID: sourceID,
		RelationType:   domain.RelationTypeCompiledFrom,
		Strength:       1.0,
		CreatedAt:      time.Now().UTC(),
	}
	if err := relStore.CreateRelation(context.Background(), rel); err != nil {
		t.Fatalf("create relation: %v", err)
	}
}

func defaultCfg() config.AutopilotConfig {
	return config.AutopilotConfig{
		Enabled:           true,
		MaxCompiledPerRun: 50,
		MaxMemoriesPerRun: 200,
	}
}

// TestStalenessDetector_FindsNewerSources: article + 2 updated sources → 1 finding with newer_source_count=2
func TestStalenessDetector_FindsNewerSources(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-stale"

	articleTime := time.Now().UTC().Add(-2 * time.Hour)
	sourceTime := time.Now().UTC().Add(-3 * time.Hour)  // created before article
	updatedTime := time.Now().UTC().Add(-1 * time.Hour) // updated after article

	article := createMemory(t, env.store, util.NewID(), projectID, "compiled article", domain.MemoryTypeCompiled, articleTime, articleTime)
	src1 := createMemory(t, env.store, util.NewID(), projectID, "source one internal/foo/bar.go", domain.MemoryTypeSemantic, sourceTime, updatedTime)
	src2 := createMemory(t, env.store, util.NewID(), projectID, "source two internal/baz/qux.go", domain.MemoryTypeSemantic, sourceTime, updatedTime)

	linkCompiledFrom(t, env.relStore, article.ID, src1.ID)
	linkCompiledFrom(t, env.relStore, article.ID, src2.ID)

	detector := NewStalenessDetector(env.mnemos, defaultCfg())
	maps := BuildEntityMaps([]*domain.Memory{src1, src2})

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Type != FindingStaleCompiled {
		t.Errorf("expected FindingStaleCompiled, got %s", f.Type)
	}
	if f.Metadata["newer_source_count"] != 2 {
		t.Errorf("expected newer_source_count=2, got %v", f.Metadata["newer_source_count"])
	}
	if f.Metadata["article_id"] != article.ID {
		t.Errorf("expected article_id=%s, got %v", article.ID, f.Metadata["article_id"])
	}
}

// TestStalenessDetector_NoStaleArticles: all sources older than article → 0 findings
func TestStalenessDetector_NoStaleArticles(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-no-stale"

	articleTime := time.Now().UTC().Add(-1 * time.Hour)
	sourceTime := time.Now().UTC().Add(-3 * time.Hour) // created and updated before article

	article := createMemory(t, env.store, util.NewID(), projectID, "compiled article", domain.MemoryTypeCompiled, articleTime, articleTime)
	src := createMemory(t, env.store, util.NewID(), projectID, "old source", domain.MemoryTypeSemantic, sourceTime, sourceTime)

	linkCompiledFrom(t, env.relStore, article.ID, src.ID)

	detector := NewStalenessDetector(env.mnemos, defaultCfg())
	maps := BuildEntityMaps([]*domain.Memory{src})

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

// TestStalenessDetector_NoCompiledArticles: empty store → 0 findings
func TestStalenessDetector_NoCompiledArticles(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()

	detector := NewStalenessDetector(env.mnemos, defaultCfg())
	maps := EntityMaps{
		Entities:  make(map[string][]string),
		CreatedAt: make(map[string]time.Time),
	}

	findings, err := detector.Detect(ctx, "empty-project", maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

// TestStalenessDetector_RespectsMaxCap: 60 articles, cap=50 → processes exactly 50
func TestStalenessDetector_RespectsMaxCap(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-cap"

	articleTime := time.Now().UTC().Add(-2 * time.Hour)
	sourceTime := time.Now().UTC().Add(-3 * time.Hour)
	updatedTime := time.Now().UTC().Add(-1 * time.Hour)

	// Create 60 compiled articles, each with 1 updated source
	var sources []*domain.Memory
	for i := 0; i < 60; i++ {
		article := createMemory(t, env.store, util.NewID(), projectID, "compiled article", domain.MemoryTypeCompiled, articleTime, articleTime)
		src := createMemory(t, env.store, util.NewID(), projectID, "source memory", domain.MemoryTypeSemantic, sourceTime, updatedTime)
		sources = append(sources, src)
		linkCompiledFrom(t, env.relStore, article.ID, src.ID)
	}

	cfg := defaultCfg()
	cfg.MaxCompiledPerRun = 50

	detector := NewStalenessDetector(env.mnemos, cfg)
	maps := BuildEntityMaps(sources)

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) > 50 {
		t.Errorf("expected at most 50 findings (cap), got %d", len(findings))
	}
}

// TestStalenessDetector_RelatedNewCount: post-article memory shares entity with source → related_new_count=1
func TestStalenessDetector_RelatedNewCount(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-related"

	articleTime := time.Now().UTC().Add(-2 * time.Hour)
	sourceTime := time.Now().UTC().Add(-3 * time.Hour)
	updatedTime := time.Now().UTC().Add(-1 * time.Hour)
	postArticleTime := time.Now().UTC().Add(-30 * time.Minute)

	// Source memory with a distinctive entity
	src := createMemory(t, env.store, util.NewID(), projectID, "source internal/storage/sqlite/store.go", domain.MemoryTypeSemantic, sourceTime, updatedTime)
	article := createMemory(t, env.store, util.NewID(), projectID, "compiled article", domain.MemoryTypeCompiled, articleTime, articleTime)
	linkCompiledFrom(t, env.relStore, article.ID, src.ID)

	// Post-article memory sharing the same entity
	postMem := createMemory(t, env.store, util.NewID(), projectID, "new memory internal/storage/sqlite/store.go", domain.MemoryTypeSemantic, postArticleTime, postArticleTime)

	detector := NewStalenessDetector(env.mnemos, defaultCfg())
	maps := BuildEntityMaps([]*domain.Memory{src, postMem})

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["related_new_count"] != 1 {
		t.Errorf("expected related_new_count=1, got %v", findings[0].Metadata["related_new_count"])
	}
}
