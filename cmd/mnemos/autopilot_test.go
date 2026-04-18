package main

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/autopilot"
	"github.com/mnemos-dev/mnemos/internal/config"
	core "github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/spf13/cobra"
)

// buildTestMnemos creates a real in-memory Mnemos instance for tests.
func buildTestMnemos(t *testing.T) (*core.Mnemos, *sqlitestore.SQLiteStore) {
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

	searchEng := search.NewSearchEngine(fts, embedStore, embedder, relStore, logger, 0.7, 0.0)
	relMgr := relation.NewManager(relStore, store, logger)
	lcEngine := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, logger)

	mnemos := core.NewMnemos(memMgr, searchEng, relMgr, lcEngine, store, logger)
	return mnemos, store
}

// buildDisabledDaemon creates an AutopilotDaemon with enabled=false.
func buildDisabledDaemon(mnemos *core.Mnemos) *autopilot.AutopilotDaemon {
	cfg := config.AutopilotConfig{
		Enabled:           false,
		Interval:          15 * time.Minute,
		InitialDelay:      30 * time.Second,
		MaxCompiledPerRun: 50,
		MaxMemoriesPerRun: 200,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return autopilot.NewAutopilotDaemon(mnemos, cfg, logger, nil)
}

// buildEnabledDaemon creates an AutopilotDaemon with enabled=true.
func buildEnabledDaemon(mnemos *core.Mnemos) *autopilot.AutopilotDaemon {
	cfg := config.AutopilotConfig{
		Enabled:           true,
		Interval:          15 * time.Minute,
		InitialDelay:      10 * time.Second,
		MaxCompiledPerRun: 50,
		MaxMemoriesPerRun: 200,
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return autopilot.NewAutopilotDaemon(mnemos, cfg, logger, nil)
}

// captureStdout runs f and returns whatever was written to os.Stdout.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// executeCmd runs a cobra command with the given args and captures stdout.
func executeCmd(t *testing.T, cmd *cobra.Command, args []string) string {
	t.Helper()
	cmd.SetArgs(args)
	out := captureStdout(t, func() {
		cmd.Execute() //nolint:errcheck
	})
	return out
}

// TestAutopilotStatus_Disabled verifies that "autopilot status" prints "false" for enabled
// when the daemon is configured with enabled=false.
func TestAutopilotStatus_Disabled(t *testing.T) {
	mnemos, _ := buildTestMnemos(t)
	daemon := buildDisabledDaemon(mnemos)

	rootCmd := &cobra.Command{Use: "mnemos"}
	rootCmd.AddCommand(newAutopilotCmd(daemon, mnemos))

	out := executeCmd(t, rootCmd, []string{"autopilot", "status"})

	if !strings.Contains(out, "enabled:       false") {
		t.Errorf("expected 'enabled:       false' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "next_run:      (not scheduled)") {
		t.Errorf("expected '(not scheduled)' in output, got:\n%s", out)
	}
}

// TestAutopilotReport_NoReport verifies that "autopilot report" prints the no-report message
// when no autopilot report exists in the store.
func TestAutopilotReport_NoReport(t *testing.T) {
	mnemos, _ := buildTestMnemos(t)
	daemon := buildEnabledDaemon(mnemos)

	rootCmd := &cobra.Command{Use: "mnemos"}
	rootCmd.AddCommand(newAutopilotCmd(daemon, mnemos))

	out := executeCmd(t, rootCmd, []string{"autopilot", "report", "--project", "test-project"})

	if !strings.Contains(out, "No autopilot report found.") {
		t.Errorf("expected 'No autopilot report found.' in output, got:\n%s", out)
	}
}

// TestAutopilotRun_DryRun verifies that "autopilot run --dry-run" prefixes output with
// "[DRY RUN]" and does not write any report to the store.
func TestAutopilotRun_DryRun(t *testing.T) {
	mnemos, store := buildTestMnemos(t)
	daemon := buildEnabledDaemon(mnemos)

	ctx := context.Background()
	projectID := "proj-cli-dry-run"
	now := time.Now().UTC()

	// Store two memories sharing a file path so the relation detector fires.
	m1 := &domain.Memory{
		ID:             util.NewID(),
		Content:        "see internal/storage/sqlite/store.go for details",
		Type:           domain.MemoryTypeSemantic,
		Category:       "code",
		ProjectID:      projectID,
		Status:         domain.MemoryStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
		RelevanceScore: 1.0,
		ContentHash:    util.NewID(),
	}
	m2 := &domain.Memory{
		ID:             util.NewID(),
		Content:        "bug in internal/storage/sqlite/store.go line 42",
		Type:           domain.MemoryTypeSemantic,
		Category:       "code",
		ProjectID:      projectID,
		Status:         domain.MemoryStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
		RelevanceScore: 1.0,
		ContentHash:    util.NewID(),
	}
	if err := store.Create(ctx, m1); err != nil {
		t.Fatalf("create m1: %v", err)
	}
	if err := store.Create(ctx, m2); err != nil {
		t.Fatalf("create m2: %v", err)
	}

	rootCmd := &cobra.Command{Use: "mnemos"}
	rootCmd.AddCommand(newAutopilotCmd(daemon, mnemos))

	out := executeCmd(t, rootCmd, []string{"autopilot", "run", "--dry-run", "--project", projectID})

	// Output must be prefixed with [DRY RUN]
	if !strings.Contains(out, "[DRY RUN]") {
		t.Errorf("expected '[DRY RUN]' prefix in output, got:\n%s", out)
	}

	// No report should have been written to the store
	report, err := mnemos.GetLatestAutopilotReport(ctx, projectID)
	if err != nil {
		t.Fatalf("GetLatestAutopilotReport: %v", err)
	}
	if report != nil {
		t.Errorf("expected no report written in dry-run, but found one: %s", report.ID)
	}
}
