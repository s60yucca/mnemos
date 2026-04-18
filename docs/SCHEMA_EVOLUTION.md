# Database Schema Evolution Strategy

## Problem Statement

SQLite's `ALTER TABLE ADD COLUMN` always adds columns at the END of the table, regardless of where they appear in the base schema. This creates two different column orders:

1. **Fresh databases**: Columns in the order defined in `schema.go`
2. **Migrated databases**: Original columns + new columns appended at the end

This causes `sql.Scan()` errors when code expects one order but the database has another.

## Solution: Explicit Column Lists

We use **explicit column lists** in all SELECT queries, ordered to match the **base schema** (not the migrated order). This ensures:

1. Consistent column order across all databases
2. No dependency on physical table structure
3. Future-proof against new columns

### Implementation

```go
// Define explicit column list matching base schema order
const selectMemories = `SELECT id, content, summary, type, category, tags, source, project_id, agent, session_id, metadata, created_at, updated_at, last_accessed_at, access_count, relevance_score, quality_score, status, content_hash FROM memories`

// Scan in the same order as the SELECT
func scanMemory(row scanner) (*domain.Memory, error) {
    err := row.Scan(
        &m.ID, &m.Content, &m.Summary, &mType, &m.Category,
        &tagsJSON, &m.Source, &m.ProjectID, &m.Agent, &m.SessionID,
        &metaJSON, &createdAt, &updatedAt, &lastAccessedAt,
        &m.AccessCount, &m.RelevanceScore, &m.QualityScore, &mStatus, &m.ContentHash,
    )
    // ...
}
```

## Adding New Columns

When adding a new column to the `memories` table:

1. **Add to base schema** in `schema.go` at the desired position
2. **Create migration** using `ALTER TABLE ADD COLUMN` (will append at end)
3. **Update `selectMemories`** constant to include the new column in base schema order
4. **Update `scanMemory`** to scan the new column in the same order
5. **Update `scanMemoryByName`** (for sql.Rows) to handle the new column
6. **Update FTS queries** if the column should be searchable

### Example: Adding `embedding_model` column

```go
// 1. Add to base schema (schema.go)
CREATE TABLE IF NOT EXISTS memories (
    id              TEXT PRIMARY KEY,
    content         TEXT NOT NULL,
    // ... other columns ...
    quality_score   REAL NOT NULL DEFAULT 1.0,
    embedding_model TEXT NOT NULL DEFAULT '',  // NEW COLUMN HERE
    status          TEXT NOT NULL DEFAULT 'active',
    content_hash    TEXT NOT NULL UNIQUE
);

// 2. Create migration (schema.go)
{version: 3, sql: `ALTER TABLE memories ADD COLUMN embedding_model TEXT DEFAULT ''`},

// 3. Update selectMemories (store.go)
const selectMemories = `SELECT id, content, ..., quality_score, embedding_model, status, content_hash FROM memories`

// 4. Update scanMemory (store.go)
err := row.Scan(
    &m.ID, &m.Content, ..., &m.QualityScore, &m.EmbeddingModel, &mStatus, &m.ContentHash,
)

// 5. Update scanMemoryByName (store.go)
m := &domain.Memory{
    // ...
    EmbeddingModel: getString("embedding_model"),
}
```

## Why Not Use Column Name Mapping?

Go's `database/sql` package doesn't support scanning by column name for `sql.Row` (single row queries). While we could use column name mapping for `sql.Rows` (multiple rows), it would create inconsistency and add runtime overhead.

The explicit column list approach is:
- **Simpler**: One scanning strategy for all queries
- **Faster**: No runtime column name lookups
- **Safer**: Compile-time verification of column count
- **Standard**: Common practice in Go database code

## Testing Strategy

1. **Unit tests**: Test with fresh databases (base schema order)
2. **Integration tests**: Test with migrated databases (ALTER TABLE order)
3. **Property tests**: Verify scan order matches SELECT order

## References

- SQLite ALTER TABLE documentation: https://www.sqlite.org/lang_altertable.html
- Go database/sql best practices: https://go.dev/doc/database/querying
