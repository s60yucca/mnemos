package core

import (
	"context"
	"log/slog"

	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

// Mnemos is the main facade that wires all engines together
type Mnemos struct {
	memManager  *coremem.Manager
	searchEngine *search.SearchEngine
	relManager  *relation.Manager
	lifecycle   *lifecycle.Engine
	store       storage.IMemoryStore
	logger      *slog.Logger
}

// NewMnemos constructs the Mnemos facade with all dependencies
func NewMnemos(
	memManager *coremem.Manager,
	searchEngine *search.SearchEngine,
	relManager *relation.Manager,
	lifecycle *lifecycle.Engine,
	store storage.IMemoryStore,
	logger *slog.Logger,
) *Mnemos {
	return &Mnemos{
		memManager:   memManager,
		searchEngine: searchEngine,
		relManager:   relManager,
		lifecycle:    lifecycle,
		store:        store,
		logger:       logger,
	}
}

// Store persists a new memory
func (m *Mnemos) Store(ctx context.Context, req *domain.StoreRequest) (*domain.StoreResult, error) {
	return m.memManager.Store(ctx, req)
}

// Get retrieves a memory by ID (with access tracking)
func (m *Mnemos) Get(ctx context.Context, id string) (*domain.Memory, error) {
	return m.memManager.Get(ctx, id)
}

// Update applies partial updates to a memory
func (m *Mnemos) Update(ctx context.Context, req *domain.UpdateRequest) (*domain.Memory, error) {
	return m.memManager.Update(ctx, req)
}

// Delete soft-deletes a memory
func (m *Mnemos) Delete(ctx context.Context, id string) error {
	return m.memManager.Delete(ctx, id)
}

// HardDelete permanently removes a memory
func (m *Mnemos) HardDelete(ctx context.Context, id string) error {
	return m.memManager.HardDelete(ctx, id)
}

// List returns memories matching the query
func (m *Mnemos) List(ctx context.Context, q storage.ListQuery) ([]*domain.Memory, error) {
	return m.memManager.List(ctx, q)
}

// Search performs hybrid search
func (m *Mnemos) Search(ctx context.Context, query, projectID string, limit int) ([]*storage.SearchResult, error) {
	return m.searchEngine.HybridSearch(ctx, query, projectID, limit)
}

// TextSearch performs full-text search only
func (m *Mnemos) TextSearch(ctx context.Context, q storage.TextSearchQuery) ([]*storage.SearchResult, error) {
	return m.searchEngine.TextSearch(ctx, q)
}

// Relate creates a relation between two memories
func (m *Mnemos) Relate(ctx context.Context, req *domain.RelateRequest) (*domain.MemoryRelation, error) {
	return m.relManager.Relate(ctx, req)
}

// Traverse performs graph traversal from a memory
func (m *Mnemos) Traverse(ctx context.Context, q domain.GraphQuery) (*domain.GraphResult, error) {
	return m.relManager.Traverse(ctx, q)
}

// AssembleContext builds a context bundle for a query
func (m *Mnemos) AssembleContext(ctx context.Context, query, projectID string, maxTokens int, includeRelations bool) (*search.ContextResult, error) {
	return m.searchEngine.AssembleContext(ctx, query, projectID, maxTokens, includeRelations)
}

// Maintain runs decay, archival, and GC
func (m *Mnemos) Maintain(ctx context.Context, projectID string) error {
	if err := m.lifecycle.RunDecay(ctx, projectID); err != nil {
		return err
	}
	return m.lifecycle.RunGC(ctx, projectID)
}

// Stats returns storage statistics
func (m *Mnemos) Stats(ctx context.Context, projectID string) (*storage.Stats, error) {
	return m.memManager.Stats(ctx, projectID)
}

// Start begins background workers
func (m *Mnemos) Start() {
	m.lifecycle.Start()
}

// Shutdown gracefully stops all background workers
func (m *Mnemos) Shutdown() {
	m.lifecycle.Stop()
	m.memManager.Stop()
}
