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
- `Autopilot ready`: one setup command wires hooks + steering for Claude, Kiro, or Cursor

---

## Quick Start

```bash
# install
curl -fsSL https://raw.githubusercontent.com/mnemos-dev/mnemos/main/install.sh | bash

# first-time setup
mnemos init

# run as MCP server
mnemos serve
```

Then wire it to your AI client — or use autopilot setup to make it fully automatic.

---

## Autopilot Setup

Mnemos is an MCP server, not an agent controller. By default, the agent only uses memory if it's instructed to. **Autopilot** closes that gap: one command injects hook config, steering files, and MCP config so the agent uses memory automatically on every session — no reminding needed.

```bash
# Claude Code
mnemos setup claude

# Kiro
mnemos setup kiro

# Cursor
mnemos setup cursor
```

That's it. From that point:

- **Session start** — Mnemos automatically loads relevant context into the agent's window
- **During work** — Mnemos searches memory when the topic changes
- **Session end** — Mnemos verifies memory was captured and cleans up state

Use `--global` to install for all projects instead of just the current one:

```bash
mnemos setup claude --global
```

Use `--force` to overwrite existing config files without prompting:

```bash
mnemos setup claude --force
```

### What `mnemos setup` writes

**Claude Code** (`mnemos setup claude`):

| File | Purpose |
|------|---------|
| `CLAUDE.md` | Steering instructions — tells Claude when and what to store |
| `.claude/hooks.json` | Hook config — wires session-start, prompt-submit, session-end |
| `.mcp.json` | MCP server config — registers `mnemos serve` |

**Kiro** (`mnemos setup kiro`):

| File | Purpose |
|------|---------|
| `.kiro/steering/mnemos.md` | Steering file — auto-loaded by Kiro on every session |
| `.kiro/mcp.json` | MCP server config |

**Cursor** (`mnemos setup cursor`):

| File | Purpose |
|------|---------|
| `.cursorrules` | Steering instructions for Cursor |
| `.mcp.json` | MCP server config |

### How autopilot works under the hood

Autopilot uses two complementary systems:

**Hooks (deterministic)** — `mnemos hook session-start/prompt-submit/session-end`

These are short-lived processes called by the AI client at specific lifecycle events. They run in `InitLight` mode (no background workers, cold start < 50ms) and always exit 0 — they never interrupt the agent session.

- `session-start`: assembles relevant context from Mnemos and injects it into the agent's window
- `prompt-submit`: detects topic changes using Jaccard similarity; searches memory when the topic shifts; respects a 5-minute cooldown per topic to avoid noise
- `session-end`: counts memories stored during the session; optionally leaves a breadcrumb; cleans up session state

**Steering (LLM-guided)** — `CLAUDE.md` / `.kiro/steering/mnemos.md` / `.cursorrules`

These files instruct the agent on *what* to store and *when*. The agent makes the semantic judgment — hooks handle the mechanical retrieval.

```
Session start  →  hook injects context  →  agent reads it
During work    →  hook searches on topic change  →  agent receives results
               →  agent discovers durable learning  →  agent calls mnemos_store via MCP
Session end    →  hook verifies coverage  →  state cleanup
```

### Session state

Each session gets its own state file at `.mnemos/sessions/session-<id>.json` (falls back to `~/.mnemos/sessions/` if no local `.mnemos/` dir exists). Stale and orphaned sessions are cleaned up automatically.

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
| One-command autopilot setup | ❌ | ❌ | ❌ | ✅ |
| Written in Go | ❌ | ✅ | ❌ (Python) | ✅ |

---

## Install

```bash
# curl (macOS / Linux)
curl -fsSL https://raw.githubusercontent.com/mnemos-dev/mnemos/main/install.sh | bash

# Homebrew
brew install mnemos-dev/tap/mnemos

# Build from source (requires Go 1.23+)
git clone https://github.com/mnemos-dev/mnemos
cd mnemos && make build
# binary at: bin/mnemos
```

Initialize on first run:

```bash
mnemos init
# Creates ~/.mnemos/mnemos.db and ~/.mnemos/config.yaml
```

---

## Use with Claude Code

**Autopilot (recommended):**

```bash
mnemos setup claude
```

This writes `CLAUDE.md`, `.claude/hooks.json`, and `.mcp.json` in one shot. Restart Claude Code and memory is fully automatic.

**Manual setup:**

Add to `.mcp.json` in your project root:

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

Then add a session instruction based on [templates/claude/CLAUDE.md](./templates/claude/CLAUDE.md).

---

## Use with Kiro

**Autopilot (recommended):**

```bash
mnemos setup kiro
```

This writes `.kiro/steering/mnemos.md` and `.kiro/mcp.json`. Kiro picks up the steering file automatically on every session.

**Manual setup:**

Add to `.kiro/settings/mcp.json`:

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

Copy [templates/kiro/steering/mnemos.md](./templates/kiro/steering/mnemos.md) to `.kiro/steering/mnemos.md`.

---

## Use with Cursor / Windsurf / any MCP client

**Autopilot:**

```bash
mnemos setup cursor
```

**Manual:** Same JSON MCP config as above — mnemos speaks standard MCP over stdio.

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

# Autopilot setup
mnemos setup claude                                   # setup for Claude Code
mnemos setup kiro                                     # setup for Kiro
mnemos setup cursor                                   # setup for Cursor
mnemos setup claude --global                          # install globally (home dir)
mnemos setup claude --force                           # overwrite without prompting

# Hook subcommands (called automatically by AI clients — not for manual use)
mnemos hook session-start
mnemos hook prompt-submit
mnemos hook session-end
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

hook:
  enabled: true
  search_cooldown: 5m
  session_start_max_tokens: 2000
  prompt_search_limit: 5
  stale_timeout: 1h
  log_level: warn
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
| hook session-start | < 200 ms | — | cold start, InitLight |
| hook prompt-submit | < 100 ms | — | cold start, InitLight |
| hook session-end | < 100 ms | — | cold start, InitLight |
| binary size | 12 MB | — | single static binary |
| startup time | ~50 ms | — | cold start |

Most operations stay under 60 ms regardless of dataset size. Hook subcommands use `InitLight` mode — no background workers start, keeping latency low enough to not interrupt agent sessions.

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
