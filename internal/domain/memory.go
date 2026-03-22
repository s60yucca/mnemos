package domain

import (
	"strings"
	"time"
)

// Memory is the core domain entity
type Memory struct {
	ID             string            `json:"id"`
	Content        string            `json:"content"`
	Summary        string            `json:"summary,omitempty"`
	Type           MemoryType        `json:"type"`
	Category       string            `json:"category"`
	Tags           []string          `json:"tags,omitempty"`
	Source         string            `json:"source,omitempty"`
	ProjectID      string            `json:"project_id,omitempty"`
	Agent          string            `json:"agent,omitempty"`
	SessionID      string            `json:"session_id,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	LastAccessedAt time.Time         `json:"last_accessed_at"`
	AccessCount    int               `json:"access_count"`
	RelevanceScore float64           `json:"relevance_score"`
	QualityScore   float64           `json:"quality_score,omitempty"`
	Status         MemoryStatus      `json:"status"`
	ContentHash    string            `json:"content_hash"`
}

// IsGlobal returns true if the memory has no project scope
func (m *Memory) IsGlobal() bool { return m.ProjectID == "" }

// HasTag returns true if the memory has the given tag
func (m *Memory) HasTag(tag string) bool {
	for _, t := range m.Tags {
		if strings.EqualFold(t, tag) {
			return true
		}
	}
	return false
}

// AddTag adds a tag if not already present
func (m *Memory) AddTag(tag string) {
	if !m.HasTag(tag) {
		m.Tags = append(m.Tags, tag)
	}
}

// TouchAccess increments access count and updates last accessed time
func (m *Memory) TouchAccess() {
	m.AccessCount++
	m.LastAccessedAt = time.Now().UTC()
}

// ContentPreview returns a truncated preview of the content
func (m *Memory) ContentPreview(maxLen int) string {
	if len(m.Content) <= maxLen {
		return m.Content
	}
	return m.Content[:maxLen] + "..."
}

// MemoryRelation represents a directed edge in the knowledge graph
type MemoryRelation struct {
	ID             string            `json:"id"`
	SourceMemoryID string            `json:"source_memory_id"`
	TargetMemoryID string            `json:"target_memory_id"`
	RelationType   RelationType      `json:"relation_type"`
	Strength       float64           `json:"strength"`
	Metadata       map[string]string `json:"metadata,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
}

// StoreRequest is the input for storing a memory
type StoreRequest struct {
	Content   string            `json:"content"`
	Summary   string            `json:"summary,omitempty"`
	Type      MemoryType        `json:"type,omitempty"`
	Category  string            `json:"category,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Source    string            `json:"source,omitempty"`
	ProjectID string            `json:"project_id,omitempty"`
	Agent     string            `json:"agent,omitempty"`
	SessionID string            `json:"session_id,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// UpdateRequest is the input for partial updates (PATCH semantics)
type UpdateRequest struct {
	ID       string
	Content  *string
	Summary  *string
	Type     *MemoryType
	Category *string
	Tags     []string
	Source   *string
	Metadata map[string]string
}

// StoreResult is the output of a Store operation
type StoreResult struct {
	Memory      *Memory `json:"memory"`
	Created     bool    `json:"created"`
	QualityNote string  `json:"quality_note,omitempty"`
}

// RelateRequest is the input for creating a relation
type RelateRequest struct {
	SourceID     string            `json:"source_id"`
	TargetID     string            `json:"target_id"`
	RelationType RelationType      `json:"relation_type"`
	Strength     float64           `json:"strength"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// GraphQuery defines a graph traversal request
type GraphQuery struct {
	StartID       string
	MaxDepth      int
	RelationTypes []RelationType
	MinStrength   float64
	Direction     string // "outgoing", "incoming", "both"
}

// GraphResult holds traversal results
type GraphResult struct {
	Memories  []*Memory         `json:"memories"`
	Relations []*MemoryRelation `json:"relations"`
}
