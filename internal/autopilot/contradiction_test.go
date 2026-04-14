package autopilot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// contradictionCfg returns a config with contradiction detection enabled.
func contradictionCfg() config.AutopilotConfig {
	cfg := defaultCfg()
	cfg.ContradictionEnabled = true
	cfg.ContradictionThreshold = 0.3
	cfg.MaxContradictionPairs = 100
	return cfg
}

// TestContradictionDetector_DisabledByDefault: default config → 0 findings, no storage queries.
func TestContradictionDetector_DisabledByDefault(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()

	// Use default config — contradiction_enabled=false
	cfg := defaultCfg()
	// Verify it's disabled
	if cfg.ContradictionEnabled {
		t.Fatal("expected ContradictionEnabled=false by default")
	}

	detector := NewContradictionDetector(env.mnemos, cfg)

	// Build entity maps with some data
	maps := EntityMaps{
		Entities: map[string][]string{
			"mem-a": {"internal/foo/bar.go", "internal/baz/qux.go", "MNEMOS_DB_PATH"},
			"mem-b": {"internal/foo/bar.go", "internal/baz/qux.go", "MNEMOS_DB_PATH"},
		},
		CreatedAt: map[string]time.Time{
			"mem-a": time.Now(),
			"mem-b": time.Now(),
		},
	}

	findings, err := detector.Detect(ctx, "proj-disabled", maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when disabled, got %d", len(findings))
	}
}

// TestContradictionDetector_Gate1_InsufficientEntities: 2 shared entities → Gate 1 fails, 0 findings.
func TestContradictionDetector_Gate1_InsufficientEntities(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-gate1"

	now := time.Now().UTC()
	memA := createMemory(t, env.store, util.NewID(), projectID,
		"internal/foo/bar.go works great, MNEMOS_DB_PATH is set",
		domain.MemoryTypeSemantic, now, now)
	memB := createMemory(t, env.store, util.NewID(), projectID,
		"internal/foo/bar.go is broken, MNEMOS_DB_PATH is wrong",
		domain.MemoryTypeSemantic, now, now)

	// Only 2 shared entities — Gate 1 requires 3+
	maps := EntityMaps{
		Entities: map[string][]string{
			memA.ID: {"internal/foo/bar.go", "MNEMOS_DB_PATH"},
			memB.ID: {"internal/foo/bar.go", "MNEMOS_DB_PATH"},
		},
		CreatedAt: map[string]time.Time{
			memA.ID: now,
			memB.ID: now,
		},
	}

	detector := NewContradictionDetector(env.mnemos, contradictionCfg())
	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (Gate 1 should fail with 2 shared entities), got %d", len(findings))
	}
}

// TestContradictionDetector_Gate2_LowOverlapScore: 3 shared entities but ratio below threshold → Gate 2 fails.
func TestContradictionDetector_Gate2_LowOverlapScore(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-gate2"

	now := time.Now().UTC()
	// memA has 3 shared + 7 unique = 10 total entities → overlap = 3/10 = 0.3
	// Set threshold to 0.5 so Gate 2 fails
	memA := createMemory(t, env.store, util.NewID(), projectID,
		"internal/foo/bar.go works, internal/baz/qux.go works, MNEMOS_DB_PATH set, "+
			"internal/a/b.go internal/c/d.go internal/e/f.go internal/g/h.go internal/i/j.go internal/k/l.go internal/m/n.go",
		domain.MemoryTypeSemantic, now, now)
	memB := createMemory(t, env.store, util.NewID(), projectID,
		"internal/foo/bar.go broken, internal/baz/qux.go broken, MNEMOS_DB_PATH wrong",
		domain.MemoryTypeSemantic, now, now)

	sharedEntities := []string{"internal/foo/bar.go", "internal/baz/qux.go", "MNEMOS_DB_PATH"}
	// memA has 10 entities, memB has 3 → overlap = 3/10 = 0.3, threshold = 0.5 → Gate 2 fails
	maps := EntityMaps{
		Entities: map[string][]string{
			memA.ID: append(sharedEntities, "internal/a/b.go", "internal/c/d.go", "internal/e/f.go", "internal/g/h.go", "internal/i/j.go", "internal/k/l.go", "internal/m/n.go"),
			memB.ID: sharedEntities,
		},
		CreatedAt: map[string]time.Time{
			memA.ID: now,
			memB.ID: now,
		},
	}

	cfg := contradictionCfg()
	cfg.ContradictionThreshold = 0.5 // higher threshold so 3/10 fails

	detector := NewContradictionDetector(env.mnemos, cfg)
	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (Gate 2 should fail with overlap=0.3 < threshold=0.5), got %d", len(findings))
	}
}

