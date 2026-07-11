package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/benchmark"
	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/storage/markdown"
	sqlitestore "github.com/mnemos-dev/mnemos/internal/storage/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratedReportIdentityRequiresAllMarkers(t *testing.T) {
	base := &domain.Memory{
		Category: "autopilot",
		Source:   "autopilot-daemon",
		Tags:     []string{"autopilot-report"},
	}
	assert.True(t, isGeneratedReport(base))

	withoutTag := *base
	withoutTag.Tags = nil
	assert.False(t, isGeneratedReport(&withoutTag))

	userAuthored := *base
	userAuthored.Source = "user"
	assert.False(t, isGeneratedReport(&userAuthored))

	compiled := *base
	compiled.Type = domain.MemoryTypeCompiled
	compiled.Category = "compiled"
	compiled.Tags = []string{"auto-compiled", "autopilot"}
	assert.False(t, isGeneratedReport(&compiled))
}

func TestAutoInjectSignalRequiresSuccessfulPayload(t *testing.T) {
	now := time.Now().UTC()
	skipped := autoInjectSignal([]Event{{
		Timestamp: now,
		Feature:   "auto_inject",
		Attrs:     map[string]string{"outcome": "skipped:no_memories", "payload": "false"},
	}}, true)
	assert.Equal(t, CheckWarn, skipped.Status)

	success := autoInjectSignal([]Event{{
		Timestamp: now,
		Feature:   "auto_inject",
		Attrs:     map[string]string{"outcome": "ok", "payload": "true"},
	}}, true)
	assert.Equal(t, CheckPass, success.Status)
}

func TestCompileSignalUsesSourcesAfterLatestArticle(t *testing.T) {
	now := time.Now().UTC()
	article := &domain.Memory{
		ID: "compiled", ProjectID: "hms", Type: domain.MemoryTypeCompiled,
		Source: "autopilot-daemon", Metadata: map[string]string{"compiled_by": "autopilot"},
		CreatedAt: now.Add(-time.Hour), Status: domain.MemoryStatusActive,
	}
	memories := []*domain.Memory{article}
	for i := 0; i < 4; i++ {
		memories = append(memories, &domain.Memory{
			ID: "source-old-" + string(rune('a'+i)), ProjectID: "hms",
			Type: domain.MemoryTypeLongTerm, CreatedAt: now.Add(-2 * time.Hour),
			Status: domain.MemoryStatusActive,
		})
	}
	assert.Equal(t, CheckPass, compileSignal(memories, 5, "hms").Status)

	for i := 0; i < 5; i++ {
		memories = append(memories, &domain.Memory{
			ID: "source-new-" + string(rune('a'+i)), ProjectID: "hms",
			Type: domain.MemoryTypeLongTerm, CreatedAt: now.Add(time.Duration(i) * time.Minute),
			Status: domain.MemoryStatusActive,
		})
	}
	assert.Equal(t, CheckFail, compileSignal(memories, 5, "hms").Status)
}

func TestAggregateUnknownDailyAndLaunch(t *testing.T) {
	signals := []CheckSignal{{Name: "version", Status: CheckUnknown, Critical: true}}
	assert.Equal(t, CheckWarn, aggregateCheckStatus(signals, false))
	assert.Equal(t, CheckFail, aggregateCheckStatus(signals, true))
}

func TestMissingMCPRuntimeSignalIsOptionalForDailyChecks(t *testing.T) {
	daily := missingMCPRuntimeSignal(false)
	assert.Equal(t, "mcp runtime", daily.Name)
	assert.Equal(t, CheckPass, daily.Status)
	assert.False(t, daily.Critical)
	assert.Equal(t, "not checked", daily.Value)

	launch := missingMCPRuntimeSignal(true)
	assert.Equal(t, CheckUnknown, launch.Status)
	assert.True(t, launch.Critical)
	assert.Equal(t, "not provided", launch.Value)
}

func TestFeatureHealthSparseTrafficDoesNotFail(t *testing.T) {
	now := time.Now().UTC()
	events := []Event{
		{Timestamp: now, Feature: "store_call"},
		{Timestamp: now, Feature: "quality_gate"},
		{Timestamp: now, Feature: "dedup"},
		{Timestamp: now, Feature: "context_call"},
		{Timestamp: now, Feature: "mmr"},
	}

	signal := featureHealthSignal(events)
	assert.Equal(t, CheckUnknown, signal.Status)
	assert.Contains(t, signal.Value, "0 critical low")
}

