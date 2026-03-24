package search

import (
	"context"
	"log/slog"
	"strings"
	"sync"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// ContextResult holds assembled context for a query
type ContextResult struct {
	Memories    []*domain.Memory         `json:"memories"`
	Relations   []*domain.MemoryRelation `json:"relations"`
	TotalTokens int                      `json:"total_tokens"`
}

// SearchEngine handles text, semantic, and hybrid search
type SearchEngine struct {
	fts        storage.ITextSearcher
	embedStore storage.IEmbeddingStore
	embedder   embedding.IEmbeddingProvider
	relations  storage.IRelationStore
	logger     *slog.Logger
}

func NewSearchEngine(
	fts storage.ITextSearcher,
	embedStore storage.IEmbeddingStore,
	embedder embedding.IEmbeddingProvider,
	relations storage.IRelationStore,
	logger *slog.Logger,
) *SearchEngine {
	return &SearchEngine{
		fts:        fts,
		embedStore: embedStore,
		embedder:   embedder,
		relations:  relations,
		logger:     logger,
	}
}

// TextSearch performs full-text search
func (e *SearchEngine) TextSearch(ctx context.Context, q storage.TextSearchQuery) ([]*storage.SearchResult, error) {
	return e.fts.Search(ctx, q)
}

// SemanticSearch embeds the query and performs vector similarity search
func (e *SearchEngine) SemanticSearch(ctx context.Context, query string, projectID string, limit int, minSim float64) ([]*storage.SearchResult, error) {
	if e.embedder == nil || e.embedStore == nil {
		return []*storage.SearchResult{}, nil
	}
	vec, err := e.embedder.Embed(ctx, query)
	if err != nil {
		e.logger.Warn("embed query failed", "err", err)
		return nil, nil
	}
	return e.embedStore.Search(ctx, storage.SemanticSearchQuery{
		Vector:        vec,
		ProjectID:     projectID,
		MinSimilarity: minSim,
		Limit:         limit,
	})
}

// HybridSearch runs text and semantic search in parallel and fuses with RRF
func (e *SearchEngine) HybridSearch(ctx context.Context, query, projectID string, limit int) ([]*storage.SearchResult, error) {
	var (
		textResults     []*storage.SearchResult
		semanticResults []*storage.SearchResult
		textErr         error
		wg              sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		textResults, textErr = e.fts.Search(ctx, storage.TextSearchQuery{
			Query:     query,
			ProjectID: projectID,
			Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
			Limit:     limit * 2,
		})
	}()

	if e.embedder != nil && e.embedStore != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			semanticResults, _ = e.SemanticSearch(ctx, query, projectID, limit*2, 0.5)
		}()
	}

	wg.Wait()

	if textErr != nil {
		return nil, textErr
	}

	if len(semanticResults) == 0 {
		// Fall back to text-only
		return textResults, nil
	}

	fused := ReciprocRankFusion(textResults, semanticResults, rrfK)
	if len(fused) > limit {
		fused = fused[:limit]
	}
	return fused, nil
}

// AssembleContext builds a context bundle for a query within token budget.
// Uses a 3-stage pipeline:
//  1. Candidate retrieval via HybridSearch (up to 20 results)
//  2. MMR diversity filter — balances relevance vs. redundancy (lambda=0.6)
//  3. Adaptive packing — chooses full/summary/one-line detail level by budget
func (e *SearchEngine) AssembleContext(ctx context.Context, query, projectID string, maxTokens int, includeRelations bool) (*ContextResult, error) {
	candidates, err := e.HybridSearch(ctx, query, projectID, 20)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return &ContextResult{}, nil
	}

	// Stage 2: MMR diversity filter
	selected := mmrSelect(candidates, maxTokens, 0.6)

	// Stage 3: Adaptive packing
	result := &ContextResult{}
	relSet := map[string]bool{}

	totalFull := 0
	for _, r := range selected {
		totalFull += estimateTokens(r.Memory.Content)
	}

	for i, r := range selected {
		mem := r.Memory
		text := pickDetail(mem, i, len(selected), totalFull, maxTokens)
		tokens := estimateTokens(text)
		result.Memories = append(result.Memories, mem)
		result.TotalTokens += tokens

		if includeRelations && e.relations != nil {
			rels, _ := e.relations.ListRelations(ctx, storage.RelationQuery{
				MemoryID:  mem.ID,
				Direction: "both",
			})
			for _, rel := range rels {
				if !relSet[rel.ID] {
					relSet[rel.ID] = true
					result.Relations = append(result.Relations, rel)
				}
			}
		}
	}

	return result, nil
}

