package memory

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/embedding"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// Manager handles memory CRUD with dedup, classification, and embedding
type Manager struct {
	store      storage.IMemoryStore
	embedStore storage.IEmbeddingStore
	embedder   embedding.IEmbeddingProvider
	classifier Classifier
	dedup      *ContentDedup
	mirror     storage.IMarkdownMirror
	embedQueue chan string
	embedWg    sync.WaitGroup
	logger     *slog.Logger
}

// NewManager creates a new memory Manager
func NewManager(
	store storage.IMemoryStore,
	embedStore storage.IEmbeddingStore,
	embedder embedding.IEmbeddingProvider,
	mirror storage.IMarkdownMirror,
	fuzzyThreshold, semanticThreshold float64,
	logger *slog.Logger,
) *Manager {
	m := &Manager{
		store:      store,
		embedStore: embedStore,
		embedder:   embedder,
		classifier: NewRuleClassifier(),
		dedup:      NewContentDedup(store, embedStore, fuzzyThreshold, semanticThreshold),
		mirror:     mirror,
		embedQueue: make(chan string, 1000),
		logger:     logger,
	}
	m.embedWg.Add(1)
	go m.processEmbedQueue()
	return m
}

// Store persists a memory through the full 9-step pipeline
func (m *Manager) Store(ctx context.Context, req *domain.StoreRequest) (*domain.StoreResult, error) {
	// 1. Validate
	if err := domain.ValidateStoreRequest(req); err != nil {
		return nil, err
	}

	// 2. Compute hash
	hash := util.ContentHash(req.Content, req.ProjectID)

	// 3. Dedup check
	existing, simType, score, err := m.dedup.Check(ctx, req, hash)
	if err != nil {
		return nil, fmt.Errorf("dedup check: %w", err)
	}
	if existing != nil {
		// Merge: append new content to existing
		if simType != "exact" {
			existing.Content = existing.Content + "\n\n---\n\n" + req.Content
			existing.ContentHash = util.ContentHash(existing.Content, existing.ProjectID)
			existing.UpdatedAt = time.Now().UTC()
			if err := m.store.Update(ctx, existing); err != nil {
				return nil, err
			}
		}
		m.logger.Debug("dedup hit", "id", existing.ID, "type", simType, "score", score)
		return &domain.StoreResult{Memory: existing, Created: false}, nil
	}

	// 4. Auto-classify
	memType := req.Type
	if memType == "" {
		memType = m.classifier.ClassifyType(req.Content, req.Tags)
	}
	category := req.Category
	if category == "" {
		category = m.classifier.ClassifyCategory(req.Content, req.Tags)
	}

	// 5. Build memory
	now := time.Now().UTC()
	mem := &domain.Memory{
		ID:             util.NewID(),
		Content:        req.Content,
		Summary:        req.Summary,
		Type:           memType,
		Category:       category,
		Tags:           req.Tags,
		Source:         req.Source,
		ProjectID:      req.ProjectID,
		Agent:          req.Agent,
		SessionID:      req.SessionID,
		Metadata:       req.Metadata,
		CreatedAt:      now,
		UpdatedAt:      now,
		LastAccessedAt: now,
		AccessCount:    0,
		RelevanceScore: 1.0,
		Status:         domain.MemoryStatusActive,
		ContentHash:    hash,
	}

	// 6. Persist
	if err := m.store.Create(ctx, mem); err != nil {
		return nil, fmt.Errorf("store create: %w", err)
	}

	// 7. Queue embedding (async, non-blocking)
	select {
	case m.embedQueue <- mem.ID:
	default:
		m.logger.Warn("embed queue full, skipping", "id", mem.ID)
	}

	// 8. Mirror to markdown (async)
	if m.mirror != nil && m.mirror.IsEnabled() {
		go func() {
			if err := m.mirror.SyncMemory(context.Background(), mem); err != nil {
				m.logger.Warn("markdown mirror failed", "id", mem.ID, "err", err)
			}
		}()
	}

	// 9. Return result
	return &domain.StoreResult{Memory: mem, Created: true}, nil
}

// Get retrieves a memory by ID and increments access count
func (m *Manager) Get(ctx context.Context, id string) (*domain.Memory, error) {
	mem, err := m.store.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	mem.TouchAccess()
	if err := m.store.Update(ctx, mem); err != nil {
		m.logger.Warn("touch access failed", "id", id, "err", err)
	}
	return mem, nil
}

