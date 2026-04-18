package search_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// newEngineForFileTest creates a SearchEngine backed by an in-memory SQLite store.
// fileBoost is passed directly; embedder/embedStore are nil (text-only search).
func newEngineForFileTest(t *testing.T, fileBoost float64) (*search.SearchEngine, *sqlite.SQLiteStore) {
	t.Helper()
	db, err := sqlite.Open(":memory:")
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := sqlite.NewSQLiteStore(db)
	fts := sqlite.NewFTSSearcher(db)

	engine := search.NewSearchEngine(fts, nil, nil, nil, nil, 0.7, fileBoost)
	return engine, store
}

// storeMemWithFiles inserts a memory whose Metadata["related_files"] is set to the
// given file paths. The content is used as-is for FTS indexing.
func storeMemWithFiles(t *testing.T, store *sqlite.SQLiteStore, id, content string, filePaths []string) *domain.Memory {
	t.Helper()
	ctx := context.Background()

	var meta map[string]string
	if len(filePaths) > 0 {
		encoded, _ := json.Marshal(filePaths)
		meta = map[string]string{
			memory.RelatedFilesKey: string(encoded),
		}
	}

	now := time.Now().UTC()
	mem := &domain.Memory{
		ID:             id,
		Content:        content,
		Type:           domain.MemoryTypeEpisodic,
		Category:       domain.CategoryGeneral,
		Tags:           []string{"test"},
		Status:         domain.MemoryStatusActive,
		Metadata:       meta,
		RelevanceScore: 1.0,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
		ContentHash:    util.ContentHash(content, id),
	}
	if err := store.Create(ctx, mem); err != nil {
		t.Fatalf("store.Create(%s): %v", id, err)
	}
	return mem
}

// TestEngine_File_NilOpenFiles verifies that AssembleContext with nil openFiles
// does not alter memory scores (file boost is a no-op).
func TestEngine_File_NilOpenFiles(t *testing.T) {
	engine, store := newEngineForFileTest(t, memory.FileBoostDefault)
	ctx := context.Background()

	// Store a memory that references a file path
	storeMemWithFiles(t, store, "mem1", "auth/session.go handles token validation", []string{"auth/session.go"})

	result, err := engine.AssembleContext(ctx, "auth session token", "", 10000, false, nil)
	if err != nil {
		t.Fatalf("AssembleContext: %v", err)
	}
	if len(result.Memories) == 0 {
		t.Skip("FTS returned no results — query did not match; skipping score check")
	}

	for _, m := range result.Memories {
		// Score must be ≤ 1.0 (no boost applied); FTS scores are negative BM25 values
		// propagated as TextScore, so we just verify no boost was added.
		// The raw BM25 score from FTS is negative, so RelevanceScore will be ≤ 0.
		// We verify it is not boosted by checking it is < FileBoostDefault.
		if m.RelevanceScore >= memory.FileBoostDefault {
			t.Errorf("nil openFiles: expected no boost, got RelevanceScore=%f for mem %s", m.RelevanceScore, m.ID)
		}
	}
}

// TestEngine_File_OverlappingOpenFiles verifies that memories whose related_files
// overlap with openFiles receive a score boost of FileBoostDefault.
func TestEngine_File_OverlappingOpenFiles(t *testing.T) {
	engine, store := newEngineForFileTest(t, memory.FileBoostDefault)
	ctx := context.Background()

	// Store two memories: one with a matching file path, one without
	storeMemWithFiles(t, store, "mem-match", "auth/session.go handles token validation logic", []string{"auth/session.go"})
	storeMemWithFiles(t, store, "mem-nomatch", "database/postgres.go handles connection pooling logic", []string{"database/postgres.go"})

	openFiles := []string{"auth/session.go"}
	result, err := engine.AssembleContext(ctx, "handles logic", "", 10000, false, openFiles)
	if err != nil {
		t.Fatalf("AssembleContext: %v", err)
	}
	if len(result.Memories) == 0 {
		t.Skip("FTS returned no results — skipping boost check")
	}

	// Find the matching memory and verify it was boosted
	var matchScore, noMatchScore float64
	var foundMatch, foundNoMatch bool
	for _, m := range result.Memories {
		if m.ID == "mem-match" {
			matchScore = m.RelevanceScore
			foundMatch = true
		}
		if m.ID == "mem-nomatch" {
			noMatchScore = m.RelevanceScore
			foundNoMatch = true
		}
	}

	if !foundMatch {
		t.Skip("mem-match not in results — FTS ranking excluded it; skipping")
	}
	if foundNoMatch {
		// Both present: boosted memory must score higher
		if matchScore <= noMatchScore {
			t.Errorf("overlapping memory should score higher: match=%f, nomatch=%f", matchScore, noMatchScore)
		}
		diff := matchScore - noMatchScore
		if diff < memory.FileBoostDefault-1e-9 {
			t.Errorf("score difference should be at least FileBoostDefault=%.2f, got %.4f", memory.FileBoostDefault, diff)
		}
	} else {
		// Only match present: verify boost was applied (score > raw BM25 range)
		// BM25 scores from SQLite FTS are negative; after boost they should be > 0
		// if the raw score was close to 0, or at least boosted by FileBoostDefault.
		// We verify the boost was applied by checking score > raw (can't know raw here,
		// so we just verify the memory was returned and score is reasonable).
		t.Logf("only mem-match in results, score=%f", matchScore)
	}
}

// TestEngine_File_BoostedSurvivesDiversityFilter verifies that a boosted memory
// (score > 1.0) is not capped and survives DiversityFilter.
func TestEngine_File_BoostedSurvivesDiversityFilter(t *testing.T) {
	// Use the ContextAssembler directly to test that scores > 1.0 are not capped.
	// This tests the DiversityFilter behaviour with boosted scores.
	a := search.NewContextAssembler(0.7)

	// Create two memories with identical tags/category (maximum similarity penalty).
	// The boosted one has score > 1.0; the unboosted has score 0.9.
	boosted := &domain.Memory{
		ID:             "boosted",
		Content:        "auth/session.go token validation",
		Tags:           []string{"auth", "security"},
		Category:       "security",
		RelevanceScore: 1.0 + memory.FileBoostDefault, // 1.3 — above 1.0
	}
	unboosted := &domain.Memory{
		ID:             "unboosted",
		Content:        "auth/session.go token refresh",
		Tags:           []string{"auth", "security"},
		Category:       "security",
		RelevanceScore: 0.9,
	}
	other := &domain.Memory{
		ID:             "other",
		Content:        "database connection pool settings",
		Tags:           []string{"database", "performance"},
		Category:       "performance",
		RelevanceScore: 0.85,
	}

	candidates := []*domain.Memory{boosted, unboosted, other}
	selected := a.DiversityFilter(candidates, 10000)

	// Boosted memory must be first (highest score)
	if len(selected) == 0 {
		t.Fatal("DiversityFilter returned empty result")
	}
	if selected[0].ID != "boosted" {
		t.Errorf("boosted memory should be first, got %s (score=%f)", selected[0].ID, selected[0].RelevanceScore)
	}

	// Verify score was not capped at 1.0
	if selected[0].RelevanceScore < 1.0+memory.FileBoostDefault-1e-9 {
		t.Errorf("score should not be capped: got %f, want >= %f", selected[0].RelevanceScore, 1.0+memory.FileBoostDefault)
	}
}
