# mnemos

> Your AI agent has the memory of a goldfish. Mnemos fixes that.

A persistent memory engine for AI coding agents.

Mnemos gives Claude Code, Kiro, Cursor, Windsurf, and other MCP clients a memory that survives across sessions: architecture decisions, bug root causes, project conventions, and non-obvious implementation details.

Single Go binary. Embedded SQLite. Zero runtime dependencies. No Docker. No cloud. No Python. No Node.

```
Agent (Claude Code / Kiro / Cursor / Windsurf / ...)
    ↓ MCP stdio
mnemos serve
    ↓
SQLite + FTS5 (~/.mnemos/mnemos.db)
```

---

## What does it actually do?

Every time your agent learns something worth keeping, it stores it in Mnemos. Next session, it can pull that context back before it starts coding.

That means:

- fewer repeated explanations
- less re-discovery of old bugs and decisions
- more continuity across sessions
- better context for long-running projects

No more re-explaining your project structure every Monday morning. No more rediscovering the same environment quirk three times in one week.

**The memory lifecycle:**

1. Agent finishes something meaningful (fixed a bug, made a decision, learned a pattern)
2. Calls `mnemos_store` with the content
3. Mnemos deduplicates, classifies, and indexes it
4. Next session: `mnemos_context` assembles relevant memories within a token budget
5. Agent picks up right where it left off

---

## Why it feels different

Mnemos is built for real coding workflows, not just generic note storage.

- `MCP-native`: designed to be called directly by coding agents
- `Fast to install`: one binary, one local database
- `Actually useful retrieval`: FTS + optional semantic search + context assembly
- `Lifecycle aware`: deduplication, relevance decay, archive/GC
- `Readable by humans too`: optional Markdown mirror
- `Ready for autopilot`: works best when paired with Claude Code prompts or Kiro steering

---

## Quick Start

```bash
# install
curl -fsSL https://raw.githubusercontent.com/s60yucca/mnemos/main/install.sh | bash

# first-time setup
mnemos init

# run as MCP server
mnemos serve
```

Then connect it from Claude Code or Kiro and teach the agent to:

1. call `mnemos_context` at task start
2. call `mnemos_search` before targeted implementation
3. call `mnemos_store` after durable learnings

If you want that to happen consistently, use the autopilot setup below.

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
curl -fsSL https://raw.githubusercontent.com/s60yucca/mnemos/main/install.sh | bash

# Homebrew
brew install s60yucca/tap/mnemos

# Build from source (requires Go 1.23+)
git clone https://github.com/s60yucca/mnemos
cd mnemos && make build
# binary at: bin/mnemos
```

Initialize on first run:

```bash
mnemos init
# Creates ~/.mnemos/mnemos.db and ~/.mnemos/config.yaml
```

---

## Autopilot First

Mnemos is an MCP server, not an agent controller.

That distinction matters:

- installing the server gives the agent access to memory
- steering, prompts, or plugins make the agent use memory consistently

If you want Mnemos to feel automatic, the best current paths are:

- `Claude Code`: pair Mnemos with a reusable session prompt
- `Kiro`: pair Mnemos with a steering file and tool auto-approve

The recommended policy is simple:

1. At session start, call `mnemos_context` once.
2. Before targeted implementation, call `mnemos_search`.
3. After meaningful completed work, call `mnemos_store` once if the learning is durable.
4. Skip low-value memories.

The full policy lives in [docs/autopilot.md](./docs/autopilot.md).

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

Claude Code will not reliably use Mnemos just because the MCP server exists. If you want near-autopilot behavior, add a reusable session instruction based on [templates/claude/SYSTEM_PROMPT.md](./templates/claude/SYSTEM_PROMPT.md).

Recommended Claude Code flow:

1. At the start of a new task or session, call `mnemos_context` once with the current task, bug, feature, or subsystem as the query.
2. Before coding in a specific area, call `mnemos_search` if targeted memory could affect the implementation.
3. After a meaningful change, call `mnemos_store` once if the work produced a durable learning.
4. Do not store temporary plans, obvious code summaries, or task chatter.

This is the same pattern used by workflow plugins: the MCP server provides capability, but the client prompt or plugin must make its usage mandatory.

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

You can start from [templates/kiro/steering/mnemos.md](./templates/kiro/steering/mnemos.md).

Recommended Kiro flow:

1. Install Mnemos as an MCP server.
2. Auto-approve read-oriented tools: `mnemos_context`, `mnemos_search`, `mnemos_get`.
3. Add the steering file so Kiro automatically loads memory at task start.
4. Keep `mnemos_store` manual at first if you want to watch memory quality, then auto-approve writes once the workflow is stable.

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

## Autopilot

Mnemos is an MCP server, not an agent controller. Automatic usage depends on client-side steering.

- Claude Code: use a strong reusable session prompt so Claude calls `mnemos_context` at task start, `mnemos_search` before targeted work, and `mnemos_store` after durable learnings.
- Kiro: use a steering file plus auto-approve for read-oriented Mnemos tools.
- Generic MCP clients: Mnemos can expose the tools, but if the client does not support steering, plugins, or strong reusable prompts, memory usage will remain inconsistent.

**Recommended default policy**

- Session start: call `mnemos_context` once with a `1500-3000` token budget.
- Before targeted implementation: call `mnemos_search` with the subsystem, bug, or feature name.
- After meaningful completion: call `mnemos_store` once for durable learnings only.
- Prefer no memory over a weak memory.

**Good things to store**

- architecture decisions
- bug root causes
- project conventions
- important implementation constraints
- deployment or environment gotchas

**Do not store**

- temporary plans
- raw diffs
- obvious code descriptions
- work-in-progress notes
- conversational filler

The full recommended policy is in [docs/autopilot.md](./mnemos/docs/autopilot.md).

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
