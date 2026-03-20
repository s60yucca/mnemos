package storage

import (
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
)

// SortField constants for ordering results
const (
	SortByCreatedAt      = "created_at"
	SortByUpdatedAt      = "updated_at"
	SortByLastAccessedAt = "last_accessed_at"
	SortByRelevanceScore = "relevance_score"
	SortByAccessCount    = "access_count"
)

// ListQuery defines filtering and pagination for listing memories
type ListQuery struct {
	ProjectID     string
	Types         []domain.MemoryType
	Statuses      []domain.MemoryStatus
	Categories    []string
	Tags          []string // AND — memory must have all tags
	AnyTags       []string // OR — memory must have at least one tag
	Agent         string
	SessionID     string
	CreatedAfter  *time.Time
	CreatedBefore *time.Time
	MinRelevance  float64
	MaxRelevance  float64
	SortBy        string
	SortDesc      bool
	Limit         int
	Offset        int
	Cursor        string // cursor-based pagination
}

// TextSearchQuery defines a full-text search request
type TextSearchQuery struct {
	Query      string
	ProjectID  string
	Types      []domain.MemoryType
	Categories []string
	Tags       []string
	Statuses   []domain.MemoryStatus
	Limit      int
}

// SemanticSearchQuery defines a vector similarity search request
type SemanticSearchQuery struct {
	Vector        []float32
	ProjectID     string
	MinSimilarity float64
	Limit         int
}

// RelationQuery defines a relation lookup request
type RelationQuery struct {
	MemoryID      string
	RelationTypes []domain.RelationType
	Direction     string // "outgoing", "incoming", "both"
	MinStrength   float64
}

// LifecycleQuery defines a query for lifecycle operations
type LifecycleQuery struct {
	ProjectID        string
	MaxRelevance     float64
	LastAccessBefore *time.Time
	UpdatedBefore    *time.Time
	Statuses         []domain.MemoryStatus
	Limit            int
}

// SearchResult wraps a memory with its search score
type SearchResult struct {
	Memory        *domain.Memory `json:"memory"`
	TextScore     float64        `json:"text_score,omitempty"`
	SemanticScore float64        `json:"semantic_score,omitempty"`
	HybridScore   float64        `json:"hybrid_score,omitempty"`
	MatchSnippet  string         `json:"match_snippet,omitempty"`
	Source        string         `json:"source"` // "fts", "semantic", "hybrid"
}

// BulkUpdateItem is a single item for bulk relevance updates
type BulkUpdateItem struct {
	ID    string
	Score float64
}

// Stats holds aggregate storage statistics
type Stats struct {
	Total       int            `json:"total"`
	ByType      map[string]int `json:"by_type"`
	ByStatus    map[string]int `json:"by_status"`
	ByCategory  map[string]int `json:"by_category"`
	ProjectID   string         `json:"project_id,omitempty"`
	DBSizeBytes int64          `json:"db_size_bytes,omitempty"`
}
