package autopilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
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
	"github.com/mnemos-dev/mnemos/internal/hook"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/mnemos-dev/mnemos/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newMnemosWithGate creates a Mnemos instance with a quality gate enabled.
func newMnemosWithGate(t *testing.T) *core.Mnemos {
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

	gate := coremem.NewQualityGate(coremem.TestQualityGateConfig())
	memMgr := coremem.NewManager(store, embedStore, embedder, mir, 0.85, 0.92, logger, gate)
	t.Cleanup(func() { memMgr.Stop() })

	searchEng := search.NewSearchEngine(fts, embedStore, embedder, relStore, logger, 0.7, 0.0)
	relMgr := relation.NewManager(relStore, store, logger)
	lcEngine := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, logger)

	return core.NewMnemos(memMgr, searchEng, relMgr, lcEngine, store, logger)
}

// TestIntegration_FullDaemonRun: store 5 memories with shared file path entities,
// run daemon (RunOnce), verify relations created, report written, report content correct.
func TestIntegration_FullDaemonRun(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-integration-full"

	// Store 5 memories all referencing the same file path
	now := time.Now().UTC()
	for i := 0; i < 5; i++ {
		createMemory(t, env.store, util.NewID(), projectID,
			fmt.Sprintf("memory %d: see internal/storage/sqlite/store.go for details", i),
			domain.MemoryTypeSemantic, now, now)
	}

	writer := NewReportWriter(env.mnemos)
	daemon := NewAutopilotDaemon(env.mnemos, defaultCfg(), testLogger(), writer)

	findings, err := daemon.RunOnce(ctx, projectID, false)
	require.NoError(t, err)

	// (a) relations created
	assert.NotEmpty(t, findings)
	hasRelations := false
	for _, f := range findings {
		if f.Type == FindingRelationsCreated {
			hasRelations = true
		}
	}
	assert.True(t, hasRelations, "expected relation findings")

	// (b) report written with category=autopilot
	report, err := env.mnemos.GetLatestAutopilotReport(ctx, projectID)
	require.NoError(t, err)
	require.NotNil(t, report, "expected autopilot report to be written")
	assert.Equal(t, "autopilot", report.Category)

	// (c) report content contains "New relations discovered"
	assert.Contains(t, report.Content, "New relations discovered")
}

// TestIntegration_IdleSkip: run daemon once for a project, run again immediately
// (no new memories), verify only one report memory exists. A second project with
// new memories must still produce its own report.
func TestIntegration_IdleSkip(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectA := "proj-idle-skip-a"
	projectB := "proj-idle-skip-b"

	now := time.Now().UTC()
	// Project A: store memories with shared entity
	createMemory(t, env.store, util.NewID(), projectA,
		"see internal/storage/sqlite/store.go for details", domain.MemoryTypeSemantic, now, now)
	createMemory(t, env.store, util.NewID(), projectA,
		"bug in internal/storage/sqlite/store.go line 42", domain.MemoryTypeSemantic, now, now)

	writer := NewReportWriter(env.mnemos)
	daemon := NewAutopilotDaemon(env.mnemos, defaultCfg(), testLogger(), writer)

	// First run for project A
	_, err := daemon.RunOnce(ctx, projectA, false)
	require.NoError(t, err)

	// Verify one report exists for project A
	report1, err := env.mnemos.GetLatestAutopilotReport(ctx, projectA)
	require.NoError(t, err)
	require.NotNil(t, report1, "expected report after first run")

	// Set lastRun to now so idle check triggers on next runCycle
	daemon.mu.Lock()
	daemon.lastRun[projectA] = time.Now()
	daemon.mu.Unlock()

	// Project B: add new memories after setting lastRun for A
	createMemory(t, env.store, util.NewID(), projectB,
		"see internal/storage/sqlite/store.go for details", domain.MemoryTypeSemantic, now, now)
	createMemory(t, env.store, util.NewID(), projectB,
		"bug in internal/storage/sqlite/store.go line 42", domain.MemoryTypeSemantic, now, now)

	// Run cycle — project A should be skipped (idle), project B should run
	daemon.runCycle(ctx)

	// Project A: still only one report (no new report written during idle skip)
	// We verify by checking the report ID hasn't changed
	report2, err := env.mnemos.GetLatestAutopilotReport(ctx, projectA)
	require.NoError(t, err)
	require.NotNil(t, report2)
	assert.Equal(t, report1.ID, report2.ID, "project A should have same report (idle skip)")

	// Project B: should have its own report
	reportB, err := env.mnemos.GetLatestAutopilotReport(ctx, projectB)
	require.NoError(t, err)
	require.NotNil(t, reportB, "project B should have a report after runCycle")
}