// GetWithoutTouch retrieves a memory without updating access stats
func (m *Manager) GetWithoutTouch(ctx context.Context, id string) (*domain.Memory, error) {
	return m.store.GetByID(ctx, id)
}

// List returns memories matching the query
func (m *Manager) List(ctx context.Context, q storage.ListQuery) ([]*domain.Memory, error) {
	return m.store.List(ctx, q)
}

// Count returns the count of memories matching the query
func (m *Manager) Count(ctx context.Context, q storage.ListQuery) (int, error) {
	return m.store.Count(ctx, q)
}

// Update applies partial updates to a memory
func (m *Manager) Update(ctx context.Context, req *domain.UpdateRequest) (*domain.Memory, error) {
	if err := domain.ValidateUpdateRequest(req); err != nil {
		return nil, err
	}

	mem, err := m.store.GetByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	contentChanged := false
	if req.Content != nil && *req.Content != mem.Content {
		mem.Content = *req.Content
		mem.ContentHash = util.ContentHash(*req.Content, mem.ProjectID)
		contentChanged = true
	}
	if req.Summary != nil {
		mem.Summary = *req.Summary
	}
	if req.Type != nil {
		mem.Type = *req.Type
	} else if contentChanged {
		mem.Type = m.classifier.ClassifyType(mem.Content, mem.Tags)
	}
	if req.Category != nil {
		mem.Category = *req.Category
	} else if contentChanged {
		mem.Category = m.classifier.ClassifyCategory(mem.Content, mem.Tags)
	}
	if req.Tags != nil {
		mem.Tags = req.Tags
	}
	if req.Source != nil {
		mem.Source = *req.Source
	}
	if req.Metadata != nil {
		if mem.Metadata == nil {
			mem.Metadata = make(map[string]string)
		}
		for k, v := range req.Metadata {
			mem.Metadata[k] = v
		}
	}
	mem.UpdatedAt = time.Now().UTC()

	if err := m.store.Update(ctx, mem); err != nil {
		return nil, err
	}

	if contentChanged {
		select {
		case m.embedQueue <- mem.ID:
		default:
		}
	}

	return mem, nil
}

// Delete soft-deletes a memory
func (m *Manager) Delete(ctx context.Context, id string) error {
	return m.store.Delete(ctx, id)
}

// HardDelete permanently removes a memory and all its relations/embeddings
func (m *Manager) HardDelete(ctx context.Context, id string) error {
	if m.embedStore != nil {
		m.embedStore.DeleteEmbedding(ctx, id) //nolint:errcheck
	}
	if m.mirror != nil && m.mirror.IsEnabled() {
		m.mirror.DeleteMemory(ctx, id) //nolint:errcheck
	}
	return m.store.HardDelete(ctx, id)
}

// Stats returns storage statistics
func (m *Manager) Stats(ctx context.Context, projectID string) (*storage.Stats, error) {
	return m.store.Stats(ctx, projectID)
}

// processEmbedQueue processes embedding generation in the background
func (m *Manager) processEmbedQueue() {
	defer m.embedWg.Done()
	for id := range m.embedQueue {
		if m.embedder == nil || m.embedStore == nil {
			continue
		}
		ctx := context.Background()
		mem, err := m.store.GetByID(ctx, id)
		if err != nil {
			continue
		}
		text := mem.Content
		if mem.Summary != "" {
			text = mem.Summary + " " + mem.Content
		}
		// Truncate to avoid huge embeddings
		if len(text) > 8192 {
			text = text[:8192]
		}
		vec, err := m.embedder.Embed(ctx, text)
		if err != nil {
			m.logger.Warn("embed failed", "id", id, "err", err)
			continue
		}
		if err := m.embedStore.StoreEmbedding(ctx, id, vec); err != nil {
			m.logger.Warn("store embedding failed", "id", id, "err", err)
		}
	}
}

// Stop closes the embed queue channel and waits for all pending embeddings to complete
func (m *Manager) Stop() {
	close(m.embedQueue)
	m.embedWg.Wait()
}

// MergeContent merges new content into existing memory
func mergeContent(existing, newContent string) string {
	if strings.Contains(existing, newContent) {
		return existing
	}
	return existing + "\n\n---\n\n" + newContent
}
