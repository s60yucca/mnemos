package search

import (
	"sort"

	"github.com/mnemos-dev/mnemos/internal/storage"
)

const rrfK = 60.0

// ReciprocRankFusion fuses text and semantic results using RRF
// score(d) = Σ 1/(k + rank_i(d))
func ReciprocRankFusion(textResults, semanticResults []*storage.SearchResult, k float64) []*storage.SearchResult {
	if k <= 0 {
		k = rrfK
	}

	scores := make(map[string]float64)
	byID := make(map[string]*storage.SearchResult)

	for rank, r := range textResults {
		scores[r.Memory.ID] += 1.0 / (k + float64(rank+1))
		if _, ok := byID[r.Memory.ID]; !ok {
			byID[r.Memory.ID] = r
		}
	}
	for rank, r := range semanticResults {
		scores[r.Memory.ID] += 1.0 / (k + float64(rank+1))
		if _, ok := byID[r.Memory.ID]; !ok {
			byID[r.Memory.ID] = r
		}
	}

	results := make([]*storage.SearchResult, 0, len(scores))
	for id, score := range scores {
		r := *byID[id] // copy
		r.HybridScore = score
		r.Source = "hybrid"
		results = append(results, &r)
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].HybridScore > results[j].HybridScore
	})
	return results
}
