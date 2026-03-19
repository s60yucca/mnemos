package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mnemos-dev/mnemos/internal/domain"
	"github.com/mnemos-dev/mnemos/internal/storage"
	"github.com/mnemos-dev/mnemos/internal/util"
)

// SQLiteStore implements storage.IMemoryStore
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new SQLiteStore backed by the given db
func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Create(ctx context.Context, m *domain.Memory) error {
	tags, _ := json.Marshal(m.Tags)
	meta, _ := json.Marshal(m.Metadata)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO memories
		(id, content, summary, type, category, tags, source, project_id, agent, session_id,
		 metadata, created_at, updated_at, last_accessed_at, access_count, relevance_score, status, content_hash)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		m.ID, m.Content, m.Summary, string(m.Type), m.Category,
		string(tags), m.Source, m.ProjectID, m.Agent, m.SessionID,
		string(meta),
		util.TimeToUnixNano(m.CreatedAt),
		util.TimeToUnixNano(m.UpdatedAt),
		util.TimeToUnixNano(m.LastAccessedAt),
		m.AccessCount, m.RelevanceScore, string(m.Status), m.ContentHash,
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return domain.ErrDuplicate
		}
		return fmt.Errorf("create memory: %w", err)
	}
	return nil
}

func (s *SQLiteStore) GetByID(ctx context.Context, id string) (*domain.Memory, error) {
	row := s.db.QueryRowContext(ctx, `SELECT * FROM memories WHERE id = ?`, id)
	m, err := scanMemory(row)
	if err == sql.ErrNoRows {
		return nil, &domain.NotFoundError{ID: id}
	}
	return m, err
}

