# mnemos

> Your AI agent has the memory of a goldfish. Mnemos fixes that.

A persistent memory engine for AI coding agents. Single Go binary, zero runtime dependencies, MCP-native.

Mnemos stores, searches, and manages memories across sessions using embedded SQLite — no external services, no Docker, no cloud, no Python, no Node.js. Just one binary and a `.db` file.

```
Agent (Claude Code / Kiro / Cursor / Windsurf / ...)
    ↓ MCP stdio
mnemos serve
    ↓
SQLite + FTS5 (~/.mnemos/mnemos.db)
```

---

## What does it actually do?

Every time your agent learns something worth keeping — an architecture decision, a bug fix, a project convention — it calls `mnemos_store`. Next session, it calls `mnemos_context` and gets that knowledge back, as if it never forgot.

No more re-explaining your project structure every Monday morning.

**The memory lifecycle:**

1. Agent finishes something meaningful (fixed a bug, made a decision, learned a pattern)
2. Calls `mnemos_store` with the content
3. Mnemos deduplicates, classifies, and indexes it
4. Next session: `mnemos_context` assembles relevant memories within a token budget
5. Agent picks up right where it left off

---

## Why mnemos?

| | claude-mem | engram | neural-memory | mnemos |
|---|---|---|---|---|
| MCP native | ✅ | ✅ | ✅ | ✅ |
| Single binary / zero install | ❌ | ✅ | ❌ (pip) | ✅ |
| Zero config to start | ✅ | ✅ | ❌ | ✅ |
| Hybrid search (FTS + semantic RRF) | ❌ | ❌ | ❌ | ✅ |
| Memory decay / lifecycle | ❌ | ❌ | ✅ | ✅ |
| Deduplication | ❌ | ❌ | ❌ | ✅ (3-tier) |
| Relation graph | ❌ | ❌ | ✅ (spreading activation) | ✅ |
| Token-budget context assembly | ❌ | partial | ❌ | ✅ |
| Human-readable Markdown mirror | ✅ | ❌ | ❌ | ✅ |
| Works with Kiro / Cursor / Windsurf | ❌ | ✅ | ✅ | ✅ |
| No Python / Node runtime required | ✅ | ✅ | ❌ | ✅ |
| Written in Go | ❌ | ✅ | ❌ (Python) | ✅ |

---

## Install

```bash
# curl (macOS / Linux)
curl -fsSL https://raw.githubusercontent.com/s60yucca/mnemo/main/mnemos/install.sh | bash

# Homebrew
brew install mnemos-dev/tap/mnemos

# Build from source (requires Go 1.23+)
git clone https://github.com/s60yucca/mnemo
cd mnemo/mnemos && make build
# binary at: bin/mnemos
```

Initialize on first run:

```bash
mnemos init
# Creates ~/.mnemos/mnemos.db and ~/.mnemos/config.yaml
```

---

## Use with Claude Code

Add to `~/.claude.json` (global) or `.mcp.json` in your project root:

```json
{
  "mcpServers": {
    "mnemos": {
      "command": "mnemos",
      "args": ["serve"],
      "env": {
        "MNEMOS_PROJECT_ID": "my-project"
      }
    }
  }
}
```

Restart Claude Code. Mnemos tools appear automatically.

---

## Use with Kiro

Add to `~/.kiro/settings/mcp.json` (global) or `.kiro/settings/mcp.json` in your workspace:

```json
{
  "mcpServers": {
    "mnemos": {
      "command": "mnemos",
      "args": ["serve"],
      "env": {
        "MNEMOS_PROJECT_ID": "my-project"
      },
      "disabled": false,
      "autoApprove": ["mnemos_search", "mnemos_get", "mnemos_context"]
    }
  }
}
```

For automatic memory usage on every session, add a steering file at `.kiro/steering/mnemos.md` telling the agent to call `mnemos_context` at session start and `mnemos_store` when it learns something. Kiro will follow it automatically.

---

## Use with Cursor / Windsurf / any MCP client

Same JSON config — mnemos speaks standard MCP over stdio. Works with any client that supports MCP tools.

---

## MCP Tools

| Tool | What it does |
|------|-------------|
| `mnemos_store` | Store a memory with optional type, tags, project scope |
| `mnemos_search` | Hybrid FTS + semantic search with RRF ranking |
| `mnemos_get` | Fetch a memory by ID |
| `mnemos_update` | Update content, summary, or tags |
| `mnemos_delete` | Soft-delete (recoverable via maintain) |
| `mnemos_relate` | Link two memories with a typed relation |
| `mnemos_context` | Assemble relevant memories within a token budget |
| `mnemos_maintain` | Run decay, archival, and garbage collection |

**Resources**: `mnemos://memories/{project_id}`, `mnemos://stats`

**Prompts**: `load_context` (session start), `save_session` (session end)

---

## CLI

