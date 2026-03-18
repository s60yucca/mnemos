package sqlite

import (
	"context"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// EmbeddingStore implements storage.IEmbeddingStore using SQLite BLOBs
type EmbeddingStore struct {
	db *sql.DB
}

func NewEmbeddingStore(db *sql.DB) *EmbeddingStore {
	return &EmbeddingStore{db: db}
}

func (e *EmbeddingStore) StoreEmbedding(ctx context.Context, memoryID string, vector []float32) error {
	blob := float32SliceToBlob(vector)
	now := util.TimeToUnixNano(util.NowUTC())
	_, err := e.db.ExecContext(ctx,
		`INSERT INTO memory_embeddings(memory_id, vector, created_at) VALUES(?,?,?)
		 ON CONFLICT(memory_id) DO UPDATE SET vector=excluded.vector`,
		memoryID, blob, now,
	)
	return err
}

func (e *EmbeddingStore) GetEmbedding(ctx context.Context, memoryID string) ([]float32, error) {
	var blob []byte
	err := e.db.QueryRowContext(ctx, `SELECT vector FROM memory_embeddings WHERE memory_id=?`, memoryID).Scan(&blob)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return blobToFloat32Slice(blob), nil
}

func (e *EmbeddingStore) DeleteEmbedding(ctx context.Context, memoryID string) error {
	_, err := e.db.ExecContext(ctx, `DELETE FROM memory_embeddings WHERE memory_id=?`, memoryID)
	return err
}

func (e *EmbeddingStore) Search(ctx context.Context, q storage.SemanticSearchQuery) ([]*storage.SearchResult, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	minSim := q.MinSimilarity
	if minSim <= 0 {
		minSim = 0.5
	}

	// Load all embeddings for the project (brute-force cosine)
	var rows *sql.Rows
	var err error
	if q.ProjectID != "" {
		rows, err = e.db.QueryContext(ctx,
			`SELECT e.memory_id, e.vector FROM memory_embeddings e
			 JOIN memories m ON e.memory_id = m.id
			 WHERE m.project_id=? AND m.status='active'`, q.ProjectID)
	} else {
		rows, err = e.db.QueryContext(ctx,
			`SELECT e.memory_id, e.vector FROM memory_embeddings e
			 JOIN memories m ON e.memory_id = m.id
			 WHERE m.status='active'`)
	}
	if err != nil {
		return nil, fmt.Errorf("semantic search load: %w", err)
	}
	defer rows.Close()

	var candidates []embCandidate

	for rows.Next() {
		var memID string
		var blob []byte
		if err := rows.Scan(&memID, &blob); err != nil {
			continue
		}
		vec := blobToFloat32Slice(blob)
		score := util.CosineSimilarity(q.Vector, vec)
		if score >= minSim {
			candidates = append(candidates, embCandidate{id: memID, score: score})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Sort by score descending
	sortCandidates(candidates)
	if len(candidates) > limit {
		candidates = candidates[:limit]
	}

	// Fetch full memory objects
	var results []*storage.SearchResult
	for _, c := range candidates {
		row := e.db.QueryRowContext(ctx, `SELECT * FROM memories WHERE id=?`, c.id)
		m, err := scanMemory(row)
		if err != nil {
			continue
		}
		results = append(results, &storage.SearchResult{
			Memory:        m,
			SemanticScore: c.score,
			Source:        "semantic",
		})
	}
	return results, nil
}

func (e *EmbeddingStore) HasEmbedding(ctx context.Context, memoryID string) (bool, error) {
	var count int
	err := e.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_embeddings WHERE memory_id=?`, memoryID).Scan(&count)
	return count > 0, err
}

func (e *EmbeddingStore) CountEmbeddings(ctx context.Context) (int, error) {
	var count int
	err := e.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memory_embeddings`).Scan(&count)
	return count, err
}

func (e *EmbeddingStore) ListWithoutEmbeddings(ctx context.Context, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := e.db.QueryContext(ctx,
		`SELECT m.id FROM memories m
		 LEFT JOIN memory_embeddings e ON m.id = e.memory_id
		 WHERE e.memory_id IS NULL AND m.status='active'
		 LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		rows.Scan(&id) //nolint:errcheck
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// --- binary encoding helpers ---

func float32SliceToBlob(v []float32) []byte {
	buf := make([]byte, len(v)*4)
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(f))
	}
	return buf
}

func blobToFloat32Slice(b []byte) []float32 {
	if len(b)%4 != 0 {
		return nil
	}
	v := make([]float32, len(b)/4)
	for i := range v {
		v[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return v
}

type embCandidate struct {
	id    string
	score float64
}

func sortCandidates(c []embCandidate) {
	// simple insertion sort (small N)
	for i := 1; i < len(c); i++ {
		key := c[i]
		j := i - 1
		for j >= 0 && c[j].score < key.score {
			c[j+1] = c[j]
			j--
		}
		c[j+1] = key
	}
}