func (s *SQLiteStore) GetByHash(ctx context.Context, hash string) (*domain.Memory, error) {
	row := s.db.QueryRowContext(ctx, `SELECT * FROM memories WHERE content_hash = ? AND status = 'active'`, hash)
	m, err := scanMemory(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return m, err
}

func (s *SQLiteStore) List(ctx context.Context, q storage.ListQuery) ([]*domain.Memory, error) {
	query, args := buildListQuery(q, false)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list memories: %w", err)
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (s *SQLiteStore) Count(ctx context.Context, q storage.ListQuery) (int, error) {
	query, args := buildListQuery(q, true)
	var count int
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	return count, err
}

func (s *SQLiteStore) Update(ctx context.Context, m *domain.Memory) error {
	tags, _ := json.Marshal(m.Tags)
	meta, _ := json.Marshal(m.Metadata)
	res, err := s.db.ExecContext(ctx, `
		UPDATE memories SET
		content=?, summary=?, type=?, category=?, tags=?, source=?,
		metadata=?, updated_at=?, last_accessed_at=?, access_count=?,
		relevance_score=?, status=?, content_hash=?
		WHERE id=?`,
		m.Content, m.Summary, string(m.Type), m.Category, string(tags), m.Source,
		string(meta),
		util.TimeToUnixNano(m.UpdatedAt),
		util.TimeToUnixNano(m.LastAccessedAt),
		m.AccessCount, m.RelevanceScore, string(m.Status), m.ContentHash,
		m.ID,
	)
	if err != nil {
		return fmt.Errorf("update memory: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return &domain.NotFoundError{ID: m.ID}
	}
	return nil
}

func (s *SQLiteStore) Delete(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE memories SET status='deleted', updated_at=? WHERE id=? AND status != 'deleted'`,
		util.TimeToUnixNano(time.Now().UTC()), id,
	)
	if err != nil {
		return fmt.Errorf("soft delete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return &domain.NotFoundError{ID: id}
	}
	return nil
}

func (s *SQLiteStore) HardDelete(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM memories WHERE id=?`, id)
	return err
}

func (s *SQLiteStore) BulkUpdateRelevance(ctx context.Context, items []storage.BulkUpdateItem) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	stmt, err := tx.PrepareContext(ctx, `UPDATE memories SET relevance_score=?, updated_at=? WHERE id=?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	now := util.TimeToUnixNano(time.Now().UTC())
	for _, item := range items {
		if _, err := stmt.ExecContext(ctx, item.Score, now, item.ID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLiteStore) BulkUpdateStatus(ctx context.Context, ids []string, status domain.MemoryStatus) error {
	if len(ids) == 0 {
		return nil
	}
	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids)+2)
	args[0] = string(status)
	args[1] = util.TimeToUnixNano(time.Now().UTC())
	for i, id := range ids {
		args[i+2] = id
	}
	_, err := s.db.ExecContext(ctx,
		fmt.Sprintf(`UPDATE memories SET status=?, updated_at=? WHERE id IN (%s)`, placeholders),
		args...,
	)
	return err
}

func (s *SQLiteStore) ListForLifecycle(ctx context.Context, q storage.LifecycleQuery) ([]*domain.Memory, error) {
	var conditions []string
	var args []any

	if q.ProjectID != "" {
		conditions = append(conditions, "project_id=?")
		args = append(args, q.ProjectID)
	}
	if q.MaxRelevance > 0 {
		conditions = append(conditions, "relevance_score<=?")
		args = append(args, q.MaxRelevance)
	}
	if q.LastAccessBefore != nil {
		conditions = append(conditions, "last_accessed_at<?")
		args = append(args, util.TimeToUnixNano(*q.LastAccessBefore))
	}
	if len(q.Statuses) > 0 {
		ph := strings.Repeat("?,", len(q.Statuses))
		ph = ph[:len(ph)-1]
		conditions = append(conditions, fmt.Sprintf("status IN (%s)", ph))
		for _, st := range q.Statuses {
			args = append(args, string(st))
		}
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 1000
	}
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT * FROM memories %s ORDER BY relevance_score ASC LIMIT ?`, where),
		args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemories(rows)
}

func (s *SQLiteStore) Stats(ctx context.Context, projectID string) (*storage.Stats, error) {
	stats := &storage.Stats{
		ByType:     make(map[string]int),
		ByStatus:   make(map[string]int),
		ByCategory: make(map[string]int),
		ProjectID:  projectID,
	}

	where := ""
	var args []any
	if projectID != "" {
		where = "WHERE project_id=?"
		args = append(args, projectID)
	}

	row := s.db.QueryRowContext(ctx, fmt.Sprintf(`SELECT COUNT(*) FROM memories %s`, where), args...)
	row.Scan(&stats.Total) //nolint:errcheck

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT type, COUNT(*) FROM memories %s GROUP BY type`, where), args...)
	if err == nil {
		for rows.Next() {
			var t string
			var c int
			rows.Scan(&t, &c) //nolint:errcheck
			stats.ByType[t] = c
		}
		rows.Close()
	}

	rows2, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT status, COUNT(*) FROM memories %s GROUP BY status`, where), args...)
	if err == nil {
		for rows2.Next() {
			var st string
			var c int
			rows2.Scan(&st, &c) //nolint:errcheck
			stats.ByStatus[st] = c
		}
		rows2.Close()
	}

	rows3, err := s.db.QueryContext(ctx, fmt.Sprintf(`SELECT category, COUNT(*) FROM memories %s GROUP BY category`, where), args...)
	if err == nil {
		for rows3.Next() {
			var cat string
			var c int
			rows3.Scan(&cat, &c) //nolint:errcheck
			stats.ByCategory[cat] = c
		}
		rows3.Close()
	}

	return stats, nil
}

func (s *SQLiteStore) Close() error  { return s.db.Close() }
func (s *SQLiteStore) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// --- helpers ---

