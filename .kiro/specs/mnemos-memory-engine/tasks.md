# Tasks: Mnemos Memory Engine

## Task List

- [-] 1 Project Scaffolding & Module Setup
  - [x] 1.1 Initialize Go module `github.com/mnemos-dev/mnemos` with go.mod (Go 1.23)
  - [ ] 1.2 Add all required dependencies: modernc.org/sqlite, mark3labs/mcp-go, spf13/cobra, spf13/viper, oklog/ulid/v2, stretchr/testify, pgregory.net/rapid
  - [x] 1.3 Create directory structure: cmd/mnemos, internal/domain, internal/storage, internal/core, internal/embedding, internal/transport, internal/config, internal/util
  - [-] 1.4 Create Makefile with targets: build, test, lint, release, clean
  - [ ] 1.5 Create .goreleaser.yml for multi-platform binary distribution

- [-] 2 Domain Layer (internal/domain)
  - [x] 2.1 Implement internal/domain/types.go: MemoryType, MemoryStatus, RelationType, Trigger, Category constants with IsValid(), String(), DefaultDecayRate() methods
  - [x] 2.2 Implement internal/domain/memory.go: Memory struct, MemorySource, StoreRequest, UpdateRequest, StoreResult with helper methods (IsGlobal, HasTag, AddTag, TouchAccess, ContentPreview)
  - [x] 2.3 Implement internal/domain/relation.go: MemoryRelation, RelateRequest, GraphQuery, GraphResult with helper methods
  - [x] 2.4 Implement internal/domain/errors.go: sentinel errors (ErrNotFound, ErrDuplicate, ErrValidation, ErrConflict, ErrStorageUnavailable, ErrEmbeddingUnavailable) and typed errors (NotFoundError, ValidationError, ValidationErrors, DuplicateError)
  - [x] 2.5 Implement internal/domain/validation.go: ValidateStoreRequest, ValidateUpdateRequest with all field constraints

- [-] 3 Utility Layer (internal/util)
  - [x] 3.1 Implement internal/util/id.go: NewID() using oklog/ulid/v2 with monotonic entropy
  - [x] 3.2 Implement internal/util/hash.go: ContentHash(content string) string using crypto/sha256, NormalizeContent(content string) string
  - [x] 3.3 Implement internal/util/logger.go: NewLogger(level, format) *slog.Logger with JSON and text output formats
  - [x] 3.4 Implement internal/util/timeutil.go: NowUTC(), TimeToUnixNano(), UnixNanoToTime() helpers

- [-] 4 Storage Interfaces & Query Types (internal/storage)
  - [x] 4.1 Implement internal/storage/query.go: ListQuery, TextSearchQuery, SemanticSearchQuery, RelationQuery, LifecycleQuery, SearchResult, BulkUpdateItem, Stats, SortField constants
  - [x] 4.2 Implement internal/storage/interfaces.go: IMemoryStore (13 methods), ITextSearcher (4 methods), IEmbeddingStore (7 methods), IRelationStore (9 methods), IMarkdownMirror (6 methods), Store composite interface

- [-] 5 SQLite Storage Adapter (internal/storage/sqlite)
  - [ ] 5.1 Implement internal/storage/sqlite/schema.go: SQL schema for memories, memories_fts, memory_relations, memory_embeddings tables; migration runner; PRAGMA configuration (WAL, synchronous=NORMAL, cache_size, foreign_keys)
  - [ ] 5.2 Implement internal/storage/sqlite/store.go: SQLiteStore struct implementing IMemoryStore — Create, GetByID, GetByHash, List, Count, Update, Delete, HardDelete, BulkUpdateRelevance, BulkUpdateStatus, ListForLifecycle, Stats, Close, Ping
  - [ ] 5.3 Implement internal/storage/sqlite/fts.go: FTS5 implementation of ITextSearcher — Search (BM25 ranking + snippet), IndexMemory, RemoveFromIndex, Reindex
  - [ ] 5.4 Implement internal/storage/sqlite/embedding.go: IEmbeddingStore implementation — StoreEmbedding (float32 slice → BLOB), GetEmbedding, DeleteEmbedding, Search (brute-force cosine), HasEmbedding, CountEmbeddings, ListWithoutEmbeddings
  - [ ] 5.5 Implement internal/storage/sqlite/relation.go: IRelationStore implementation — CreateRelation, GetRelation, ListRelations, GetRelationBetween, UpdateRelation, DeleteRelation, DeleteRelationsForMemory, Traverse (BFS), FindPath, CountRelations
  - [ ] 5.6 Write integration tests for SQLite adapter using in-memory SQLite (file::memory:?cache=shared)

