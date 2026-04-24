package search

import (
	"context"
	"log/slog"
	"sync"

	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

// ContextResult holds assembled context for a query
type ContextResult struct {
	Memories    []*domain.Memory         `json:"memories"`
	Relations   []*domain.MemoryRelation `json:"relations"`
	TotalTokens int                      `json:"total_tokens"`
	Message     string                   `json:"message,omitempty"` // Optional helpful message for cold start UX
}

// SearchEngine handles text, semantic, and hybrid search
type SearchEngine struct {
	fts        storage.ITextSearcher
	embedStore storage.IEmbeddingStore
	embedder   embedding.IEmbeddingProvider
	relations  storage.IRelationStore
	logger     *slog.Logger
	assembler  *ContextAssembler
	fileBoost  float64
}

func NewSearchEngine(
	fts storage.ITextSearcher,
	embedStore storage.IEmbeddingStore,
	embedder embedding.IEmbeddingProvider,
	relations storage.IRelationStore,
	logger *slog.Logger,
	lambda float64,
	fileBoost float64,
) *SearchEngine {
	return &SearchEngine{
		fts:        fts,
		embedStore: embedStore,
		embedder:   embedder,
		relations:  relations,
		logger:     logger,
		assembler:  NewContextAssembler(lambda),
		fileBoost:  fileBoost,
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
//  2. MMR diversity filter — balances relevance vs. redundancy
//  3. Adaptive packing — chooses full/summary/one-line detail level by budget
func (e *SearchEngine) AssembleContext(ctx context.Context, query, projectID string, maxTokens int, includeRelations bool, openFiles []string) (*ContextResult, error) {
	// DEBUG: Log incoming parameters
	e.logger.Info("AssembleContext called",
		"query", query,
		"project_id", projectID,
		"max_tokens", maxTokens)

	candidates, err := e.HybridSearch(ctx, query, projectID, 20)
	if err != nil {
		e.logger.Error("HybridSearch failed", "error", err, "query", query, "project_id", projectID)
		return nil, err
	}

	// DEBUG: Log candidate count
	e.logger.Info("HybridSearch completed",
		"query", query,
		"project_id", projectID,
		"candidate_count", len(candidates))

	if len(candidates) == 0 {
		return &ContextResult{}, nil
	}

	// Extract memories from search results, propagating the hybrid score as RelevanceScore.
	memories := make([]*domain.Memory, len(candidates))
	for i, r := range candidates {
		mem := r.Memory
		score := r.HybridScore
		if score == 0 {
			score = r.TextScore
		}
		mem.RelevanceScore = score
		memories[i] = mem
	}

	// Apply file-aware boost (no-op when openFiles is nil/empty)
	if len(openFiles) > 0 {
		coremem.ApplyFileBoost(memories, openFiles, e.fileBoost)
	}

	// Stage 2: MMR diversity filter
	selected := e.assembler.DiversityFilter(memories, maxTokens)

	// Stage 3: Adaptive packing
	result := &ContextResult{
		Memories: selected,
	}

	contextStr := e.assembler.PackWithBudget(selected, maxTokens)
	result.TotalTokens = estimateTokens(contextStr)

	// Relation expansion (unchanged)
	if includeRelations && e.relations != nil {
		relSet := map[string]bool{}
		for _, mem := range selected {
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

// estimateTokens approximates token count (1 token ≈ 4 chars)
func estimateTokens(text string) int {
	return len(text)/4 + 1
}
