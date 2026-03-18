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

// FTSSearcher implements storage.ITextSearcher using SQLite FTS5
type FTSSearcher struct {
	db *sql.DB
}

func NewFTSSearcher(db *sql.DB) *FTSSearcher {
	return &FTSSearcher{db: db}
}

func (f *FTSSearcher) Search(ctx context.Context, q storage.TextSearchQuery) ([]*storage.SearchResult, error) {
	if q.Query == "" {
		return nil, nil
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}

	// Sanitize query for FTS5 — strip special chars that cause syntax errors
	ftsQuery := sanitizeFTSQuery(q.Query)

	var joinConditions []string
	var extraArgs []any

	if q.ProjectID != "" {
		joinConditions = append(joinConditions, "m.project_id=?")
		extraArgs = append(extraArgs, q.ProjectID)
	}
	if len(q.Types) > 0 {
		ph := strings.Repeat("?,", len(q.Types))
		ph = ph[:len(ph)-1]
		joinConditions = append(joinConditions, fmt.Sprintf("m.type IN (%s)", ph))
		for _, t := range q.Types {
			extraArgs = append(extraArgs, string(t))
		}
	}
	if len(q.Statuses) > 0 {
		ph := strings.Repeat("?,", len(q.Statuses))
		ph = ph[:len(ph)-1]
		joinConditions = append(joinConditions, fmt.Sprintf("m.status IN (%s)", ph))
		for _, st := range q.Statuses {
			extraArgs = append(extraArgs, string(st))
		}
	}

	joinWhere := ""
	if len(joinConditions) > 0 {
		joinWhere = "AND " + strings.Join(joinConditions, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT m.id, m.content, m.summary, m.type, m.category,
		       m.tags, m.source, m.project_id, m.agent, m.session_id,
		       m.metadata, m.created_at, m.updated_at, m.last_accessed_at,
		       m.access_count, m.relevance_score, m.status, m.content_hash,
		       bm25(memories_fts) as score,
		       snippet(memories_fts, 1, '<b>', '</b>', '...', 32) as snippet
		FROM memories_fts
		JOIN memories m ON memories_fts.id = m.id
		WHERE memories_fts MATCH ?
		%s
		ORDER BY bm25(memories_fts)
		LIMIT ?`, joinWhere)

	args := []any{ftsQuery}
	args = append(args, extraArgs...)
	args = append(args, limit)

	rows, err := f.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	var results []*storage.SearchResult
	for rows.Next() {
		m, score, snippet, err := scanFTSRow(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, &storage.SearchResult{
			Memory:       m,
			TextScore:    score,
			MatchSnippet: snippet,
			Source:       "fts",
		})
	}
	return results, rows.Err()
}

func (f *FTSSearcher) IndexMemory(_ context.Context, _ *domain.Memory) error {
	return nil // handled by triggers
}

func (f *FTSSearcher) RemoveFromIndex(_ context.Context, _ string) error {
	return nil // handled by triggers
}

func (f *FTSSearcher) Reindex(ctx context.Context) error {
	_, err := f.db.ExecContext(ctx, `INSERT INTO memories_fts(memories_fts) VALUES('rebuild')`)
	return err
}

// sanitizeFTSQuery removes characters that cause FTS5 syntax errors and
// wraps each token in double-quotes so the query is treated as a phrase prefix search.
func sanitizeFTSQuery(q string) string {
	// Strip FTS5 special characters: " ( ) * : ^ -
	replacer := strings.NewReplacer(
		`"`, ` `,
		`(`, ` `,
		`)`, ` `,
		`*`, ` `,
		`:`, ` `,
		`^`, ` `,
		`-`, ` `,
		`/`, ` `,
		`\`, ` `,
	)
	cleaned := strings.TrimSpace(replacer.Replace(q))

	// Split into tokens and quote each one so FTS5 treats them as literals
	var tokens []string
	for _, tok := range strings.Fields(cleaned) {
		tok = strings.TrimSpace(tok)
		if tok != "" {
			tokens = append(tokens, `"`+tok+`"`)
		}
	}
	if len(tokens) == 0 {
		return `""`
	}
	return strings.Join(tokens, " ")
}

func scanFTSRow(rows *sql.Rows) (*domain.Memory, float64, string, error) {
	var m domain.Memory
	var tagsJSON, metaJSON string
	var createdAt, updatedAt, lastAccessedAt int64
	var mType, mStatus string
	var score float64
	var snippet string

	err := rows.Scan(
		&m.ID, &m.Content, &m.Summary, &mType, &m.Category,
		&tagsJSON, &m.Source, &m.ProjectID, &m.Agent, &m.SessionID,
		&metaJSON, &createdAt, &updatedAt, &lastAccessedAt,
		&m.AccessCount, &m.RelevanceScore, &mStatus, &m.ContentHash,
		&score, &snippet,
	)
	if err != nil {
		return nil, 0, "", err
	}

	m.Type = domain.MemoryType(mType)
	m.Status = domain.MemoryStatus(mStatus)
	m.CreatedAt = util.UnixNanoToTime(createdAt)
	m.UpdatedAt = util.UnixNanoToTime(updatedAt)
	m.LastAccessedAt = util.UnixNanoToTime(lastAccessedAt)
	json.Unmarshal([]byte(tagsJSON), &m.Tags)    //nolint:errcheck
	json.Unmarshal([]byte(metaJSON), &m.Metadata) //nolint:errcheck

	return &m, score, snippet, nil
}