- [-] 6 Markdown Mirror Adapter (internal/storage/markdown)
  - [ ] 6.1 Implement internal/storage/markdown/mirror.go: MarkdownMirror implementing IMarkdownMirror — SyncMemory (YAML frontmatter + content body), SyncRelation, DeleteMemory, SyncAll, GetBasePath, SetEnabled, IsEnabled
  - [ ] 6.2 Write unit tests for markdown mirror with temp directory

- [-] 7 Embedding Providers (internal/embedding)
  - [ ] 7.1 Implement internal/embedding/provider.go: IEmbeddingProvider interface with Name(), Dimensions(), Embed(), EmbedBatch(), Healthy() methods
  - [ ] 7.2 Implement internal/embedding/noop.go: NoopProvider returning zero vectors (Level 0-1 default)
  - [ ] 7.3 Implement internal/embedding/ollama.go: OllamaProvider using net/http to call Ollama /api/embeddings endpoint; configurable base_url and model
  - [ ] 7.4 Implement internal/embedding/openai.go: OpenAIProvider using net/http to call OpenAI /v1/embeddings endpoint; configurable api_key and model
  - [ ] 7.5 Write unit tests for embedding providers with HTTP mock server

- [-] 8 Core Engine — Memory Manager (internal/core/memory)
  - [ ] 8.1 Implement internal/core/memory/options.go: ManagerConfig struct with all tunable parameters and DefaultManagerConfig(); functional Option pattern (WithDedup, WithAutoClassify, WithLogger, etc.)
  - [ ] 8.2 Implement internal/core/memory/classifier.go: RuleClassifier implementing Classifier interface — ClassifyType (keyword rules + structural analysis + tag boosts + tie-breaking), ClassifyCategory (keyword rules + tag matching), Tokenize, TokenSet, isStopWord
  - [ ] 8.3 Implement internal/core/memory/dedup.go: ContentDedup implementing Deduplicator interface — CheckExact (Tier 1 hash), CheckFuzzy (Tier 2 Jaccard + Tier 3 semantic), JaccardSimilarity, CosineSimilarity, ShingleSimilarity, CombinedSimilarity, FindDuplicates
  - [ ] 8.4 Implement internal/core/memory/manager.go: Manager struct — Store (full 9-step pipeline), Get (with TouchAccess), GetWithoutTouch, List, Count, Update (with re-hash/re-classify on content change), Delete (soft), HardDelete, Stats
  - [ ] 8.5 Write unit tests for RuleClassifier with diverse content samples
  - [ ] 8.6 Write unit tests for ContentDedup (exact, fuzzy, semantic tiers)
  - [ ] 8.7 Write unit tests for MemoryManager with mock storage interfaces
  - [ ] 8.8 Write property-based tests: Store round-trip, dedup idempotency, Jaccard/cosine mathematical properties

- [ ] 9 Core Engine — Search Engine (internal/core/search)
  - [ ] 9.1 Implement internal/core/search/rrf.go: ReciprocRankFusion(textResults, semanticResults []storage.SearchResult, k float64) []storage.SearchResult
  - [ ] 9.2 Implement internal/core/search/engine.go: SearchEngine struct — TextSearch, SemanticSearch (embed query → vector search), HybridSearch (parallel text+semantic → RRF), AssembleContext (hybrid search + relation expansion + token budget)
  - [ ] 9.3 Write unit tests for RRF with known rank lists
  - [ ] 9.4 Write unit tests for SearchEngine with mock ITextSearcher and IEmbeddingStore

- [ ] 10 Core Engine — Relation Manager (internal/core/relation)
  - [ ] 10.1 Implement internal/core/relation/manager.go: RelationManager struct — Relate (validate + create), Unrelate, Traverse (delegate to IRelationStore BFS), AutoDetect (scan recent memories for implicit relations), FindPath
  - [ ] 10.2 Write unit tests for RelationManager with mock IRelationStore

- [ ] 11 Core Engine — Lifecycle Engine (internal/core/lifecycle)
  - [ ] 11.1 Implement internal/core/lifecycle/decay.go: ComputeDecayScore(memory, now) float64 implementing exponential decay formula with type-specific lambda, accessBoost, typeMultiplier, and floor
  - [ ] 11.2 Implement internal/core/lifecycle/engine.go: LifecycleEngine struct — RunDecay (compute + BulkUpdateRelevance + archive below threshold), RunArchival, RunGC (hard-delete old deleted memories), PromoteMemory, Start (background ticker), Stop
  - [ ] 11.3 Write unit tests for decay formula: monotonicity, floor enforcement, type-specific rates
  - [ ] 11.4 Write property-based tests: decay score monotonicity over time

