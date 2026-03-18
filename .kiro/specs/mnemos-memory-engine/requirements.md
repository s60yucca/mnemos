# Requirements: Mnemos Memory Engine

## Introduction

Mnemos là một unified memory engine cho AI coding agents, viết bằng Go. Hệ thống cho phép AI agents (Claude Code, Cursor, v.v.) lưu trữ và truy xuất context xuyên suốt các sessions, giải quyết vấn đề mất context giữa các cuộc hội thoại.

## Requirements

### Requirement 1: Memory Storage (Core CRUD)

**User Story**: As an AI coding agent, I want to store memories with content, type, category, and tags so that I can persist important context across sessions.

#### Acceptance Criteria

1.1 The system SHALL accept a `StoreRequest` with required `content` field (1 byte to 100KB, valid UTF-8) and optional `type`, `category`, `tags`, `project_id`, `agent`, `session_id`, `trigger` fields.

1.2 The system SHALL auto-classify `type` (short_term | long_term | episodic | semantic) using keyword rules, structural analysis, and tag boosts when `type` is not provided.

1.3 The system SHALL auto-classify `category` (one of 13 built-in categories or custom) using keyword rules and tag matching when `category` is not provided.

1.4 The system SHALL generate a ULID as the memory ID, ensuring global uniqueness and lexicographic sortability by creation time.

1.5 The system SHALL compute a SHA-256 hash of normalized content and store it as `content_hash` for deduplication.

1.6 The system SHALL set initial `relevance_score` to 1.0, `status` to `active`, `access_count` to 0, and all timestamps to current UTC time.

1.7 The system SHALL return a `StoreResult` containing the persisted `Memory` object and a `Created` boolean indicating whether a new memory was created or a duplicate was detected.

1.8 The system SHALL support retrieving a memory by ID via `Get(id)`, returning `ErrNotFound` if the memory does not exist.

1.9 The system SHALL support partial updates via `UpdateRequest` (PATCH semantics): only non-nil fields are applied.

1.10 The system SHALL support soft-delete via `Delete(id)`, setting `status` to `deleted` without removing the record from storage.

1.11 The system SHALL support hard-delete via `HardDelete(id)`, permanently removing the memory and all associated relations and embeddings.

1.12 The system SHALL support listing memories with filters: `project_id`, `types`, `statuses`, `categories`, `tags` (AND), `anyTags` (OR), `agent`, time ranges, relevance range, with sorting and pagination (limit/offset and cursor-based).

### Requirement 2: Content Deduplication

**User Story**: As an AI agent, I want the system to detect and merge duplicate memories so that the knowledge base stays clean and non-redundant.

#### Acceptance Criteria

2.1 The system SHALL implement a 3-tier deduplication pipeline: Tier 1 (exact SHA-256 hash match), Tier 2 (Jaccard similarity on word token sets), Tier 3 (cosine similarity on embedding vectors).

2.2 The system SHALL short-circuit on the first tier that finds a match, returning the existing memory with `Created: false`.

2.3 For Tier 1 (exact match): the system SHALL perform an O(1) indexed lookup by `content_hash` and return the existing memory if found and `status = active`.

2.4 For Tier 2 (fuzzy match): the system SHALL load up to 200 recent active memories, compute Jaccard similarity on word token sets (stop-words removed), and return the best match if its score >= configurable threshold (default 0.85).

2.5 For Tier 3 (semantic match): the system SHALL use the embedding store to find memories with cosine similarity >= configurable threshold (default 0.92), only when an embedding store is available.

2.6 When a fuzzy or semantic duplicate is detected, the system SHALL merge the new content into the existing memory by appending with a separator, recompute the hash, and update the record.

2.7 The system SHALL expose a `FindDuplicates(projectID, threshold, limit)` function for batch duplicate analysis without automatic merging.

### Requirement 3: Full-Text Search

**User Story**: As an AI agent, I want to search memories by keyword so that I can quickly find relevant context.

#### Acceptance Criteria

3.1 The system SHALL implement full-text search using SQLite FTS5 virtual tables with BM25 ranking.

3.2 The system SHALL index `content`, `summary`, `tags`, and `category` fields in the FTS5 index.

