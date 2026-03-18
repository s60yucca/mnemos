package relation

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// Manager handles memory relations
type Manager struct {
	store  storage.IRelationStore
	memory storage.IMemoryStore
	logger *slog.Logger
}

func NewManager(store storage.IRelationStore, memory storage.IMemoryStore, logger *slog.Logger) *Manager {
	return &Manager{store: store, memory: memory, logger: logger}
}

// Relate creates a directed relation between two memories
func (m *Manager) Relate(ctx context.Context, req *domain.RelateRequest) (*domain.MemoryRelation, error) {
	if req.SourceID == "" || req.TargetID == "" {
		return nil, &domain.ValidationError{Field: "source_id/target_id", Message: "both IDs required"}
	}
	if !req.RelationType.IsValid() {
		return nil, &domain.ValidationError{Field: "relation_type", Message: "invalid relation type"}
	}
	if req.Strength < 0 || req.Strength > 1 {
		return nil, &domain.ValidationError{Field: "strength", Message: "strength must be in [0.0, 1.0]"}
	}
	if req.Strength == 0 {
		req.Strength = 1.0
	}

	// Check for existing relation
	existing, err := m.store.GetRelationBetween(ctx, req.SourceID, req.TargetID, req.RelationType)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil // idempotent
	}

	rel := &domain.MemoryRelation{
		ID:             util.NewID(),
		SourceMemoryID: req.SourceID,
		TargetMemoryID: req.TargetID,
		RelationType:   req.RelationType,
		Strength:       req.Strength,
		Metadata:       req.Metadata,
		CreatedAt:      time.Now().UTC(),
	}

	if err := m.store.CreateRelation(ctx, rel); err != nil {
		return nil, fmt.Errorf("create relation: %w", err)
	}
	return rel, nil
}

// Unrelate removes a relation by ID
func (m *Manager) Unrelate(ctx context.Context, relationID string) error {
	return m.store.DeleteRelation(ctx, relationID)
}

// Traverse performs BFS graph traversal from a starting memory
func (m *Manager) Traverse(ctx context.Context, q domain.GraphQuery) (*domain.GraphResult, error) {
	return m.store.Traverse(ctx, q)
}

// FindPath finds the shortest path between two memories
func (m *Manager) FindPath(ctx context.Context, fromID, toID string, maxDepth int) ([]*domain.Memory, error) {
	return m.store.FindPath(ctx, fromID, toID, maxDepth)
}

// AutoDetect scans recent memories for implicit relations based on content similarity
func (m *Manager) AutoDetect(ctx context.Context, newMemory *domain.Memory, recentLimit int) error {
	if recentLimit <= 0 {
		recentLimit = 20
	}

	recent, err := m.memory.List(ctx, storage.ListQuery{
		ProjectID: newMemory.ProjectID,
		Statuses:  []domain.MemoryStatus{domain.MemoryStatusActive},
		Limit:     recentLimit,
		SortBy:    "created_at",
		SortDesc:  true,
	})
	if err != nil {
		return err
	}

	tokA := util.TokenSet(util.Tokenize(newMemory.Content))

	for _, mem := range recent {
		if mem.ID == newMemory.ID {
			continue
		}
		tokB := util.TokenSet(util.Tokenize(mem.Content))
		score := util.JaccardSimilarity(tokA, tokB)

		var relType domain.RelationType
		switch {
		case score >= 0.7:
			relType = domain.RelationTypeRelatesTo
		case score >= 0.5:
			relType = domain.RelationTypeDependsOn
		default:
			continue
		}

		_, err := m.Relate(ctx, &domain.RelateRequest{
			SourceID:     newMemory.ID,
			TargetID:     mem.ID,
			RelationType: relType,
			Strength:     score,
		})
		if err != nil {
			m.logger.Debug("auto-detect relation failed", "err", err)
		}
	}
	return nil
}
