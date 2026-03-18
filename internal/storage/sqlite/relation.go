package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// RelationStore implements storage.IRelationStore
type RelationStore struct {
	db *sql.DB
}

func NewRelationStore(db *sql.DB) *RelationStore {
	return &RelationStore{db: db}
}

func (r *RelationStore) CreateRelation(ctx context.Context, rel *domain.MemoryRelation) error {
	meta, _ := json.Marshal(rel.Metadata)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO memory_relations(id, source_id, target_id, relation_type, strength, metadata, created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		rel.ID, rel.SourceMemoryID, rel.TargetMemoryID, string(rel.RelationType),
		rel.Strength, string(meta), util.TimeToUnixNano(rel.CreatedAt),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return domain.ErrConflict
		}
		return fmt.Errorf("create relation: %w", err)
	}
	return nil
}

func (r *RelationStore) GetRelation(ctx context.Context, id string) (*domain.MemoryRelation, error) {
	row := r.db.QueryRowContext(ctx, `SELECT * FROM memory_relations WHERE id=?`, id)
	return scanRelation(row)
}

func (r *RelationStore) ListRelations(ctx context.Context, q storage.RelationQuery) ([]*domain.MemoryRelation, error) {
	var conditions []string
	var args []any

	switch q.Direction {
	case "outgoing":
		conditions = append(conditions, "source_id=?")
		args = append(args, q.MemoryID)
	case "incoming":
		conditions = append(conditions, "target_id=?")
		args = append(args, q.MemoryID)
	default:
		conditions = append(conditions, "(source_id=? OR target_id=?)")
		args = append(args, q.MemoryID, q.MemoryID)
	}

	if len(q.RelationTypes) > 0 {
		ph := strings.Repeat("?,", len(q.RelationTypes))
		ph = ph[:len(ph)-1]
		conditions = append(conditions, fmt.Sprintf("relation_type IN (%s)", ph))
		for _, rt := range q.RelationTypes {
			args = append(args, string(rt))
		}
	}
	if q.MinStrength > 0 {
		conditions = append(conditions, "strength>=?")
		args = append(args, q.MinStrength)
	}

	where := strings.Join(conditions, " AND ")
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT * FROM memory_relations WHERE %s ORDER BY created_at DESC`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanRelations(rows)
}

func (r *RelationStore) GetRelationBetween(ctx context.Context, sourceID, targetID string, relType domain.RelationType) (*domain.MemoryRelation, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT * FROM memory_relations WHERE source_id=? AND target_id=? AND relation_type=?`,
		sourceID, targetID, string(relType),
	)
	rel, err := scanRelation(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return rel, err
}

func (r *RelationStore) UpdateRelation(ctx context.Context, rel *domain.MemoryRelation) error {
	meta, _ := json.Marshal(rel.Metadata)
	_, err := r.db.ExecContext(ctx,
		`UPDATE memory_relations SET strength=?, metadata=? WHERE id=?`,
		rel.Strength, string(meta), rel.ID,
	)
	return err
}

func (r *RelationStore) DeleteRelation(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM memory_relations WHERE id=?`, id)
	return err
}

func (r *RelationStore) DeleteRelationsForMemory(ctx context.Context, memoryID string) error {
	_, err := r.db.ExecContext(ctx,
		`DELETE FROM memory_relations WHERE source_id=? OR target_id=?`,
		memoryID, memoryID,
	)
	return err
}

// Traverse performs BFS graph traversal
func (r *RelationStore) Traverse(ctx context.Context, q domain.GraphQuery) (*domain.GraphResult, error) {
	maxDepth := q.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 2
	}

	visited := map[string]bool{q.StartID: true}
	queue := []string{q.StartID}
	var allRelations []*domain.MemoryRelation
	var allIDs []string

	for depth := 0; depth < maxDepth && len(queue) > 0; depth++ {
		var nextQueue []string
		for _, id := range queue {
			rq := storage.RelationQuery{
				MemoryID:      id,
				RelationTypes: q.RelationTypes,
				MinStrength:   q.MinStrength,
				Direction:     q.Direction,
			}
			rels, err := r.ListRelations(ctx, rq)
			if err != nil {
				return nil, err
			}
			for _, rel := range rels {
				allRelations = append(allRelations, rel)
				neighbor := rel.TargetMemoryID
				if rel.SourceMemoryID != id {
					neighbor = rel.SourceMemoryID
				}
				if !visited[neighbor] {
					visited[neighbor] = true
					nextQueue = append(nextQueue, neighbor)
					allIDs = append(allIDs, neighbor)
				}
			}
		}
		queue = nextQueue
	}

	// Fetch memory objects for all visited IDs
	var memories []*domain.Memory
	for _, id := range allIDs {
		row := r.db.QueryRowContext(ctx, `SELECT * FROM memories WHERE id=?`, id)
		m, err := scanMemory(row)
		if err == nil {
			memories = append(memories, m)
		}
	}

	return &domain.GraphResult{Memories: memories, Relations: allRelations}, nil
}

func (r *RelationStore) FindPath(ctx context.Context, fromID, toID string, maxDepth int) ([]*domain.Memory, error) {
	if maxDepth <= 0 {
		maxDepth = 5
	}

	// BFS to find shortest path
	type node struct {
		id   string
		path []string
	}
	visited := map[string]bool{fromID: true}
	queue := []node{{id: fromID, path: []string{fromID}}}

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr.id == toID {
			var memories []*domain.Memory
			for _, id := range curr.path {
				row := r.db.QueryRowContext(ctx, `SELECT * FROM memories WHERE id=?`, id)
				m, err := scanMemory(row)
				if err == nil {
					memories = append(memories, m)
				}
			}
			return memories, nil
		}

		if len(curr.path) >= maxDepth {
			continue
		}

		rows, err := r.db.QueryContext(ctx,
			`SELECT target_id FROM memory_relations WHERE source_id=?`, curr.id)
		if err != nil {
			continue
		}
		for rows.Next() {
			var neighborID string
			rows.Scan(&neighborID) //nolint:errcheck
			if !visited[neighborID] {
				visited[neighborID] = true
				newPath := make([]string, len(curr.path)+1)
				copy(newPath, curr.path)
				newPath[len(curr.path)] = neighborID
				queue = append(queue, node{id: neighborID, path: newPath})
			}
		}
		rows.Close()
	}
	return nil, nil // no path found
}

func (r *RelationStore) CountRelations(ctx context.Context, memoryID string) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM memory_relations WHERE source_id=? OR target_id=?`,
		memoryID, memoryID,
	).Scan(&count)
	return count, err
}

// --- helpers ---

func scanRelation(row scanner) (*domain.MemoryRelation, error) {
	var rel domain.MemoryRelation
	var metaJSON string
	var createdAt int64
	var relType string

	err := row.Scan(
		&rel.ID, &rel.SourceMemoryID, &rel.TargetMemoryID,
		&relType, &rel.Strength, &metaJSON, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	rel.RelationType = domain.RelationType(relType)
	rel.CreatedAt = util.UnixNanoToTime(createdAt)
	json.Unmarshal([]byte(metaJSON), &rel.Metadata) //nolint:errcheck
	return &rel, nil
}

func scanRelations(rows *sql.Rows) ([]*domain.MemoryRelation, error) {
	var rels []*domain.MemoryRelation
	for rows.Next() {
		rel, err := scanRelation(rows)
		if err != nil {
			return nil, err
		}
		rels = append(rels, rel)
	}
	return rels, rows.Err()
}
