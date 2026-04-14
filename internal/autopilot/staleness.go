package autopilot

import (
	"context"

	"github.com/mnemos-dev/mnemos/internal/config"
	"github.com/mnemos-dev/mnemos/internal/core"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

// StalenessDetector finds compiled articles whose source memories have been
// updated or added since the article was compiled.
type StalenessDetector struct {
	mnemos *core.Mnemos
	cfg    config.AutopilotConfig
}

// NewStalenessDetector creates a new StalenessDetector.
func NewStalenessDetector(mnemos *core.Mnemos, cfg config.AutopilotConfig) *StalenessDetector {
	return &StalenessDetector{mnemos: mnemos, cfg: cfg}
}

// Name returns the detector name.
func (d *StalenessDetector) Name() string { return "staleness" }

// Detect queries compiled articles and finds those with newer source memories.
func (d *StalenessDetector) Detect(ctx context.Context, projectID string, maps EntityMaps) ([]Finding, error) {
	articles, err := d.mnemos.GetCompiledArticles(ctx, projectID, d.cfg.MaxCompiledPerRun)
	if err != nil {
		return nil, err
	}

	// Batch-collect all source IDs across all articles first.
	articleSources := make(map[string][]string) // articleID → []sourceMemoryID
	var allSourceIDs []string

	for _, article := range articles {
		rels, err := d.mnemos.ListRelations(ctx, storage.RelationQuery{
			MemoryID:      article.ID,
			RelationTypes: []domain.RelationType{domain.RelationTypeCompiledFrom},
			Direction:     "outgoing",
		})
		if err != nil {
			return nil, err
		}
		for _, rel := range rels {
			articleSources[article.ID] = append(articleSources[article.ID], rel.TargetMemoryID)
			allSourceIDs = append(allSourceIDs, rel.TargetMemoryID)
		}
	}

	// One batch fetch for all source memories.
	sourceMems, err := d.mnemos.GetByIDs(ctx, allSourceIDs)
	if err != nil {
		return nil, err
	}

	var findings []Finding

	for _, article := range articles {
		newerCount := 0
		sourceEntitySet := make(map[string]struct{})

		for _, sourceID := range articleSources[article.ID] {
			sourceMem, ok := sourceMems[sourceID]
			if !ok {
				continue // deleted or archived
			}
			if sourceMem.UpdatedAt.After(article.CreatedAt) {
				newerCount++
			}
			// Collect entities from source memories using shared map.
			for _, entity := range maps.Entities[sourceMem.ID] {
				sourceEntitySet[entity] = struct{}{}
			}
		}

		// Count post-article memories sharing entities with sources.
		relatedNewCount := 0
		for memID, entities := range maps.Entities {
			createdAt, ok := maps.CreatedAt[memID]
			if !ok {
				continue
			}
			if !createdAt.After(article.CreatedAt) {
				continue
			}
			for _, entity := range entities {
				if _, found := sourceEntitySet[entity]; found {
					relatedNewCount++
					break
				}
			}
		}

		if newerCount > 0 {
			topic := ""
			if article.Metadata != nil {
				topic = article.Metadata["topic"]
			}
			findings = append(findings, Finding{
				Type: FindingStaleCompiled,
				Metadata: map[string]any{
					"article_id":         article.ID,
					"article_topic":      topic,
					"newer_source_count": newerCount,
					"related_new_count":  relatedNewCount,
				},
			})
		}
	}

	return findings, nil
}
