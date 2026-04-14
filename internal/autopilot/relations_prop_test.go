package autopilot

import (
	"context"
	"fmt"
	"testing"
	"testing/quick"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// TestProp_EntityExtractionDeterministic: same content → same entity set on every call.
//
// Validates: Requirements DETECT-2 (entity extraction is deterministic, no randomness or state dependency)
func TestProp_EntityExtractionDeterministic(t *testing.T) {
	f := func(content string) bool {
		a := ExtractEntities(content)
		b := ExtractEntities(content)
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if err := quick.Check(f, nil); err != nil {
		t.Error(err)
	}
}

// TestProp_RelationCreationIdempotent: N runs with same EntityMaps → same relation count as 1 run.
//
// Validates: Requirements DETECT-2.4 (GetRelationBetween check prevents duplicate creation)
func TestProp_RelationCreationIdempotent(t *testing.T) {
	f := func(seed uint8) bool {
		// Use seed to generate a small deterministic set of memories (1–4 memories)
		n := int(seed%4) + 2 // 2 to 5 memories
		env := newRelationTestEnv(t)
		ctx := context.Background()
		projectID := fmt.Sprintf("prop-proj-%d", seed)

		// Fixed entity-rich contents so memories share entities
		contents := []string{
			"see internal/storage/sqlite/store.go for the implementation",
			"bug in internal/storage/sqlite/store.go causes SQLiteStore.GetByID to fail",
			"fix internal/storage/sqlite/store.go and update SQLiteStore.GetByID",
			"MNEMOS_DB_PATH controls internal/storage/sqlite/store.go location",
			"mnemos serve reads internal/storage/sqlite/store.go on startup",
		}

		memories := make([]*domain.Memory, n)
		for i := 0; i < n; i++ {
			now := time.Now().UTC()
			m := &domain.Memory{
				ID:             util.NewID(),
				Content:        contents[i%len(contents)],
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
			if err := env.store.Create(ctx, m); err != nil {
				return false
			}
			memories[i] = m
		}

		maps := BuildEntityMaps(memories)
		detector := NewRelationDetector(env.mnemos, defaultCfg())

		// First run
		if _, err := detector.Detect(ctx, projectID, maps); err != nil {
			return false
		}

		// Count relations after first run
		rels1, err := env.relStore.ListRelations(ctx, storage.RelationQuery{
			RelationTypes: []domain.RelationType{domain.RelationTypeRelatesTo},
		})
		if err != nil {
			return false
		}
		count1 := len(rels1)

		// Second run with same maps
		if _, err := detector.Detect(ctx, projectID, maps); err != nil {
			return false
		}

		// Count relations after second run — must be identical
		rels2, err := env.relStore.ListRelations(ctx, storage.RelationQuery{
			RelationTypes: []domain.RelationType{domain.RelationTypeRelatesTo},
		})
		if err != nil {
			return false
		}
		count2 := len(rels2)

		return count1 == count2
	}

	if err := quick.Check(f, &quick.Config{MaxCount: 20}); err != nil {
		t.Error(err)
	}
}