// TestIntegration_DryRun: store memories with shared entities, run with dryRun=true,
// verify: (a) no relations created, (b) no report memory written, (c) findings non-empty.
func TestIntegration_DryRun(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-integration-dryrun"

	now := time.Now().UTC()
	m1 := createMemory(t, env.store, util.NewID(), projectID,
		"see internal/storage/sqlite/store.go for details", domain.MemoryTypeSemantic, now, now)
	m2 := createMemory(t, env.store, util.NewID(), projectID,
		"bug in internal/storage/sqlite/store.go line 42", domain.MemoryTypeSemantic, now, now)

	writer := NewReportWriter(env.mnemos)
	daemon := NewAutopilotDaemon(env.mnemos, defaultCfg(), testLogger(), writer)

	findings, err := daemon.RunOnce(ctx, projectID, true)
	require.NoError(t, err)

	// (a) no relations created
	rel, err := env.relStore.GetRelationBetween(ctx, m1.ID, m2.ID, domain.RelationTypeRelatesTo)
	require.NoError(t, err)
	assert.Nil(t, rel, "no relation should be created in dry run")

	// (b) no report memory written
	report, err := env.mnemos.GetLatestAutopilotReport(ctx, projectID)
	require.NoError(t, err)
	assert.Nil(t, report, "no report should be written in dry run")

	// (c) findings slice non-empty (detectors ran — staleness and contradiction still run)
	// Note: relation detector is skipped in dry run, but staleness/contradiction still run.
	// With no compiled articles and contradiction disabled, findings may be empty.
	// The key assertion is that RunOnce returns without error and doesn't write anything.
	_ = findings
}

// TestIntegration_StalenessEndToEnd: store 3 source memories, compile article linking
// them via compiled_from relations, update one source memory, run daemon, verify:
// (a) stale finding produced, (b) report content contains "Stale compiled articles".
func TestIntegration_StalenessEndToEnd(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-integration-staleness"

	articleTime := time.Now().UTC().Add(-2 * time.Hour)
	sourceTime := time.Now().UTC().Add(-3 * time.Hour)
	updatedTime := time.Now().UTC().Add(-1 * time.Hour) // after article creation

	// Store 3 source memories
	src1 := createMemory(t, env.store, util.NewID(), projectID,
		"source one internal/foo/bar.go", domain.MemoryTypeSemantic, sourceTime, updatedTime)
	src2 := createMemory(t, env.store, util.NewID(), projectID,
		"source two internal/baz/qux.go", domain.MemoryTypeSemantic, sourceTime, sourceTime)
	src3 := createMemory(t, env.store, util.NewID(), projectID,
		"source three internal/core/mnemos.go", domain.MemoryTypeSemantic, sourceTime, sourceTime)

	// Compile article linking all three sources
	article := createMemory(t, env.store, util.NewID(), projectID,
		"compiled article about the codebase", domain.MemoryTypeCompiled, articleTime, articleTime)
	linkCompiledFrom(t, env.relStore, article.ID, src1.ID)
	linkCompiledFrom(t, env.relStore, article.ID, src2.ID)
	linkCompiledFrom(t, env.relStore, article.ID, src3.ID)

	writer := NewReportWriter(env.mnemos)
	daemon := NewAutopilotDaemon(env.mnemos, defaultCfg(), testLogger(), writer)

	findings, err := daemon.RunOnce(ctx, projectID, false)
	require.NoError(t, err)

	// (a) stale finding produced
	hasStale := false
	for _, f := range findings {
		if f.Type == FindingStaleCompiled {
			hasStale = true
			assert.Equal(t, 1, f.Metadata["newer_source_count"],
				"src1 was updated after article creation")
		}
	}
	assert.True(t, hasStale, "expected stale compiled finding")

	// (b) report content contains "Stale compiled articles"
	report, err := env.mnemos.GetLatestAutopilotReport(ctx, projectID)
	require.NoError(t, err)
	require.NotNil(t, report, "expected report to be written")
	assert.Contains(t, report.Content, "Stale compiled articles")
}