func buildListQuery(q storage.ListQuery, count bool) (string, []any) {
	var conditions []string
	var args []any

	if q.ProjectID != "" {
		conditions = append(conditions, "project_id=?")
		args = append(args, q.ProjectID)
	}
	if len(q.Types) > 0 {
		ph := strings.Repeat("?,", len(q.Types))
		ph = ph[:len(ph)-1]
		conditions = append(conditions, fmt.Sprintf("type IN (%s)", ph))
		for _, t := range q.Types {
			args = append(args, string(t))
		}
	}
	if len(q.Statuses) > 0 {
		ph := strings.Repeat("?,", len(q.Statuses))
		ph = ph[:len(ph)-1]
		conditions = append(conditions, fmt.Sprintf("status IN (%s)", ph))
		for _, st := range q.Statuses {
			args = append(args, string(st))
		}
	}
	if len(q.Categories) > 0 {
		ph := strings.Repeat("?,", len(q.Categories))
		ph = ph[:len(ph)-1]
		conditions = append(conditions, fmt.Sprintf("category IN (%s)", ph))
		for _, c := range q.Categories {
			args = append(args, c)
		}
	}
	if q.Agent != "" {
		conditions = append(conditions, "agent=?")
		args = append(args, q.Agent)
	}
	if q.MinRelevance > 0 {
		conditions = append(conditions, "relevance_score>=?")
		args = append(args, q.MinRelevance)
	}
	if q.MaxRelevance > 0 {
		conditions = append(conditions, "relevance_score<=?")
		args = append(args, q.MaxRelevance)
	}
	if q.CreatedAfter != nil {
		conditions = append(conditions, "created_at>=?")
		args = append(args, util.TimeToUnixNano(*q.CreatedAfter))
	}
	if q.CreatedBefore != nil {
		conditions = append(conditions, "created_at<=?")
		args = append(args, util.TimeToUnixNano(*q.CreatedBefore))
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	if count {
		return fmt.Sprintf(`SELECT COUNT(*) FROM memories %s`, where), args
	}

	sortBy := q.SortBy
	allowedSortCols := map[string]bool{
		"created_at": true, "updated_at": true,
		"last_accessed_at": true, "relevance_score": true,
	}
	if !allowedSortCols[sortBy] {
		sortBy = "created_at"
	}
	dir := "ASC"
	if q.SortDesc {
		dir = "DESC"
	}
	limit := q.Limit
	if limit <= 0 {
		limit = 50
	}
	args = append(args, limit, q.Offset)
	return fmt.Sprintf(`SELECT * FROM memories %s ORDER BY %s %s LIMIT ? OFFSET ?`, where, sortBy, dir), args
}

type scanner interface {
	Scan(dest ...any) error
}

func scanMemory(row scanner) (*domain.Memory, error) {
	var m domain.Memory
	var tagsJSON, metaJSON string
	var createdAt, updatedAt, lastAccessedAt int64
	var mType, mStatus string

	err := row.Scan(
		&m.ID, &m.Content, &m.Summary, &mType, &m.Category,
		&tagsJSON, &m.Source, &m.ProjectID, &m.Agent, &m.SessionID,
		&metaJSON, &createdAt, &updatedAt, &lastAccessedAt,
		&m.AccessCount, &m.RelevanceScore, &mStatus, &m.ContentHash,
	)
	if err != nil {
		return nil, err
	}

	m.Type = domain.MemoryType(mType)
	m.Status = domain.MemoryStatus(mStatus)
	m.CreatedAt = util.UnixNanoToTime(createdAt)
	m.UpdatedAt = util.UnixNanoToTime(updatedAt)
	m.LastAccessedAt = util.UnixNanoToTime(lastAccessedAt)

	json.Unmarshal([]byte(tagsJSON), &m.Tags)   //nolint:errcheck
	json.Unmarshal([]byte(metaJSON), &m.Metadata) //nolint:errcheck

	return &m, nil
}

func scanMemories(rows *sql.Rows) ([]*domain.Memory, error) {
	var memories []*domain.Memory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		memories = append(memories, m)
	}
	return memories, rows.Err()
}