3.3 The system SHALL support FTS5 query syntax: simple words, phrases (quoted), boolean operators (AND, OR, NOT), and prefix matching (word*).

3.4 The system SHALL return results with a `TextScore` (BM25 rank), `MatchSnippet` (highlighted excerpt), and the full `Memory` object.

3.5 The system SHALL support filtering text search results by `project_id`, `types`, `categories`, `tags`, and `statuses`.

3.6 Text search for 10,000 memories SHALL complete in under 100ms.

3.7 The system SHALL keep the FTS5 index synchronized with the main `memories` table: index on create, update on update, remove on delete.

### Requirement 4: Semantic Search

**User Story**: As an AI agent, I want to search memories by meaning so that I can find relevant context even when exact keywords don't match.

#### Acceptance Criteria

4.1 The system SHALL support semantic search using embedding vectors stored as BLOBs in SQLite (serialized `[]float32`).

4.2 The system SHALL support at least two embedding providers: Ollama (local HTTP) and OpenAI (API HTTP), both implemented using only `net/http` stdlib.

4.3 The system SHALL include a Noop embedding provider that returns zero vectors, enabling Level 0-1 operation without any embedding service.

4.4 The system SHALL compute cosine similarity between the query vector and all stored vectors, returning results above a configurable `MinSimilarity` threshold (default 0.5).

4.5 Semantic search for 10,000 memories SHALL complete in under 500ms.

4.6 The system SHALL support async embedding generation: after storing a memory, the embedding is queued and generated in a background goroutine without blocking the Store response.

4.7 The system SHALL expose `ListWithoutEmbeddings(limit)` to identify memories that need embedding generation (for backfill).

### Requirement 5: Hybrid Search with RRF

**User Story**: As an AI agent, I want to combine text and semantic search results for the best possible retrieval quality.

#### Acceptance Criteria

5.1 The system SHALL implement hybrid search by running text search and semantic search in parallel (using goroutines) and fusing results using Reciprocal Rank Fusion (RRF).

5.2 The RRF formula SHALL be: `score(d) = Σ 1/(k + rank_i(d))` where `k=60` (standard constant) and `rank_i` is the 1-based position in each result list.

5.3 The system SHALL return a `HybridScore` for each result in the fused list.

5.4 Hybrid search SHALL fall back gracefully to text-only search when no embedding provider is configured.

5.5 Hybrid search for 10,000 memories SHALL complete in under 600ms.

### Requirement 6: Context Assembly

**User Story**: As an AI agent, I want to assemble a relevant context bundle for a given topic so that I can efficiently load session context within token limits.

#### Acceptance Criteria

6.1 The system SHALL implement `AssembleContext(query, project_id, max_tokens, include_relations)` that returns a curated set of memories relevant to the query.

6.2 The system SHALL use hybrid search to find the most relevant memories, then expand the result set by traversing relations (if `include_relations = true`).

6.3 The system SHALL respect the `max_tokens` limit by prioritizing memories with higher `HybridScore` and truncating lower-priority memories.

6.4 The system SHALL return a `ContextResult` containing the selected memories, their relations, and the total estimated token count.

6.5 The system SHALL use the memory `Summary` field (when available) instead of full `Content` to reduce token usage.

### Requirement 7: Memory Relations (Knowledge Graph)

**User Story**: As an AI agent, I want to create and traverse relationships between memories so that I can understand how pieces of knowledge connect.

#### Acceptance Criteria

7.1 The system SHALL support 7 relation types: `relates_to`, `depends_on`, `contradicts`, `supersedes`, `derived_from`, `part_of`, `caused_by`.

7.2 The system SHALL store relations as directed edges with a `strength` value in [0.0, 1.0] and optional `metadata` map.

7.3 The system SHALL prevent duplicate relations: the combination of `(source_id, target_id, relation_type)` must be unique.

7.4 The system SHALL support BFS graph traversal via `Traverse(GraphQuery)` with configurable `MaxDepth` (default 2), relation type filters, and minimum strength filter.

7.5 The system SHALL support finding the shortest path between two memories via `FindPath(fromID, toID, maxDepth)`.

