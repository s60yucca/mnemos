package core

import (
	"context"
	"log/slog"
	"time"

	"github.com/mnemos-dev/mnemos/internal/core/lifecycle"
	coremem "github.com/mnemos-dev/mnemos/internal/core/memory"
	"github.com/mnemos-dev/mnemos/internal/core/relation"
	"github.com/mnemos-dev/mnemos/internal/core/search"
	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage"
)

// InitMode controls how Mnemos initializes its subsystems.
type InitMode int

const (
	// InitFull starts all background workers and health-checks the embedding provider.
	// Used by `mnemos serve` and `mnemos maintain`.
	InitFull InitMode = iota
	// InitLight opens storage and initializes engines but does NOT start background
	// workers (decay, GC, markdown sync) and does NOT health-check the embedding
	// provider (lazy init). Used by hook subcommands for fast cold-start.
	InitLight
	// InitReadOnly opens storage in read-only mode. Reserved for future use cases
	// that only need to query data without writing.
	InitReadOnly
)

// Mnemos is the main facade that wires all engines together
type Mnemos struct {
	memManager   *coremem.Manager
	searchEngine *search.SearchEngine
	relManager   *relation.Manager
	lifecycle    *lifecycle.Engine
	store        storage.IMemoryStore
	logger       *slog.Logger
}

// NewMnemos constructs the Mnemos facade with all dependencies.
// Background workers are NOT started — call Start() to begin them.
// This preserves the existing behavior where main.go calls Start() explicitly.
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

// NewMnemosWithMode constructs the Mnemos facade and applies the given InitMode:
//
//   - InitFull: starts all background workers and health-checks the embedding provider.
//   - InitLight: opens storage and initializes engines only — no background workers,
//     no embedding health-check (lazy init). Suitable for short-lived hook processes.
//   - InitReadOnly: same as InitLight; the caller is responsible for opening storage
//     in read-only mode before passing it in.
//
// embedProvider may be nil; if nil, the health-check step is skipped even for InitFull.
func NewMnemosWithMode(
	mode InitMode,
	memManager *coremem.Manager,
	searchEngine *search.SearchEngine,
	relManager *relation.Manager,
	lc *lifecycle.Engine,
	store storage.IMemoryStore,
	embedProvider embedding.IEmbeddingProvider,
	logger *slog.Logger,
) *Mnemos {
	m := &Mnemos{
		memManager:   memManager,
		searchEngine: searchEngine,
		relManager:   relManager,
		lifecycle:    lc,
		store:        store,
		logger:       logger,
	}

	switch mode {
	case InitFull:
		// Health-check embedding provider (non-fatal — log only)
		if embedProvider != nil {
			if !embedProvider.Healthy(context.Background()) {
				logger.Warn("embedding provider health-check failed", "provider", embedProvider.Name())
			}
		}
		// Start background workers (decay, GC)
		m.lifecycle.Start()
	case InitLight, InitReadOnly:
		// No background workers, no health-check — fast cold-start
	}

	return m
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

// SemanticSearch performs vector similarity search only
func (m *Mnemos) SemanticSearch(ctx context.Context, query, projectID string, limit int, minSim float64) ([]*storage.SearchResult, error) {
	return m.searchEngine.SemanticSearch(ctx, query, projectID, limit, minSim)
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

// CountMemoriesSince counts active memories for a project created at or after since.
func (m *Mnemos) CountMemoriesSince(ctx context.Context, projectID string, since time.Time) (int, error) {
	return m.store.CountMemoriesSince(ctx, projectID, since)
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
