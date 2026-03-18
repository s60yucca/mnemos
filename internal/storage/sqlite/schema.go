package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=-64000;
PRAGMA foreign_keys=ON;
PRAGMA temp_store=MEMORY;

CREATE TABLE IF NOT EXISTS memories (
    id              TEXT PRIMARY KEY,
    content         TEXT NOT NULL,
    summary         TEXT NOT NULL DEFAULT '',
    type            TEXT NOT NULL DEFAULT 'episodic',
    category        TEXT NOT NULL DEFAULT 'general',
    tags            TEXT NOT NULL DEFAULT '[]',
    source          TEXT NOT NULL DEFAULT '',
    project_id      TEXT NOT NULL DEFAULT '',
    agent           TEXT NOT NULL DEFAULT '',
    session_id      TEXT NOT NULL DEFAULT '',
    metadata        TEXT NOT NULL DEFAULT '{}',
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL,
    last_accessed_at INTEGER NOT NULL,
    access_count    INTEGER NOT NULL DEFAULT 0,
    relevance_score REAL NOT NULL DEFAULT 1.0,
    status          TEXT NOT NULL DEFAULT 'active',
    content_hash    TEXT NOT NULL UNIQUE
);

CREATE INDEX IF NOT EXISTS idx_memories_project_id ON memories(project_id);
CREATE INDEX IF NOT EXISTS idx_memories_status ON memories(status);
CREATE INDEX IF NOT EXISTS idx_memories_type ON memories(type);
CREATE INDEX IF NOT EXISTS idx_memories_category ON memories(category);
CREATE INDEX IF NOT EXISTS idx_memories_relevance ON memories(relevance_score);
CREATE INDEX IF NOT EXISTS idx_memories_created_at ON memories(created_at);
CREATE INDEX IF NOT EXISTS idx_memories_content_hash ON memories(content_hash);

CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    id UNINDEXED,
    content,
    summary,
    tags,
    category,
    content='memories',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS memories_fts_insert AFTER INSERT ON memories BEGIN
    INSERT INTO memories_fts(rowid, id, content, summary, tags, category)
    VALUES (new.rowid, new.id, new.content, new.summary, new.tags, new.category);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_update AFTER UPDATE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, id, content, summary, tags, category)
    VALUES ('delete', old.rowid, old.id, old.content, old.summary, old.tags, old.category);
    INSERT INTO memories_fts(rowid, id, content, summary, tags, category)
    VALUES (new.rowid, new.id, new.content, new.summary, new.tags, new.category);
END;

CREATE TRIGGER IF NOT EXISTS memories_fts_delete AFTER DELETE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, id, content, summary, tags, category)
    VALUES ('delete', old.rowid, old.id, old.content, old.summary, old.tags, old.category);
END;

CREATE TABLE IF NOT EXISTS memory_relations (
    id              TEXT PRIMARY KEY,
    source_id       TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    target_id       TEXT NOT NULL REFERENCES memories(id) ON DELETE CASCADE,
    relation_type   TEXT NOT NULL,
    strength        REAL NOT NULL DEFAULT 1.0,
    metadata        TEXT NOT NULL DEFAULT '{}',
    created_at      INTEGER NOT NULL,
    UNIQUE(source_id, target_id, relation_type)
);

CREATE INDEX IF NOT EXISTS idx_relations_source ON memory_relations(source_id);
CREATE INDEX IF NOT EXISTS idx_relations_target ON memory_relations(target_id);

CREATE TABLE IF NOT EXISTS memory_embeddings (
    memory_id   TEXT PRIMARY KEY REFERENCES memories(id) ON DELETE CASCADE,
    vector      BLOB NOT NULL,
    created_at  INTEGER NOT NULL
);
`

// Open opens (or creates) a SQLite database at the given path and applies the schema
func Open(path string) (*sql.DB, error) {
	dsn := path
	if path == ":memory:" {
		dsn = "file::memory:?cache=shared&_busy_timeout=5000"
	} else {
		dsn = fmt.Sprintf("file:%s?_busy_timeout=5000", path)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite WAL supports 1 writer
	db.SetMaxIdleConns(1)

	if err := applySchema(db); err != nil {
		db.Close()
		return nil, err
	}
	if err := runMigrations(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func applySchema(db *sql.DB) error {
	_, err := db.ExecContext(context.Background(), schema)
	return err
}

// migration represents a single schema migration step.
// Add new entries to the migrations slice — never edit existing ones.
type migration struct {
	version int
	sql     string
}

// migrations is the ordered list of schema migrations.
// Rules:
//   - Never modify or delete an existing migration.
//   - Always append new migrations at the end.
//   - version must be sequential starting from 1.
//   - Keep each migration idempotent where possible (use IF NOT EXISTS / IF EXISTS).
var migrations = []migration{
	// v1: baseline — tables already created by applySchema above, nothing extra needed.
	{version: 1, sql: `SELECT 1`},
	// Add future migrations here, e.g.:
	// {version: 2, sql: `ALTER TABLE memories ADD COLUMN new_col TEXT NOT NULL DEFAULT ''`},
}

// runMigrations applies any pending migrations using PRAGMA user_version as the version counter.
func runMigrations(db *sql.DB) error {
	ctx := context.Background()

	var current int
	if err := db.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&current); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if _, err := db.ExecContext(ctx, m.sql); err != nil {
			return fmt.Errorf("migration v%d: %w", m.version, err)
		}
		// PRAGMA user_version cannot use ? placeholders — format directly (safe: integer only)
		if _, err := db.ExecContext(ctx, fmt.Sprintf(`PRAGMA user_version = %d`, m.version)); err != nil {
			return fmt.Errorf("set schema version v%d: %w", m.version, err)
		}
	}
	return nil
}
