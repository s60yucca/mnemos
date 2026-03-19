package search

import (
	"context"
	"log/slog"
	"sync"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage"
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
		return nil, nil
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

// AssembleContext builds a context bundle for a query within token budget
func (e *SearchEngine) AssembleContext(ctx context.Context, query, projectID string, maxTokens int, includeRelations bool) (*ContextResult, error) {
	results, err := e.HybridSearch(ctx, query, projectID, 20)
	if err != nil {
		return nil, err
	}

	result := &ContextResult{}
	tokenBudget := maxTokens
	relSet := map[string]bool{}

	for _, r := range results {
		mem := r.Memory
		// Use summary if available to save tokens
		text := mem.Content
		if mem.Summary != "" {
			text = mem.Summary
		}
		tokens := estimateTokens(text)
		if tokens > tokenBudget {
			break
		}
		tokenBudget -= tokens
		result.Memories = append(result.Memories, mem)
		result.TotalTokens += tokens

		// Expand via relations
		if includeRelations && e.relations != nil {
			rels, err := e.relations.ListRelations(ctx, storage.RelationQuery{
				MemoryID:  mem.ID,
				Direction: "both",
			})
			if err == nil {
				for _, rel := range rels {
					key := rel.ID
					if !relSet[key] {
						relSet[key] = true
						result.Relations = append(result.Relations, rel)
					}
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
