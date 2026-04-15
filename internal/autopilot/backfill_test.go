package autopilot

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/util"
)

func backfillLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// createMemoryWithSummary inserts a memory with an explicit summary into the store.
func createMemoryWithSummary(t *testing.T, env *stalenessTestEnv, id, projectID, content, summary string, memType domain.MemoryType) *domain.Memory {
	t.Helper()
	now := time.Now().UTC()
	m := &domain.Memory{
		ID:             id,
		Content:        content,
		Summary:        summary,
		Type:           memType,
		Category:       "code",
		ProjectID:      projectID,
		Status:         domain.MemoryStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
		RelevanceScore: 1.0,
		ContentHash:    util.NewID(),
	}
	if err := env.store.Create(context.Background(), m); err != nil {
		t.Fatalf("create memory %s: %v", id, err)
	}
	return m
}

// TestSummaryBackfillDetector_NoMemoriesNeedingBackfill: all memories have summaries → 0 findings
func TestSummaryBackfillDetector_NoMemoriesNeedingBackfill(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-backfill-none"

	// Create memories that already have summaries
	createMemoryWithSummary(t, env, util.NewID(), projectID,
		"This is a long enough content string with more than ten words to qualify for summarization.",
		"existing summary",
		domain.MemoryTypeSemantic,
	)
	createMemoryWithSummary(t, env, util.NewID(), projectID,
		"Another memory with sufficient content that has more than ten words in it.",
		"another existing summary",
		domain.MemoryTypeSemantic,
	)

	detector := NewSummaryBackfillDetector(env.mnemos, defaultCfg(), backfillLogger())
	maps := EntityMaps{Entities: make(map[string][]string), CreatedAt: make(map[string]time.Time)}

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

// TestSummaryBackfillDetector_BackfillsEmptySummaries: N memories with empty summary and >10 word content → N updated, 1 finding with count=N
func TestSummaryBackfillDetector_BackfillsEmptySummaries(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-backfill-empty"

	// Create 3 memories with empty summaries and sufficient content (>10 words)
	longContent := "The authentication service uses JWT tokens to validate user sessions and manage access control policies."
	createMemoryWithSummary(t, env, util.NewID(), projectID, longContent, "", domain.MemoryTypeSemantic)
	createMemoryWithSummary(t, env, util.NewID(), projectID, longContent, "", domain.MemoryTypeSemantic)
	createMemoryWithSummary(t, env, util.NewID(), projectID, longContent, "", domain.MemoryTypeSemantic)

	detector := NewSummaryBackfillDetector(env.mnemos, defaultCfg(), backfillLogger())
	maps := EntityMaps{Entities: make(map[string][]string), CreatedAt: make(map[string]time.Time)}

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	f := findings[0]
	if f.Type != FindingBackfillCompleted {
		t.Errorf("expected FindingBackfillCompleted, got %s", f.Type)
	}
	if f.Metadata["count"] != 3 {
		t.Errorf("expected count=3, got %v", f.Metadata["count"])
	}
}

// TestSummaryBackfillDetector_SkipsMemoriesWithExistingSummary: memories with existing summary → not touched
func TestSummaryBackfillDetector_SkipsMemoriesWithExistingSummary(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-backfill-skip"

	longContent := "The authentication service uses JWT tokens to validate user sessions and manage access control policies."
	existingSummary := "my existing summary that should not be overwritten"

	mem := createMemoryWithSummary(t, env, util.NewID(), projectID, longContent, existingSummary, domain.MemoryTypeSemantic)

	detector := NewSummaryBackfillDetector(env.mnemos, defaultCfg(), backfillLogger())
	maps := EntityMaps{Entities: make(map[string][]string), CreatedAt: make(map[string]time.Time)}

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (memory already has summary), got %d", len(findings))
	}

	// Verify the summary was not changed
	stored, err := env.store.GetByID(ctx, mem.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Summary != existingSummary {
		t.Errorf("expected summary %q unchanged, got %q", existingSummary, stored.Summary)
	}
}

// TestSummaryBackfillDetector_SkipsShortContent: memory with ≤10 word content → ExtractSummary returns "" → skipped (count=0)
func TestSummaryBackfillDetector_SkipsShortContent(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-backfill-short"

	// Content with ≤10 words — ExtractSummary returns "" for these
	shortContent := "Too short to summarize."
	createMemoryWithSummary(t, env, util.NewID(), projectID, shortContent, "", domain.MemoryTypeSemantic)

	detector := NewSummaryBackfillDetector(env.mnemos, defaultCfg(), backfillLogger())
	maps := EntityMaps{Entities: make(map[string][]string), CreatedAt: make(map[string]time.Time)}

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// ExtractSummary returns "" for short content → count stays 0 → no findings
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for short content, got %d: %+v", len(findings), findings)
	}
}