```bash
mnemos init                                           # first-time setup
mnemos store "JWT uses RS256, tokens expire in 1h"    # store a memory
mnemos search "authentication"                        # hybrid search
mnemos search "auth" --mode text                      # text-only search
mnemos list --project myapp                           # list memories
mnemos get <id>                                       # fetch by id
mnemos update <id> --content "updated text"           # update
mnemos delete <id>                                    # soft delete
mnemos delete <id> --hard                             # permanent delete
mnemos relate <src-id> <tgt-id> --type depends_on     # create relation
mnemos stats --project myapp                          # storage stats
mnemos maintain                                       # decay + GC
mnemos serve                                          # start MCP server (stdio)
mnemos serve --rest --port 8080                       # start REST server
mnemos version                                        # print version
```

Global flags: `--project <id>`, `--config <path>`, `--log-level debug|info|warn|error`

---

## Memory Types

Mnemos auto-classifies memories based on content. You can override manually.

| Type | Decay rate | Use for |
|------|-----------|---------|
| `short_term` | fast (~1 day) | todos, temp notes, WIP |
| `episodic` | medium (~1 month) | session events, bug fixes |
| `long_term` | slow (~6 months) | architecture decisions |
| `semantic` | very slow | facts, definitions, knowledge |
| `working` | fast | active task context |

---

## How search works

Mnemos uses **Reciprocal Rank Fusion (RRF)** to combine two search signals:

- **FTS5** — SQLite full-text search with BM25 ranking. Fast, offline, no setup.
- **Semantic** — vector cosine similarity via embeddings. Optional, requires Ollama or OpenAI.

With only FTS5 (default), search is keyword-based but still very good. Enable embeddings to find memories by meaning — e.g. query "token expiry" finds a memory about "JWT RS256 1h lifetime".

---

## Configuration

`~/.mnemos/config.yaml`:

```yaml
data_dir: ~/.mnemos
log_level: info
log_format: text        # text or json

embeddings:
  provider: noop          # noop (default) | ollama | openai
  base_url: http://localhost:11434
  model: nomic-embed-text
  dims: 384
  api_key: ""

dedup:
  fuzzy_threshold: 0.85
  semantic_threshold: 0.92

lifecycle:
  decay_interval: 24h
  gc_retention_days: 30
  archive_threshold: 0.1

mirror:
  enabled: false          # set true to write human-readable Markdown files
  base_dir: ~/.mnemos/mirror
```

Environment variables override config — prefix with `MNEMOS_`:

```bash
MNEMOS_PROJECT_ID=myapp        # scope memories to a project
MNEMOS_LOG_LEVEL=debug

# Only needed if using semantic embeddings (optional):
MNEMOS_EMBEDDINGS_PROVIDER=ollama
MNEMOS_EMBEDDINGS_API_KEY=sk-...
```

---

## Embedding Providers

Embeddings are **optional**. By default mnemos uses `noop` — pure FTS5 text search, zero config, works fully offline.

Enable embeddings only if you want semantic similarity search (find memories by meaning, not just keywords).

**Ollama** (local, free, no API key):
```yaml
embeddings:
  provider: ollama
  base_url: http://localhost:11434
  model: nomic-embed-text
  dims: 768
```

**OpenAI**:
```yaml
embeddings:
  provider: openai
  model: text-embedding-3-small
  dims: 1536
  api_key: sk-...
```

---

## Performance

Benchmarked on macOS (Apple M-series), SQLite WAL mode, embeddings disabled (noop), cold process start per operation.

| Operation | 350 memories | 1500 memories | Notes |
|-----------|-------------|--------------|-------|
| `store` (new) | 57 ms | 24 ms | includes dedup check |
| `store` (dedup hit) | 55 ms | 22 ms | hash match, no write |
| `search` text (FTS5) | 60 ms | 54 ms | BM25 ranking |
| `search` hybrid (RRF) | 42 ms | 39 ms | FTS + noop vector |
| `list` | 34 ms | 26 ms | sorted by created_at |
| `maintain` (decay+GC) | 27 ms | 108 ms | full table scan |
| binary size | 12 MB | — | single static binary |
| startup time | ~50 ms | — | cold start |

Most operations stay under 60 ms regardless of dataset size. With semantic embeddings enabled, `store` adds ~50–200 ms per memory for embedding generation — search quality improves significantly.

---

## REST API

```bash
mnemos serve --rest --port 8080
```

```
POST   /memories              store
GET    /memories/{id}         get
PATCH  /memories/{id}         update
DELETE /memories/{id}         soft-delete
GET    /memories              list
POST   /memories/search       search
POST   /memories/{id}/relate  relate
GET    /stats                 stats
POST   /maintain              maintenance
```

---

## Build

```bash
make build    # → bin/mnemos
make test     # all tests
make lint     # golangci-lint
make release  # goreleaser snapshot
```

---

## License

MIT