func TestVersionFromExecutablePathResolvesHomebrewSymlink(t *testing.T) {
	root := t.TempDir()
	targetDir := filepath.Join(root, "Caskroom", "mnemos", "1.1.16")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))
	target := filepath.Join(targetDir, "mnemos")
	require.NoError(t, os.WriteFile(target, []byte("binary"), 0o755))
	link := filepath.Join(root, "bin", "mnemos")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0o755))
	require.NoError(t, os.Symlink(target, link))

	assert.Equal(t, "1.1.16", versionFromExecutablePath(link))
}

func TestDatabaseSignalFailsForMissingDatabase(t *testing.T) {
	signal := databaseSignal(filepath.Join(t.TempDir(), "missing.db"))

	assert.Equal(t, CheckFail, signal.Status)
	assert.Equal(t, "database", signal.Name)
	assert.Equal(t, "missing", signal.Value)
	assert.Contains(t, signal.Explanation, "does not exist")
}

func TestAddSignalActionsRoutesDatabaseToDoctor(t *testing.T) {
	report := &CheckReport{Signals: []CheckSignal{{
		Name: "database", Status: CheckFail, Critical: true,
	}}}

	addSignalActions(report)

	require.Len(t, report.ActionItems, 1)
	assert.Equal(t, "mnemos doctor database", report.ActionItems[0].Command)
}

func TestBenchmarkSignalIsLaunchOnly(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, benchmark.WriteBenchMode(dataDir, benchmark.BenchModeOn))
	logPath := benchmarkLogPath(dataDir)
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0o755))

	now := time.Now().UTC()
	var lines []string
	for i := 0; i < 5; i++ {
		start := now.Add(time.Duration(i) * time.Minute)
		end := start.Add(time.Minute)
		sessionID := "sess-" + string(rune('a'+i))
		lines = append(lines,
			start.Format(time.RFC3339)+"\tbench_session_start\tsession_id="+sessionID+" project_id=proj-1 mode=on category=feature timestamp="+start.Format(time.RFC3339)+" provenance=production",
			end.Format(time.RFC3339)+"\tbench_session_end\tsession_id="+sessionID+" project_id=proj-1 mode=on category=feature timestamp="+end.Format(time.RFC3339)+" task_completed=true provenance=production",
		)
	}
	require.NoError(t, os.WriteFile(logPath, []byte(strings.Join(lines, "\n")+"\n"), 0o644))

	daily := benchmarkSignal(dataDir, logPath, "", false)
	assert.Equal(t, CheckPass, daily.Status)
	assert.False(t, daily.Critical)
	assert.Contains(t, daily.Explanation, "optional")

	launch := benchmarkSignal(dataDir, logPath, "", true)
	assert.Equal(t, CheckFail, launch.Status)
	assert.True(t, launch.Critical)
	assert.Contains(t, launch.Explanation, "launch readiness")
}

func TestApplyCheckFixesArchivesOnlyVerifiedReport(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "mnemos.db")
	db, err := sqlitestore.Open(dbPath)
	require.NoError(t, err)
	store := sqlitestore.NewSQLiteStore(db)
	now := time.Now().UTC()
	report := &domain.Memory{
		ID: "report-1", Content: "generated", Summary: "", Type: domain.MemoryTypeSemantic,
		Category: "autopilot", Source: "autopilot-daemon", Tags: []string{"autopilot-report"},
		ProjectID: "hms", CreatedAt: now, UpdatedAt: now, LastAccessedAt: now,
		RelevanceScore: 1, QualityScore: 1, Status: domain.MemoryStatusActive, ContentHash: "report-hash",
	}
	require.NoError(t, store.Create(context.Background(), report))
	require.NoError(t, db.Close())

	checkReport := CheckReport{Mutations: []Mutation{{
		MemoryID: "report-1", Project: "hms",
		Old: string(domain.MemoryStatusActive), New: string(domain.MemoryStatusArchived),
	}}}
	require.NoError(t, applyCheckFixes(context.Background(), dbPath, &checkReport))

	verifyDB, err := sqlitestore.OpenReadOnly(dbPath)
	require.NoError(t, err)
	defer verifyDB.Close()
	verified, err := sqlitestore.NewSQLiteStore(verifyDB).GetByID(context.Background(), "report-1")
	require.NoError(t, err)
	assert.Equal(t, domain.MemoryStatusArchived, verified.Status)
}

