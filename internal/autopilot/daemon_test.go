package autopilot

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// testLogger returns a silent logger for tests.
func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// --- mock reportWriter ---

type mockWriter struct {
	mu    sync.Mutex
	calls []writerCall
}

type writerCall struct {
	projectID string
	findings  []Finding
}

func (w *mockWriter) Write(_ context.Context, projectID string, findings []Finding) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.calls = append(w.calls, writerCall{projectID: projectID, findings: findings})
	return nil
}

func (w *mockWriter) callCount() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.calls)
}

// --- panic detector ---

type panicDetector struct{}

func (p *panicDetector) Name() string { return "panic-detector" }
func (p *panicDetector) Detect(_ context.Context, _ string, _ EntityMaps) ([]Finding, error) {
	panic("intentional test panic")
}

// --- helpers ---

func newDaemonTestEnv(t *testing.T) (*stalenessTestEnv, *mockWriter) {
	t.Helper()
	env := newStalenessTestEnv(t)
	w := &mockWriter{}
	return env, w
}

func newTestDaemon(env *stalenessTestEnv, w *mockWriter) *AutopilotDaemon {
	cfg := defaultCfg()
	cfg.InitialDelay = 10 * time.Millisecond
	cfg.Interval = 100 * time.Millisecond
	return NewAutopilotDaemon(env.mnemos, cfg, testLogger(), w)
}

// TestDaemon_IdleSkip: store memories, run once, run again without new memories → second run writes no report
func TestDaemon_IdleSkip(t *testing.T) {
	env, w := newDaemonTestEnv(t)
	ctx := context.Background()
	projectID := "proj-idle-skip"

	// Store a memory so the project has activity
	now := time.Now().UTC()
	createMemory(t, env.store, util.NewID(), projectID, "see internal/storage/sqlite/store.go", domain.MemoryTypeSemantic, now, now)

	daemon := newTestDaemon(env, w)

	// First run — should process
	_, err := daemon.RunOnce(ctx, projectID, false)
	if err != nil {
		t.Fatalf("first RunOnce: %v", err)
	}
	firstCallCount := w.callCount()

	// Manually set lastRun to now so the idle check triggers
	daemon.mu.Lock()
	daemon.lastRun[projectID] = time.Now()
	daemon.mu.Unlock()

	// Second run via runCycle — should skip because no new memories
	daemon.runCycle(ctx)

	// Writer should not have been called again
	if w.callCount() != firstCallCount {
		t.Errorf("expected no new writer calls after idle skip, got %d total (was %d)", w.callCount(), firstCallCount)
	}
}

// TestDaemon_PanicRecovery: one detector always panics → other detectors still run, daemon continues
func TestDaemon_PanicRecovery(t *testing.T) {
	env, w := newDaemonTestEnv(t)
	ctx := context.Background()
	projectID := "proj-panic"

	now := time.Now().UTC()
	createMemory(t, env.store, util.NewID(), projectID, "see internal/storage/sqlite/store.go", domain.MemoryTypeSemantic, now, now)
	createMemory(t, env.store, util.NewID(), projectID, "bug in internal/storage/sqlite/store.go", domain.MemoryTypeSemantic, now, now)

	daemon := newTestDaemon(env, w)
	// Inject a panicking detector at the front
	daemon.detectors = append([]Detector{&panicDetector{}}, daemon.detectors...)

	// Should not panic — safeDetect recovers
	findings, err := daemon.RunOnce(ctx, projectID, false)
	if err != nil {
		t.Fatalf("RunOnce should not return error after panic recovery: %v", err)
	}

	// The relation detector (non-panicking) should still have run and produced findings
	// (two memories share the same file path entity)
	hasRelationFinding := false
	for _, f := range findings {
		if f.Type == FindingRelationsCreated {
			hasRelationFinding = true
		}
	}
	if !hasRelationFinding {
		t.Error("expected relation findings from non-panicking detector, got none")
	}
}

// TestDaemon_StopWithinTimeout: Start then Stop → goroutine exits within 5s
func TestDaemon_StopWithinTimeout(t *testing.T) {
	env, w := newDaemonTestEnv(t)

	cfg := defaultCfg()
	cfg.InitialDelay = 10 * time.Second // long delay so it doesn't run during test
	cfg.Interval = 10 * time.Second

	daemon := NewAutopilotDaemon(env.mnemos, cfg, testLogger(), w)
	daemon.Start()

	done := make(chan struct{})
	go func() {
		daemon.Stop()
		close(done)
	}()

	select {
	case <-done:
		// success — stopped within timeout
	case <-time.After(6 * time.Second):
		t.Error("daemon did not stop within 5 seconds")
	}
}

// TestDaemon_DryRun: dryRun=true → no relations created, no report written, findings returned
func TestDaemon_DryRun(t *testing.T) {
	env, w := newDaemonTestEnv(t)
	ctx := context.Background()
	projectID := "proj-dry-run"

	now := time.Now().UTC()
	// Two memories sharing a file path — would normally create a relation
	m1 := createMemory(t, env.store, util.NewID(), projectID, "see internal/storage/sqlite/store.go", domain.MemoryTypeSemantic, now, now)
	m2 := createMemory(t, env.store, util.NewID(), projectID, "bug in internal/storage/sqlite/store.go", domain.MemoryTypeSemantic, now, now)

	daemon := newTestDaemon(env, w)

	_, err := daemon.RunOnce(ctx, projectID, true)
	if err != nil {
		t.Fatalf("RunOnce dry run: %v", err)
	}

	// No report should be written
	if w.callCount() != 0 {
		t.Errorf("expected 0 writer calls in dry run, got %d", w.callCount())
	}

	// No relations should be created (relation detector is skipped in dry run)
	rel, err := env.relStore.GetRelationBetween(ctx, m1.ID, m2.ID, domain.RelationTypeRelatesTo)
	if err != nil {
		t.Fatalf("GetRelationBetween: %v", err)
	}
	if rel != nil {
		t.Error("expected no relation in dry run, but one was created")
	}
}

// TestDaemon_PerProjectIdleSkip: project A has new memories, project B does not → A runs, B skips
func TestDaemon_PerProjectIdleSkip(t *testing.T) {
	env, w := newDaemonTestEnv(t)
	ctx := context.Background()

	projectA := "proj-active"
	projectB := "proj-idle"

	now := time.Now().UTC()
	past := now.Add(-1 * time.Hour)

	// Project A: has a memory created now
	createMemory(t, env.store, util.NewID(), projectA, "see internal/storage/sqlite/store.go", domain.MemoryTypeSemantic, now, now)

	// Project B: has a memory created in the past
	createMemory(t, env.store, util.NewID(), projectB, "old memory internal/foo/bar.go", domain.MemoryTypeSemantic, past, past)

	daemon := newTestDaemon(env, w)

	// Set lastRun for project B to after its memory was created → should be skipped
	daemon.mu.Lock()
	daemon.lastRun[projectB] = now // now > past, so B is idle
	daemon.mu.Unlock()

	// Run the cycle
	daemon.runCycle(ctx)

	// Project A should have been processed — lastRun updated
	daemon.mu.Lock()
	lastRunA := daemon.lastRun[projectA]
	lastRunB := daemon.lastRun[projectB]
	daemon.mu.Unlock()

	if lastRunA.IsZero() {
		t.Error("expected project A lastRun to be updated after processing")
	}
	// Project B's lastRun should still be 'now' (unchanged, since it was skipped)
	if !lastRunB.Equal(now) {
		t.Errorf("expected project B lastRun to remain %v (skipped), got %v", now, lastRunB)
	}
}
