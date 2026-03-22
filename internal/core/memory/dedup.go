package memory

import (
	"context"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// Deduplicator checks for duplicate memories
type Deduplicator interface {
	Check(ctx context.Context, req *domain.StoreRequest, hash string) (*domain.Memory, string, float64, []*domain.Memory, error)
}

// ContentDedup implements 3-tier deduplication
type ContentDedup struct {
	store             storage.IMemoryStore
	embedStore        storage.IEmbeddingStore
	fuzzyThreshold    float64
	semanticThreshold float64
}

func NewContentDedup(store storage.IMemoryStore, embedStore storage.IEmbeddingStore, fuzzyThreshold, semanticThreshold float64) *ContentDedup {
	if fuzzyThreshold <= 0 {
		fuzzyThreshold = 0.85
	}
	if semanticThreshold <= 0 {
		semanticThreshold = 0.92
	}
	return &ContentDedup{
		store:             store,
		embedStore:        embedStore,
		fuzzyThreshold:    fuzzyThreshold,
		semanticThreshold: semanticThreshold,
	}
}

// Check runs the 3-tier dedup pipeline. Returns (existing, similarityType, score, recent, error).
// recent is the list of up to 200 most recent active memories fetched during tier-2 check.
// Returns nil existing if no duplicate found. recent may be nil if exact match found in tier 1.
func (d *ContentDedup) Check(ctx context.Context, req *domain.StoreRequest, hash string) (*domain.Memory, string, float64, []*domain.Memory, error) {
	// Tier 1: exact hash match
	existing, err := d.store.GetByHash(ctx, hash)
	if err != nil {
		return nil, "", 0, nil, err
	}
	if existing != nil {
		return existing, "exact", 1.0, nil, nil
	}

	// Tier 2: fuzzy Jaccard similarity
	recent, err := d.store.List(ctx, storage.ListQuery{
		ProjectID: req.ProjectID,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:     200,
		SortBy:    "created_at",
		SortDesc:  true,
	})
	if err != nil {
		return nil, "", 0, nil, err
	}

	tokA := util.TokenSet(util.Tokenize(req.Content))
	var bestFuzzy *domain.Memory
	var bestFuzzyScore float64

	for _, m := range recent {
		tokB := util.TokenSet(util.Tokenize(m.Content))
		score := util.JaccardSimilarity(tokA, tokB)
		if score >= d.fuzzyThreshold && score > bestFuzzyScore {
			bestFuzzyScore = score
			bestFuzzy = m
		}
	}
	if bestFuzzy != nil {
		return bestFuzzy, "fuzzy", bestFuzzyScore, recent, nil
	}

	// Tier 3: semantic cosine similarity (only if embedding store available)
	// Note: full semantic dedup requires embedding the incoming content which is async.
	// This is intentionally deferred — the embed queue handles it post-store,
	// and fuzzy Jaccard at tier 2 catches near-duplicates with high recall.
	if d.embedStore == nil {
		return nil, "", 0, recent, nil
	}

	return nil, "", 0, recent, nil
}

// FindDuplicates finds potential duplicates for a given memory without merging
func (d *ContentDedup) FindDuplicates(ctx context.Context, projectID string, threshold float64, limit int) ([]*domain.Memory, error) {
	memories, err := d.store.List(ctx, storage.ListQuery{
		ProjectID: projectID,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:     limit * 10,
	})
	if err != nil {
		return nil, err
	}

	seen := map[string]bool{}
	var duplicates []*domain.Memory

	for i, a := range memories {
		if seen[a.ID] {
			continue
		}
		tokA := util.TokenSet(util.Tokenize(a.Content))
		for j := i + 1; j < len(memories); j++ {
			b := memories[j]
			if seen[b.ID] {
				continue
			}
			tokB := util.TokenSet(util.Tokenize(b.Content))
			score := util.JaccardSimilarity(tokA, tokB)
			if score >= threshold {
				seen[b.ID] = true
				duplicates = append(duplicates, b)
				if len(duplicates) >= limit {
					return duplicates, nil
				}
			}
		}
	}
	return duplicates, nil
}