func TestCheckCommandJSON(t *testing.T) {
	mn := buildEvalMnemos(t)
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Autopilot.Enabled = false
	cmd := newCheckCmd(cfg, mn, "1.1.14")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--json"})
	err := cmd.Execute()
	assert.Error(t, err)

	var report CheckReport
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))
	assert.NotEmpty(t, report.Signals)
	assert.Equal(t, CheckFail, report.Status)
}

func TestCheckCommandLaunchJSON(t *testing.T) {
	mn := buildEvalMnemos(t)
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.Autopilot.Enabled = false
	cmd := newCheckCmd(cfg, mn, "1.1.14")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--launch"})
	require.Error(t, cmd.Execute())

	var report CheckReport
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))
	assert.Equal(t, CheckFail, report.Status)
}

func TestCheckCommandFixArchivesOlderReports(t *testing.T) {
	dataDir := t.TempDir()
	dbPath := filepath.Join(dataDir, "mnemos.db")
	mn := buildCheckFileMnemos(t, dbPath)
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		_, err := mn.StoreWithoutGate(ctx, &domain.StoreRequest{
			Content:   "generated report content " + string(rune('a'+i)),
			Type:      domain.MemoryTypeSemantic,
			Category:  "autopilot",
			Tags:      []string{"autopilot-report"},
			Source:    "autopilot-daemon",
			ProjectID: "hms",
		})
		require.NoError(t, err)
		time.Sleep(time.Millisecond)
	}

	cfg := config.DefaultConfig()
	cfg.DataDir = dataDir
	cfg.Autopilot.Enabled = false
	cmd := newCheckCmd(cfg, mn, "1.1.14")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--json", "--fix"})
	require.Error(t, cmd.Execute()) // Other critical signals intentionally fail.

	var report CheckReport
	require.NoError(t, json.Unmarshal(out.Bytes(), &report))
	assert.True(t, report.FixApplied)
	assert.Len(t, report.Mutations, 1)

	active, err := mn.List(ctx, storage.ListQuery{
		ProjectID: "hms", Statuses: []domain.MemoryStatus{domain.MemoryStatusActive}, Limit: 10,
	})
	require.NoError(t, err)
	archived, err := mn.List(ctx, storage.ListQuery{
		ProjectID: "hms", Statuses: []domain.MemoryStatus{domain.MemoryStatusArchived}, Limit: 10,
	})
	require.NoError(t, err)
	assert.Len(t, active, 1)
	assert.Len(t, archived, 1)
}

func buildCheckFileMnemos(t *testing.T, dbPath string) *core.Mnemos {
	t.Helper()
	db, err := sqlitestore.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	store := sqlitestore.NewSQLiteStore(db)
	embeddings := sqlitestore.NewEmbeddingStore(db)
	fts := sqlitestore.NewFTSSearcher(db)
	relations := sqlitestore.NewRelationStore(db)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	manager := coremem.NewManager(store, embeddings, nil, markdown.NewMirror("", false), 0.85, 0.92, logger, nil)
	t.Cleanup(manager.Stop)
	engine := search.NewSearchEngine(fts, embeddings, nil, relations, logger, 0.7, 0)
	relationManager := relation.NewManager(relations, store, logger)
	lifecycleEngine := lifecycle.NewEngine(store, 24*time.Hour, 30, 0.1, false, logger)
	return core.NewMnemos(manager, engine, relationManager, lifecycleEngine, store, logger)
}

func TestCheckInvocationDetectionDoesNotMatchStoreContent(t *testing.T) {
	assert.True(t, isCheckInvocation([]string{"check"}))
	assert.True(t, isCheckInvocation([]string{"--config", "x.yaml", "check"}))
	assert.False(t, isCheckInvocation([]string{"store", "check"}))
	assert.True(t, isServerInvocation([]string{"serve"}))
	assert.False(t, isServerInvocation([]string{"stats"}))
}

func TestMissingDatabaseBootstrapDoesNotCreateFile(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.DataDir = t.TempDir()
	dbPath := cfg.DBPath()
	_, err := sqlitestore.OpenReadOnly(dbPath)
	require.Error(t, err)
	_, statErr := os.Stat(dbPath)
	assert.True(t, os.IsNotExist(statErr))
}