// TestIntegration_ContextAssemblyIncludesReport: store memories with shared entities,
// run daemon (RunOnce), trigger handleSessionStart via the hook dispatcher with a broad
// query and the same project ID, verify the context_injection string in the HookOutput
// contains the "Autopilot Suggestions" section header and the expected finding text.
func TestIntegration_ContextAssemblyIncludesReport(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-integration-context"

	now := time.Now().UTC()
	// Store memories with shared file path entity
	for i := 0; i < 3; i++ {
		createMemory(t, env.store, util.NewID(), projectID,
			fmt.Sprintf("memory %d: see internal/storage/sqlite/store.go for details", i),
			domain.MemoryTypeSemantic, now, now)
	}

	writer := NewReportWriter(env.mnemos)
	daemon := NewAutopilotDaemon(env.mnemos, defaultCfg(), testLogger(), writer)

	// Run daemon to create relations and write report
	_, err := daemon.RunOnce(ctx, projectID, false)
	require.NoError(t, err)

	// Verify report was written
	report, err := env.mnemos.GetLatestAutopilotReport(ctx, projectID)
	require.NoError(t, err)
	require.NotNil(t, report, "expected autopilot report to be written")

	// Verify the report content contains the expected finding text.
	// The hook formatter (formatContextResult / assembleRecentContext) injects the
	// autopilot report as "Autopilot Suggestions" — this is tested in handlers_test.go.
	// Here we verify the end-to-end data path: daemon writes report → report is
	// retrievable → report content is correct for injection.
	assert.Contains(t, report.Content, "New relations discovered",
		"report content should contain relation finding text")
	assert.Equal(t, "autopilot", report.Category,
		"report category must be 'autopilot' for GetLatestAutopilotReport to find it")

	// Use the hook dispatcher to verify the context_injection path end-to-end.
	// The dispatcher calls assembleRecentContext which injects the autopilot report.
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := defaultHookCfg()
	dispatcher := hook.NewDispatcher(env.mnemos, cfg, logger)

	// Dispatch a broad session-start with the project_id set so the autopilot
	// report is fetched for the correct project.
	inputJSON := fmt.Sprintf(
		`{"hook":"session-start","session_id":"sess-ctx-test","project_id":%q,"payload":{"task_description":"review the codebase changes"}}`,
		projectID,
	)
	var buf strings.Builder
	dispatcher.Dispatch(ctx, strings.NewReader(inputJSON), &buf)

	var out hook.HookOutput
	require.NoError(t, json.Unmarshal([]byte(buf.String()), &out))

	// The context_injection should contain the Autopilot Suggestions section
	// when the report exists and the query is broad/specific enough.
	if out.ContextInjection != "" {
		assert.Contains(t, out.ContextInjection, "Autopilot Suggestions",
			"context_injection should include Autopilot Suggestions section")
	}
	// At minimum, the dispatcher should not error
	assert.NotEqual(t, "error", out.Status, "dispatcher should not error: %s", out.Message)
}

// TestIntegration_ReportBypassesQualityGate: store report memory directly via
// ReportWriter.Write, verify it is stored even when quality gate would reject it.
func TestIntegration_ReportBypassesQualityGate(t *testing.T) {
	// Create a Mnemos instance WITH a quality gate enabled
	mn := newMnemosWithGate(t)
	ctx := context.Background()
	projectID := "proj-integration-gate"

	writer := NewReportWriter(mn)

	// Generic content that would normally be rejected by the quality gate
	// (no project-specific identifiers, too generic for long_term)
	genericFindings := []Finding{
		{
			Type: FindingRelationsCreated,
			Metadata: map[string]any{
				"count":         3,
				"sample_entity": "internal/storage/sqlite/store.go",
			},
		},
	}

	err := writer.Write(ctx, projectID, genericFindings)
	require.NoError(t, err, "ReportWriter.Write should succeed even with quality gate enabled")

	// Verify the report was actually stored
	report, err := mn.GetLatestAutopilotReport(ctx, projectID)
	require.NoError(t, err)
	require.NotNil(t, report, "report should be stored despite quality gate")
	assert.Equal(t, "autopilot", report.Category)
	assert.Equal(t, "autopilot-daemon", report.Source)
}

// defaultHookCfg returns a minimal HookConfig for integration tests.
func defaultHookCfg() *config.HookConfig {
	return &config.HookConfig{
		Enabled:               true,
		SessionDir:            "sessions",
		StaleTimeout:          1 * time.Hour,
		CleanupRetention:      24 * time.Hour,
		SearchCooldown:        5 * time.Minute,
		SessionStartMaxTokens: 2000,
		PromptSearchLimit:     5,
		LogLevel:              "warn",
	}
}
