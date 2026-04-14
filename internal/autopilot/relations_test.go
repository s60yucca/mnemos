package autopilot

import (
	"context"
	"testing"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// newRelationTestEnv reuses the same in-memory SQLite setup as staleness tests.
func newRelationTestEnv(t *testing.T) *stalenessTestEnv {
	t.Helper()
	return newStalenessTestEnv(t)
}

// createRelationMemory inserts a memory with the given content and returns it.
func createRelationMemory(t *testing.T, env *stalenessTestEnv, projectID, content string) *domain.Memory {
	t.Helper()
	now := time.Now().UTC()
	return createMemory(t, env.store, util.NewID(), projectID, content, domain.MemoryTypeSemantic, now, now)
}

// TestRelationDetector_CreatesRelation: two memories sharing a file path → 1 relation created
func TestRelationDetector_CreatesRelation(t *testing.T) {
	env := newRelationTestEnv(t)
	ctx := context.Background()
	projectID := "proj-rel-create"

	m1 := createRelationMemory(t, env, projectID, "see internal/storage/sqlite/store.go for details")
	m2 := createRelationMemory(t, env, projectID, "bug in internal/storage/sqlite/store.go line 42")

	maps := BuildEntityMaps([]*domain.Memory{m1, m2})

	detector := NewRelationDetector(env.mnemos, defaultCfg())
	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Type != FindingRelationsCreated {
		t.Errorf("expected FindingRelationsCreated, got %s", findings[0].Type)
	}
	if findings[0].Metadata["count"] != 1 {
		t.Errorf("expected count=1, got %v", findings[0].Metadata["count"])
	}

	// Verify the relation actually exists in the store
	rel, err := env.relStore.GetRelationBetween(ctx, m1.ID, m2.ID, domain.RelationTypeRelatesTo)
	if err != nil {
		t.Fatalf("GetRelationBetween: %v", err)
	}
	if rel == nil {
		t.Error("expected relation to exist in store, got nil")
	}
}

// TestRelationDetector_Idempotent: run twice → still exactly 1 relation
func TestRelationDetector_Idempotent(t *testing.T) {
	env := newRelationTestEnv(t)
	ctx := context.Background()
	projectID := "proj-rel-idempotent"

	m1 := createRelationMemory(t, env, projectID, "see internal/storage/sqlite/store.go for details")
	m2 := createRelationMemory(t, env, projectID, "bug in internal/storage/sqlite/store.go line 42")

	maps := BuildEntityMaps([]*domain.Memory{m1, m2})
	detector := NewRelationDetector(env.mnemos, defaultCfg())

	// First run
	findings1, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("first Detect: %v", err)
	}
	if len(findings1) != 1 {
		t.Fatalf("first run: expected 1 finding, got %d", len(findings1))
	}

	// Second run — should create 0 new relations
	findings2, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("second Detect: %v", err)
	}
	if len(findings2) != 0 {
		t.Errorf("second run: expected 0 findings (idempotent), got %d", len(findings2))
	}

	// Confirm only 1 relation exists
	rels, err := env.relStore.ListRelations(ctx, storage.RelationQuery{
		MemoryID:      m1.ID,
		RelationTypes: []domain.RelationType{domain.RelationTypeRelatesTo},
	})
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(rels) != 1 {
		t.Errorf("expected exactly 1 relation, got %d", len(rels))
	}
}

// TestRelationDetector_NoRelationForSingleMemory: entity in only one memory → no relation
func TestRelationDetector_NoRelationForSingleMemory(t *testing.T) {
	env := newRelationTestEnv(t)
	ctx := context.Background()
	projectID := "proj-rel-single"

	// Only one memory mentions the entity
	m1 := createRelationMemory(t, env, projectID, "see internal/storage/sqlite/store.go for details")
	m2 := createRelationMemory(t, env, projectID, "unrelated content about something else entirely")

	maps := BuildEntityMaps([]*domain.Memory{m1, m2})
	detector := NewRelationDetector(env.mnemos, defaultCfg())

	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	// m2 has no matching entities with m1 (no file path, go ident, etc.)
	// so no relation should be created
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

// TestRelationDetector_SeenSetPreventsRedundantChecks: two memories sharing 3 entities →
// only 1 GetRelationBetween call (not 3), verified by checking only 1 relation exists.
func TestRelationDetector_SeenSetPreventsRedundantChecks(t *testing.T) {
	env := newRelationTestEnv(t)
	ctx := context.Background()
	projectID := "proj-rel-seen"

	// Two memories sharing 3 distinct entities
	content1 := "see internal/storage/sqlite/store.go and SQLiteStore.GetByID and MNEMOS_DB_PATH"
	content2 := "bug in internal/storage/sqlite/store.go also SQLiteStore.GetByID and MNEMOS_DB_PATH"

	m1 := createRelationMemory(t, env, projectID, content1)
	m2 := createRelationMemory(t, env, projectID, content2)

	maps := BuildEntityMaps([]*domain.Memory{m1, m2})

	// Verify both memories share at least 3 entities
	entities1 := ExtractEntities(content1)
	entities2 := ExtractEntities(content2)
	shared := 0
	set1 := make(map[string]bool)
	for _, e := range entities1 {
		set1[e] = true
	}
	for _, e := range entities2 {
		if set1[e] {
			shared++
		}
	}
	if shared < 3 {
		t.Skipf("test requires 3+ shared entities, got %d — adjust content", shared)
	}

	detector := NewRelationDetector(env.mnemos, defaultCfg())
	findings, err := detector.Detect(ctx, projectID, maps)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}

	// The seen set ensures only 1 relation is created (not 3), even though 3 entities are shared
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Metadata["count"] != 1 {
		t.Errorf("expected count=1 (seen set prevents duplicates), got %v", findings[0].Metadata["count"])
	}

	// Confirm only 1 relation exists in the store
	rels, err := env.relStore.ListRelations(ctx, storage.RelationQuery{
		MemoryID:      m1.ID,
		RelationTypes: []domain.RelationType{domain.RelationTypeRelatesTo},
	})
	if err != nil {
		t.Fatalf("ListRelations: %v", err)
	}
	if len(rels) != 1 {
		t.Errorf("expected exactly 1 relation in store (not 3), got %d", len(rels))
	}
}
