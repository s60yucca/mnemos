package storage

import (
	"context"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

// IMemoryStore defines CRUD operations for memories
type IMemoryStore interface {
	Create(ctx context.Context, m *domain.Memory) error
	GetByID(ctx context.Context, id string) (*domain.Memory, error)
	GetByHash(ctx context.Context, hash string) (*domain.Memory, error)
	List(ctx context.Context, q ListQuery) ([]*domain.Memory, error)
	Count(ctx context.Context, q ListQuery) (int, error)
	Update(ctx context.Context, m *domain.Memory) error
	Delete(ctx context.Context, id string) error   // soft delete
	HardDelete(ctx context.Context, id string) error
	BulkUpdateRelevance(ctx context.Context, items []BulkUpdateItem) error
	BulkUpdateStatus(ctx context.Context, ids []string, status domain.MemoryStatus) error
	ListForLifecycle(ctx context.Context, q LifecycleQuery) ([]*domain.Memory, error)
	Stats(ctx context.Context, projectID string) (*Stats, error)
	Close() error
	Ping(ctx context.Context) error
}

// ITextSearcher defines full-text search operations
type ITextSearcher interface {
	Search(ctx context.Context, q TextSearchQuery) ([]*SearchResult, error)
	IndexMemory(ctx context.Context, m *domain.Memory) error
	RemoveFromIndex(ctx context.Context, id string) error
	Reindex(ctx context.Context) error
}

// IEmbeddingStore defines vector storage and similarity search
type IEmbeddingStore interface {
	StoreEmbedding(ctx context.Context, memoryID string, vector []float32) error
	GetEmbedding(ctx context.Context, memoryID string) ([]float32, error)
	DeleteEmbedding(ctx context.Context, memoryID string) error
	Search(ctx context.Context, q SemanticSearchQuery) ([]*SearchResult, error)
	HasEmbedding(ctx context.Context, memoryID string) (bool, error)
	CountEmbeddings(ctx context.Context) (int, error)
	ListWithoutEmbeddings(ctx context.Context, limit int) ([]string, error)
}

// IRelationStore defines knowledge graph operations
type IRelationStore interface {
	CreateRelation(ctx context.Context, r *domain.MemoryRelation) error
	GetRelation(ctx context.Context, id string) (*domain.MemoryRelation, error)
	ListRelations(ctx context.Context, q RelationQuery) ([]*domain.MemoryRelation, error)
	GetRelationBetween(ctx context.Context, sourceID, targetID string, relType domain.RelationType) (*domain.MemoryRelation, error)
	UpdateRelation(ctx context.Context, r *domain.MemoryRelation) error
	DeleteRelation(ctx context.Context, id string) error
	DeleteRelationsForMemory(ctx context.Context, memoryID string) error
	Traverse(ctx context.Context, q domain.GraphQuery) (*domain.GraphResult, error)
	FindPath(ctx context.Context, fromID, toID string, maxDepth int) ([]*domain.Memory, error)
	CountRelations(ctx context.Context, memoryID string) (int, error)
}

// IMarkdownMirror defines markdown file synchronization
type IMarkdownMirror interface {
	SyncMemory(ctx context.Context, m *domain.Memory) error
	SyncRelation(ctx context.Context, r *domain.MemoryRelation) error
	DeleteMemory(ctx context.Context, id string) error
	SyncAll(ctx context.Context, memories []*domain.Memory) error
	GetBasePath() string
	IsEnabled() bool
}

// Store is the composite interface combining all storage operations
// Note: ITextSearcher and IEmbeddingStore both have Search methods with different signatures,
// so Store does not embed both — use them separately.
type Store interface {
	IMemoryStore
	IRelationStore
}