// TestContradictionDetector_Gate3_SameSignals: both memories have positive signals → Gate 3 fails.
func TestContradictionDetector_Gate3_SameSignals(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-gate3"

	now := time.Now().UTC()
	// Both memories have positive signals near the shared entities
	memA := createMemory(t, env.store, util.NewID(), projectID,
		"internal/foo/bar.go works great, internal/baz/qux.go is fixed, MNEMOS_DB_PATH should be set",
		domain.MemoryTypeSemantic, now, now)
	memB := createMemory(t, env.store, util.NewID(), projectID,
		"internal/foo/bar.go works fine, internal/baz/qux.go is correct, MNEMOS_DB_PATH use this",
		domain.MemoryTypeSemantic, now, now)

	sharedEntities := []string{"internal/foo/bar.go", "internal/baz/qux.go", "MNEMOS_DB_PATH"}
	maps := EntityMaps{
		Entities: map[string][]string{
			memA.ID: sharedEntities,
			memB.ID: sharedEntities,
		},
		CreatedAt: map[string]time.Time{
			memA.ID: now,
			memB.ID: now,
		},
	}

	detector := NewContradictionDetector(env.mnemos, contradictionCfg())
	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings (Gate 3 should fail: both have positive signals), got %d", len(findings))
	}
}

// TestContradictionDetector_AllGatesPass: 3+ shared entities, sufficient ratio, opposing signals → 1 finding.
func TestContradictionDetector_AllGatesPass(t *testing.T) {
	env := newStalenessTestEnv(t)
	ctx := context.Background()
	projectID := "proj-all-gates"

	now := time.Now().UTC()
	// memA: positive signals near shared entities
	memA := createMemory(t, env.store, util.NewID(), projectID,
		"internal/foo/bar.go works great, internal/baz/qux.go is fixed, MNEMOS_DB_PATH should be used",
		domain.MemoryTypeSemantic, now, now)
	// memB: negative signals near the same entities
	memB := createMemory(t, env.store, util.NewID(), projectID,
		"internal/foo/bar.go is broken, internal/baz/qux.go has a bug, MNEMOS_DB_PATH is wrong",
		domain.MemoryTypeSemantic, now, now)

	sharedEntities := []string{"internal/foo/bar.go", "internal/baz/qux.go", "MNEMOS_DB_PATH"}
	maps := EntityMaps{
		Entities: map[string][]string{
			memA.ID: sharedEntities,
			memB.ID: sharedEntities,
		},
		CreatedAt: map[string]time.Time{
			memA.ID: now,
			memB.ID: now,
		},
	}

	detector := NewContradictionDetector(env.mnemos, contradictionCfg())
	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}

	f := findings[0]
	if f.Type != FindingPotentialContradiction {
		t.Errorf("expected FindingPotentialContradiction, got %s", f.Type)
	}
	if _, ok := f.Metadata["overlap_score"]; !ok {
		t.Error("expected overlap_score in metadata")
	}
	if _, ok := f.Metadata["opposing_signals"]; !ok {
		t.Error("expected opposing_signals in metadata")
	}
	opposingSignals, ok := f.Metadata["opposing_signals"].([]string)
	if !ok || len(opposingSignals) == 0 {
		t.Errorf("expected non-empty opposing_signals, got %v", f.Metadata["opposing_signals"])
	}
	// Verify format: "positive vs negative"
	for _, sig := range opposingSignals {
		if !strings.Contains(sig, " vs ") {
			t.Errorf("opposing signal %q should contain ' vs '", sig)
		}
	}
}

// TestContainSignalNearEntity_WithinWindow: signal within 10 words of entity → true.
func TestContainSignalNearEntity_WithinWindow(t *testing.T) {
	// Entity at position 0, signal at position 5 (within 10 words)
	content := "internal/foo/bar.go is a great file that works well in production"
	entities := []string{"internal/foo/bar.go"}
	signals := []string{"works"}

	if !containsSignalNearEntity(content, entities, signals) {
		t.Error("expected true: signal 'works' is within 10 words of entity")
	}
}

// TestContainSignalNearEntity_OutsideWindow: signal 15 words from entity → false.
func TestContainSignalNearEntity_OutsideWindow(t *testing.T) {
	// Build content where entity is at start and signal is 15+ words away
	// "internal/foo/bar.go w1 w2 w3 w4 w5 w6 w7 w8 w9 w10 w11 w12 w13 w14 works"
	// entity at index 0, signal at index 15 → distance = 15 > 10
	content := "internal/foo/bar.go w1 w2 w3 w4 w5 w6 w7 w8 w9 w10 w11 w12 w13 w14 works"
	entities := []string{"internal/foo/bar.go"}
	signals := []string{"works"}

	if containsSignalNearEntity(content, entities, signals) {
		t.Error("expected false: signal 'works' is 15 words from entity, outside the 10-word window")
	}
}