// mmrSelect implements Maximal Marginal Relevance selection.
// lambda=1.0 → pure relevance, lambda=0.0 → pure diversity.
// Category match adds 0.3 to similarity penalty (same category = likely redundant).
func mmrSelect(candidates []*storage.SearchResult, tokenBudget int, lambda float64) []*storage.SearchResult {
	selected := make([]*storage.SearchResult, 0, len(candidates))
	remaining := make([]*storage.SearchResult, len(candidates))
	copy(remaining, candidates)

	// Pre-tokenise all candidates once
	tokenSets := make([]map[string]struct{}, len(candidates))
	for i, r := range candidates {
		tokenSets[i] = tokenSetFor(r.Memory.Content)
	}
	// Map memory ID → token set index for fast lookup
	idxByID := make(map[string]int, len(candidates))
	for i, r := range candidates {
		idxByID[r.Memory.ID] = i
	}

	usedTokens := 0

	for len(remaining) > 0 && usedTokens < tokenBudget {
		bestScore := -1.0
		bestIdx := 0

		for ri, r := range remaining {
			rel := r.HybridScore
			if rel == 0 {
				rel = r.TextScore
			}

			// Penalty: max similarity to any already-selected memory
			maxSim := 0.0
			for _, s := range selected {
				sim := util.JaccardSimilarity(tokenSets[idxByID[r.Memory.ID]], tokenSets[idxByID[s.Memory.ID]])
				if r.Memory.Category != "" && r.Memory.Category == s.Memory.Category {
					sim += 0.3
					if sim > 1.0 {
						sim = 1.0
					}
				}
				if sim > maxSim {
					maxSim = sim
				}
			}

			score := lambda*rel - (1-lambda)*maxSim
			if score > bestScore {
				bestScore = score
				bestIdx = ri
			}
		}

		pick := remaining[bestIdx]
		text := pick.Memory.Content
		if pick.Memory.Summary != "" {
			text = pick.Memory.Summary
		}
		usedTokens += estimateTokens(text)
		selected = append(selected, pick)
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

// pickDetail chooses the text detail level for a memory based on budget pressure.
//
//	totalFull <= budget        → full content for all
//	totalFull <= budget*2      → full for top 3, summary for rest
//	totalFull > budget*2       → summary for top 5, one-line for rest
func pickDetail(mem *domain.Memory, idx, total, totalFull, budget int) string {
	switch {
	case totalFull <= budget:
		return mem.Content
	case totalFull <= budget*2:
		if idx < 3 {
			return mem.Content
		}
		if mem.Summary != "" {
			return mem.Summary
		}
		return firstSentence(mem.Content)
	default:
		if idx < 5 {
			if mem.Summary != "" {
				return mem.Summary
			}
			return firstSentence(mem.Content)
		}
		return firstSentence(mem.Content)
	}
}

// firstSentence returns the first sentence of text (up to the first ". " or end).
func firstSentence(text string) string {
	if idx := strings.Index(text, ". "); idx != -1 {
		return text[:idx+1]
	}
	if len(text) > 120 {
		return text[:120]
	}
	return text
}

// tokenSetFor returns a token set for a content string.
func tokenSetFor(content string) map[string]struct{} {
	return util.TokenSet(util.Tokenize(content))
}

// estimateTokens approximates token count (1 token ≈ 4 chars)
func estimateTokens(text string) int {
	return len(text)/4 + 1
}
