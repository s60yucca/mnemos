package main

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/spf13/cobra"
)

// longContent is content with more than 10 words so ExtractSummary returns non-empty.
const longContent = "The authentication system uses JWT tokens to validate user sessions. " +
	"Each token contains a signed payload with user ID and expiry. " +
	"The SessionStore caches active tokens to reduce database lookups."

// createMemoryWithEmptySummary inserts a memory with empty summary directly via the store.
func createMemoryWithEmptySummary(t *testing.T, store interface {
	Create(ctx context.Context, m *domain.Memory) error
}, projectID, content string) *domain.Memory {
	t.Helper()
	now := time.Now().UTC()
	mem := &domain.Memory{
		ID:             util.NewID(),
		Content:        content,
		Summary:        "", // explicitly empty
		Type:           domain.MemoryTypeSemantic,
		Category:       "test",
		ProjectID:      projectID,
		Status:         domain.MemoryStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
		RelevanceScore: 1.0,
		ContentHash:    util.NewID(),
	}
	if err := store.Create(context.Background(), mem); err != nil {
		t.Fatalf("create memory: %v", err)
	}
	return mem
}

// TestBackfillSummaries_DryRun verifies that --dry-run prints what would be updated
// without actually writing summaries to the database.
func TestBackfillSummaries_DryRun(t *testing.T) {
	mnemos, store := buildTestMnemos(t)
	projectID := "proj-backfill-dry"

	// Create two memories with empty summaries
	m1 := createMemoryWithEmptySummary(t, store, projectID, longContent)
	m2 := createMemoryWithEmptySummary(t, store, projectID, longContent+" Additional context about the system architecture and design patterns used.")

	rootCmd := &cobra.Command{Use: "mnemos"}
	rootCmd.AddCommand(newBackfillCmd(mnemos))

	out := executeCmd(t, rootCmd, []string{"backfill", "summaries", "--project", projectID, "--dry-run"})

	// Output should contain [DRY RUN] prefix
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected '[DRY RUN]' in output, got:\n%s", out)
	}

	// Summaries should still be empty in the store (no writes)
	ctx := context.Background()
	got1, err := mnemos.Get(ctx, m1.ID)
	if err != nil {
		t.Fatalf("get m1: %v", err)
	}
	if got1.Summary != "" {
		t.Errorf("expected empty summary after dry-run, got: %q", got1.Summary)
	}

	got2, err := mnemos.Get(ctx, m2.ID)
	if err != nil {
		t.Fatalf("get m2: %v", err)
	}
	if got2.Summary != "" {
		t.Errorf("expected empty summary after dry-run, got: %q", got2.Summary)
	}
}

// TestBackfillSummaries_LiveRun verifies that without --dry-run, summaries are populated.
func TestBackfillSummaries_LiveRun(t *testing.T) {
	mnemos, store := buildTestMnemos(t)
	projectID := "proj-backfill-live"

	// Create two memories with empty summaries
	m1 := createMemoryWithEmptySummary(t, store, projectID, longContent)
	m2 := createMemoryWithEmptySummary(t, store, projectID, longContent+" Additional context about the system architecture and design patterns used.")

	rootCmd := &cobra.Command{Use: "mnemos"}
	rootCmd.AddCommand(newBackfillCmd(mnemos))

	out := executeCmd(t, rootCmd, []string{"backfill", "summaries", "--project", projectID})

	// Output should report updated count
	if !strings.Contains(out, "updated") {
		t.Errorf("expected 'updated' in output, got:\n%s", out)
	}

	// Summaries should now be populated
	ctx := context.Background()
	got1, err := mnemos.Get(ctx, m1.ID)
	if err != nil {
		t.Fatalf("get m1: %v", err)
	}
	if got1.Summary == "" {
		t.Errorf("expected non-empty summary after live run for m1")
	}

	got2, err := mnemos.Get(ctx, m2.ID)
	if err != nil {
		t.Fatalf("get m2: %v", err)
	}
	if got2.Summary == "" {
		t.Errorf("expected non-empty summary after live run for m2")
	}
}

// TestBackfillSummaries_MissingProject verifies that omitting --project causes an error.
func TestBackfillSummaries_MissingProject(t *testing.T) {
	mnemos, _ := buildTestMnemos(t)

	rootCmd := &cobra.Command{Use: "mnemos"}
	rootCmd.AddCommand(newBackfillCmd(mnemos))
	rootCmd.SetArgs([]string{"backfill", "summaries"})

	err := rootCmd.Execute()
	if err == nil {
		t.Error("expected error when --project flag is missing, got nil")
	}
}