7.6 The system SHALL cascade-delete all relations when a memory is hard-deleted.

7.7 The system SHALL implement auto-detection of implicit relations: when a new memory is stored, the system scans recent memories for potential `relates_to` or `depends_on` relationships based on content similarity and keyword overlap.

### Requirement 8: Memory Lifecycle (Decay & GC)

**User Story**: As a developer, I want memories to automatically decay in relevance over time so that the knowledge base stays fresh and relevant.

#### Acceptance Criteria

8.1 The system SHALL implement exponential decay: `score = max(floor, base * e^(-λ*t) * accessBoost * typeMultiplier)` where `t` is hours since last access, `λ` is type-specific decay rate, `accessBoost = 1.0 + log(1 + accessCount) * 0.1`, and `floor = 0.05`.

8.2 The decay rates SHALL be: `short_term=0.1` (decays in ~1 day), `long_term=0.005` (~6 months), `episodic=0.02` (~1 month), `semantic=0.001` (nearly permanent).

8.3 The system SHALL run decay as a background goroutine on a configurable ticker (default every 24 hours).

8.4 The system SHALL archive memories with `relevance_score < 0.1` by setting `status = archived`.

8.5 The system SHALL hard-delete memories with `status = deleted` that are older than a configurable retention period (default 30 days) via a GC process.

8.6 The system SHALL support manual promotion of a memory (reset `relevance_score` to 1.0 and update `last_accessed_at`).

8.7 The system SHALL support `BulkUpdateRelevance([]BulkUpdateItem)` for efficient batch score updates in a single transaction.

### Requirement 9: Markdown Mirror

**User Story**: As a developer, I want all memories mirrored as human-readable markdown files so that I can inspect, edit, and git-track my knowledge base.

#### Acceptance Criteria

9.1 The system SHALL write each memory to a markdown file at path `<base_dir>/<project_id|global>/<category>/<id>.md`.

9.2 The markdown file SHALL include: YAML frontmatter (id, type, category, tags, source, created_at, relevance_score, status) and the memory content as the body.

9.3 The system SHALL write markdown files asynchronously (non-blocking) after SQLite persistence.

9.4 The system SHALL delete the markdown file when a memory is hard-deleted.

9.5 The system SHALL support `SyncAll(memories)` for full re-sync (initial setup or recovery).

9.6 The markdown mirror SHALL be disableable via config (`mirror.enabled = false`).

### Requirement 10: MCP Server

**User Story**: As an AI coding agent (Claude Code, Cursor), I want to interact with Mnemos via the MCP protocol so that I can store and retrieve memories during coding sessions.

#### Acceptance Criteria

10.1 The system SHALL implement an MCP server supporting stdio transport (primary) and SSE transport (optional).

10.2 The system SHALL expose 8 MCP tools: `mnemos_store`, `mnemos_search`, `mnemos_get`, `mnemos_update`, `mnemos_delete`, `mnemos_relate`, `mnemos_context`, `mnemos_maintain`.

10.3 The system SHALL expose 2 MCP resources: `mnemos://memories/{project_id}` (list active memories) and `mnemos://stats` (storage statistics).

10.4 The system SHALL expose 2 MCP prompts: `load_context` (load relevant context at session start) and `save_session` (save important learnings at session end).

10.5 All MCP tool inputs SHALL be validated with descriptive error messages returned as MCP error responses.

10.6 The MCP server SHALL start in under 50ms.

10.7 The system SHALL support running as `mnemos serve --mcp` for stdio MCP mode.

### Requirement 11: CLI

**User Story**: As a developer, I want a CLI to manage memories directly from the terminal so that I can inspect, search, and maintain my knowledge base without an AI agent.

#### Acceptance Criteria

11.1 The system SHALL implement a CLI using `cobra` with the following commands: `init`, `serve`, `store`, `search`, `get`, `list`, `delete`, `relate`, `stats`, `maintain`, `config`.

11.2 `mnemos init` SHALL create the data directory, SQLite database, and default config file.

11.3 `mnemos serve` SHALL start the MCP server (stdio) or REST server (`--rest` flag).