- [-] 12 Mnemos Facade (internal/core/mnemos.go)
  - [ ] 12.1 Implement internal/core/mnemos.go: Mnemos struct implementing IMnemosAPI — constructor with DI for all engines and stores, Store/Get/Update/Delete/Search/Relate/Traverse/AssembleContext/Maintain/Stats/Shutdown methods
  - [ ] 12.2 Implement background worker management: start/stop goroutines for LifecycleEngine ticker, embedding queue processor, markdown sync queue
  - [ ] 12.3 Implement graceful shutdown: drain queues, stop tickers, close SQLite connection
  - [ ] 12.4 Write integration tests for Facade with real SQLite in-memory DB

- [-] 13 Configuration (internal/config)
  - [ ] 13.1 Implement internal/config/config.go: Config struct with all fields, LoadConfig() using viper (file + env vars with MNEMOS_ prefix), DefaultConfig(), config file discovery (global ~/.mnemos/ + project-local .mnemos/)
  - [ ] 13.2 Write unit tests for config loading with temp config files

- [-] 14 MCP Transport (internal/transport/mcp)
  - [ ] 14.1 Implement internal/transport/mcp/server.go: MCP server setup using mark3labs/mcp-go, stdio transport, server info and capabilities registration
  - [ ] 14.2 Implement internal/transport/mcp/tools.go: 8 MCP tools — mnemos_store, mnemos_search, mnemos_get, mnemos_update, mnemos_delete, mnemos_relate, mnemos_context, mnemos_maintain — with input schemas, handlers, and error formatting
  - [ ] 14.3 Implement internal/transport/mcp/resources.go: 2 MCP resources — mnemos://memories/{project_id} and mnemos://stats
  - [ ] 14.4 Implement internal/transport/mcp/prompts.go: 2 MCP prompts — load_context and save_session
  - [ ] 14.5 Write integration tests for MCP tools using in-process MCP client

- [-] 15 CLI Transport (internal/transport/cli)
  - [ ] 15.1 Implement internal/transport/cli/root.go: root cobra command with persistent flags (--config, --project, --log-level) and version info
  - [ ] 15.2 Implement CLI commands: init (create data dir + DB + config), serve (start MCP or REST server)
  - [ ] 15.3 Implement CLI commands: store, get, list (with table output), search (with table output and score display)
  - [ ] 15.4 Implement CLI commands: update, delete, relate, stats (formatted output), maintain (summary output)
  - [ ] 15.5 Implement CLI commands: config (get/set config values)
  - [ ] 15.6 Write unit tests for CLI commands with mock Facade

- [-] 16 REST Transport (internal/transport/rest)
  - [ ] 16.1 Implement internal/transport/rest/server.go: net/http server with router, middleware (logging, CORS, recovery), graceful shutdown
  - [ ] 16.2 Implement internal/transport/rest/handlers.go: handlers for all REST endpoints — POST /memories, GET /memories/{id}, PATCH /memories/{id}, DELETE /memories/{id}, GET /memories, POST /memories/search, POST /memories/{id}/relate, GET /stats, POST /maintain
  - [ ] 16.3 Write integration tests for REST handlers

- [-] 17 Entry Point (cmd/mnemos/main.go)
  - [ ] 17.1 Implement cmd/mnemos/main.go: wire config loading, storage initialization, engine construction, and CLI root command execution

- [x] 18 End-to-End Integration Tests
  - [ ] 18.1 Write E2E test: full Store → Search → Get round-trip with real SQLite
  - [ ] 18.2 Write E2E test: MCP tool invocation via stdio transport
  - [ ] 18.3 Write E2E test: lifecycle decay + archival pipeline
  - [ ] 18.4 Write E2E test: hybrid search with mock embedding provider
  - [ ] 18.5 Write E2E test: relation graph traversal

- [ ] 19 Documentation & Distribution
  - [x] 19.1 Write README.md with quick start, MCP config examples, CLI reference, and configuration reference
  - [x] 19.2 Configure .goreleaser.yml for darwin-arm64, darwin-amd64, linux-amd64, linux-arm64, windows-amd64 builds
  - [x] 19.3 Create install.sh script for curl-based installation
  - [x] 19.4 Create Homebrew formula template
