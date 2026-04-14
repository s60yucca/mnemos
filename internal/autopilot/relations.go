package autopilot

import (
	"context"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
)

// pairKey uniquely identifies an unordered pair of memory IDs.
type pairKey struct{ a, b string }

// normalizePair returns IDs in alphabetical order so (A,B) and (B,A) produce the same key.
func normalizePair(a, b string) (string, string) {
	if a > b {
		return b, a
	}
	return a, b
}

// RelationDetector creates relates_to relations between memories that share structural entities.
type RelationDetector struct {
	mnemos *core.Mnemos
	cfg    config.AutopilotConfig
}

// NewRelationDetector constructs a RelationDetector.
func NewRelationDetector(mnemos *core.Mnemos, cfg config.AutopilotConfig) *RelationDetector {
	return &RelationDetector{mnemos: mnemos, cfg: cfg}
}

// Name returns the detector identifier.
func (d *RelationDetector) Name() string { return "relations" }

// Detect consumes the shared entity map, creates relates_to relations for co-referenced
// memory pairs, and returns a single FindingRelationsCreated if any were created.
func (d *RelationDetector) Detect(ctx context.Context, projectID string, maps EntityMaps) ([]Finding, error) {
	// Invert the entity map: entity → []memoryID
	entityIndex := make(map[string][]string)
	for memID, entities := range maps.Entities {
		for _, entity := range entities {
			entityIndex[entity] = append(entityIndex[entity], memID)
		}
	}

	seen := make(map[pairKey]bool)
	newRelations := 0
	sampleEntity := ""

	for entity, memIDs := range entityIndex {
		if len(memIDs) < 2 {
			continue
		}
		for i := 0; i < len(memIDs); i++ {
			for j := i + 1; j < len(memIDs); j++ {
				a, b := normalizePair(memIDs[i], memIDs[j])
				key := pairKey{a, b}
				if seen[key] {
					continue
				}
				seen[key] = true

				existing, err := d.mnemos.GetRelationBetween(ctx, a, b, domain.RelationTypeRelatesTo)
				if err != nil {
					// Conservative: skip pair on error to avoid duplicates
					continue
				}
				if existing != nil {
					continue
				}

				_, err = d.mnemos.Relate(ctx, &domain.RelateRequest{
					SourceID:     a,
					TargetID:     b,
					RelationType: domain.RelationTypeRelatesTo,
					Strength:     0.5,
					Metadata: map[string]string{
						"entity":      entity,
						"detected_by": "autopilot",
					},
				})
				if err != nil {
					continue
				}
				newRelations++
				if sampleEntity == "" {
					sampleEntity = entity
				}
			}
		}
	}

	if newRelations > 0 {
		return []Finding{{
			Type: FindingRelationsCreated,
			Metadata: map[string]any{
				"count":         newRelations,
				"sample_entity": sampleEntity,
			},
		}}, nil
	}
	return nil, nil
}