11.4 `mnemos store <content>` SHALL store a memory with optional `--type`, `--category`, `--tags`, `--project` flags.

11.5 `mnemos search <query>` SHALL search memories with optional `--mode` (text|semantic|hybrid), `--project`, `--limit` flags and display results in a table.

11.6 `mnemos list` SHALL list memories with filter flags and display in a table with truncated content preview.

11.7 `mnemos stats` SHALL display storage statistics (total memories, by type, by category, DB size).

11.8 `mnemos maintain` SHALL run decay, archival, and GC and display a summary of changes.

### Requirement 12: REST API

**User Story**: As a tool builder, I want a REST API so that I can integrate Mnemos with any tool or language.

#### Acceptance Criteria

12.1 The system SHALL implement a REST API using `net/http` stdlib with JSON request/response.

12.2 The REST API SHALL expose endpoints: `POST /memories`, `GET /memories/{id}`, `PATCH /memories/{id}`, `DELETE /memories/{id}`, `GET /memories`, `POST /memories/search`, `POST /memories/{id}/relate`, `GET /stats`, `POST /maintain`.

12.3 All REST endpoints SHALL return appropriate HTTP status codes: 200 OK, 201 Created, 400 Bad Request, 404 Not Found, 409 Conflict, 500 Internal Server Error.

12.4 The REST server SHALL be startable via `mnemos serve --rest --port 8080`.

12.5 The REST server SHALL support CORS headers for browser-based clients.

### Requirement 13: Configuration

**User Story**: As a developer, I want to configure Mnemos via a config file and environment variables so that I can customize behavior for different environments.

#### Acceptance Criteria

13.1 The system SHALL load configuration from `~/.mnemos/config.yaml` (global) and `.mnemos/config.yaml` (project-local, takes precedence) using `viper`.

13.2 The system SHALL support environment variable overrides with prefix `MNEMOS_` (e.g., `MNEMOS_EMBEDDINGS_PROVIDER=ollama`).

13.3 The config SHALL support: `data_dir`, `log_level`, `embeddings.enabled`, `embeddings.provider`, `embeddings.model`, `embeddings.api_key`, `embeddings.base_url`, `mirror.enabled`, `mirror.base_dir`, `lifecycle.decay_interval`, `lifecycle.gc_retention_days`, `dedup.fuzzy_threshold`, `dedup.semantic_threshold`.

13.4 The system SHALL work with zero configuration using sensible defaults (SQLite in `~/.mnemos/`, markdown mirror enabled, embeddings disabled, decay every 24h).

### Requirement 14: Distribution & Installation

**User Story**: As a developer, I want to install Mnemos with a single command so that I can start using it in under 30 seconds.

#### Acceptance Criteria

14.1 The system SHALL be distributed as a single static binary with zero runtime dependencies.

14.2 The system SHALL provide pre-built binaries for: `darwin-arm64`, `darwin-amd64`, `linux-amd64`, `linux-arm64`, `windows-amd64`.

14.3 The binary size SHALL be under 25MB.

14.4 The system SHALL be installable via: install script (`curl | sh`), Homebrew (`brew install`), and `go install`.

14.5 The system SHALL use `goreleaser` for automated multi-platform builds and GitHub Releases.

14.6 The system SHALL be buildable from source with a single `go build` command (no CGO, no external tools required).

### Requirement 15: Testing & Quality

**User Story**: As a developer, I want comprehensive tests so that I can confidently modify and extend the system.

#### Acceptance Criteria

15.1 The system SHALL have unit tests for all core engine components (MemoryManager, SearchEngine, RelationManager, LifecycleEngine, RuleClassifier, ContentDedup) using mock storage interfaces.

15.2 The system SHALL have property-based tests using `pgregory.net/rapid` for: Store round-trip consistency, dedup idempotency, Jaccard/cosine similarity mathematical properties, and decay monotonicity.

15.3 The system SHALL have integration tests using real SQLite in-memory database covering: full Store → Search → Get round-trip, MCP tool invocation, lifecycle pipeline, and markdown mirror sync.

15.4 The system SHALL achieve at least 80% code coverage on core engine packages.

15.5 All code SHALL pass `go vet` and `golangci-lint` with no errors.
