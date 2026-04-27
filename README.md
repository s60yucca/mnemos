# mnemos

**The autopilot knowledge base for your coding agent.**

Install once. From then on, your agent builds itself a structured knowledge base of your project — while you code. No prompts to remember, no `remember()` calls, no API to learn.

Single Go binary. Embedded SQLite. Zero cloud. No Docker. No Python. No Node runtime.

```
Agent (Claude Code / Cursor / Kiro / Gemini CLI / Codex / ...)
    ↓ MCP stdio
mnemos serve
    ↓
Auto-compiled knowledge base (~/.mnemos/mnemos.db)
```

---

## What makes mnemos different

Every memory server stores text. Mnemos **compiles a knowledge base**.

While other servers expect you (or a carefully-tuned prompt) to decide *when* to store and *when* to retrieve, mnemos runs a full pipeline in the background:

```
Agent action → mnemos auto-pipeline:
                ├── Quality gate        (reject/rewrite low-value content)
                ├── 3-tier dedup        (hash → fuzzy → semantic)
                ├── Auto-summarize      (extractive, fast; LLM if available)
                ├── File linking        (extract identifiers, link to code)
                ├── Type classification (episodic / long_term / semantic / working)
                ├── Quality scoring     (for retrieval ranking)
                └── Decay scheduling    (so knowledge base stays relevant)

Retrieval:
                ├── Hybrid search       (FTS5 + optional semantic + RRF)
                ├── File-overlap boost  (memories about active files rank higher)
                ├── MMR diversity       (kill redundant results)
                ├── Adaptive packing    (full content or summary based on budget)
                └── Token-budget cap    (always fits in context)
```

You don't call any of this. Your agent doesn't call any of this. Hooks fire it automatically on session start, prompt submit, and session end.

---

## Three layers, all shipping today

**Layer 1 — MCP transport.** Standard MCP server, stdio, works with any MCP client.

**Layer 2 — Autopilot hooks.** One command (`mnemos setup claude`) wires hooks + steering + MCP config. Session start auto-injects relevant context. Prompt submit auto-searches on topic change. Session end verifies coverage.

**Layer 3 — Auto-compiled knowledge base.** Quality gate, 3-tier dedup, auto-summarization, file linking, MMR context assembly — all automatic. You never trigger them. Includes passive background daemon that continuously detects staleness, contradiction, and missing relations across your memory base.

---

## Compared honestly

| | Mem0 | Zep/Graphiti | engram | OMEGA | **mnemos** |
|---|---|---|---|---|---|
| MCP-native | ✓ | ✓ | ✓ | ✓ | ✓ |
| Single binary, no runtime deps | — | — | ✓ | — | ✓ |
| Zero cloud / local-first | partial | — | ✓ | ✓ | ✓ |
| 1-command autopilot setup | — | — | — | — | **✓** |
| Auto-quality gate | — | — | — | — | **✓** |
| Auto-summarization | — | — | — | — | **✓** |
| Auto file-linking (git-aware) | — | — | — | — | **✓** |
| MMR context assembly | — | — | — | — | **✓** |
| Passive background daemon | — | — | — | — | **✓** |
| Temporal knowledge graph | — | ✓ | — | — | partial (decay + supersede) |
| Self-host cost | $0-cloud | ~$50/mo (Neo4j) | $0 | $0 | $0 |

Mnemos isn't trying to be Zep — different bet. Zep is the best answer if you need temporal reasoning over business facts and have enterprise infrastructure. Mnemos is the best answer if you're a coding agent user who wants an autopilot knowledge base that runs itself on your laptop.

---

## Install

```bash
# Homebrew (macOS / Linux)
brew install s60yucca/tap/mnemos && mnemos setup claude

# curl (verify mnemos.dev is live before using)
curl -fsSL https://mnemos.dev/install.sh | bash && mnemos setup claude

# npm (coming in v1.2)
# npx mnemos setup claude

# Build from source (requires Go 1.23+)
git clone https://github.com/s60yucca/mnemos
cd mnemos && make build
```

Swap `claude` for `cursor`, `kiro`, `gemini-cli`, or `codex`. Restart your client. Autopilot runs from here.

---

## What autopilot actually does

`mnemos setup <client>` writes:

- **Steering file** (`CLAUDE.md`, `.cursorrules`, `.kiro/steering/mnemos.md`) — tells the agent what's worth storing
- **Hook config** (`.claude/hooks.json` or equivalent) — wires lifecycle events
- **MCP config** (`.mcp.json`) — registers `mnemos serve` as tool provider

Three hooks run automatically:

**Session start** → `mnemos hook session-start`
Assembles relevant memories within a token budget (MMR-diversified, file-boosted). Injects into context. Cold start < 200 ms.

**Prompt submit** → `mnemos hook prompt-submit`
Detects topic + intent changes. Auto-searches knowledge base when the shift is meaningful. Respects cooldown to avoid noise.

**Session end** → `mnemos hook session-end`
Verifies whether durable memory was captured. Optionally stores a minimal breadcrumb. Cleans up session state.

Steering tells the agent *what* is worth remembering. Hooks handle retrieval, dedup, summarization, linking — so the agent doesn't waste tokens thinking about memory logistics.

---

## Passive autopilot daemon

Beyond hooks, mnemos runs a background daemon that continuously improves your knowledge base:

- **Staleness detection** — flags memories that reference deleted files or outdated patterns
- **Contradiction detection** — finds memories that conflict with each other
- **Relation inference** — automatically links related memories
- **Backfill** — retroactively generates summaries for memories that lack them

```bash
mnemos autopilot status          # check daemon state
mnemos autopilot run             # trigger immediate run
mnemos autopilot run --dry-run   # preview findings without writing
mnemos autopilot report          # view latest findings
```

---

## Performance benchmark (latency)

| Operation | 350 memories | 1,500 memories |
|---|---|---|
| `store` (new, with full pipeline) | 57 ms | 24 ms |
| `store` (dedup hit) | 55 ms | 22 ms |
| `search` hybrid (RRF + file boost) | 42 ms | 39 ms |
| `maintain` (decay + GC) | 27 ms | 108 ms |
| hook session-start (cold) | < 200 ms | — |
| binary size | ~12 MB | — |

*Hardware: M1 Pro, 16GB RAM, SQLite on SSD. Your latency may vary.*

Most operations stay under 60 ms regardless of dataset size. Hook subcommands use `InitLight` mode — no background workers, no session interrupt.

**Value benchmark (token savings, precision, gotcha avoidance) is in progress.** See [DOGFOODING_RUNBOOK.md](docs/benchmarks/DOGFOODING_RUNBOOK.md) for methodology. Real numbers will replace this placeholder before public launch.

---

## MCP tools

| Tool | What it does |
|---|---|
| `mnemos_store` | Store a memory (full auto-pipeline runs transparently) |
| `mnemos_search` | Hybrid FTS + semantic + file-overlap search with MMR |
| `mnemos_context` | Assemble budget-aware, diversified context for session start |
| `mnemos_get` | Fetch by ID |
| `mnemos_update` | Update content, summary, or tags |
| `mnemos_delete` | Soft-delete (recoverable via maintain) |
| `mnemos_relate` | Link two memories (supersedes, caused_by, depends_on) |
| `mnemos_maintain` | Run decay, archival, GC, stale detection |

---

## Quick start after install

```bash
# Agents call these automatically via MCP. You can also use directly:
mnemos store "JWT uses RS256, 1h expiry, config in auth/config.go"
mnemos search "token expiry"
mnemos stats
mnemos maintain
```

---

## Configuration

Most users never touch this. But if you want:

```yaml
# ~/.mnemos/config.yaml
embeddings:
  provider: noop              # noop (default) | ollama | openai
  # Pure FTS works fine. Enable semantic for meaning-based search.

quality_gate:
  min_words: 5
  max_words: 200
  min_density: 0.3
  require_specific: true      # long_term memories need project identifiers
  duplicate_threshold: 0.8

summarization:
  extractive: true             # always on, fast, offline

file_linking:
  enabled: true                # auto-disables outside git

hook:
  enabled: true
  search_cooldown: 5m
  session_start_max_tokens: 2000
  mmr_lambda: 0.7              # 0=max diversity, 1=max relevance
  file_boost: 0.3

autopilot:
  enabled: true
  interval: 15m
  contradiction_enabled: false
```

---

## Memory types

Mnemos auto-classifies. Override manually via `--type` flag.

| Type | Decay rate | Use for |
|---|---|---|
| `short_term` | fast (~1 day) | todos, temp notes, WIP |
| `episodic` | medium (~1 month) | session events, bug fixes |
| `long_term` | slow (~6 months) | architecture decisions |
| `semantic` | very slow | facts, definitions, knowledge |
| `working` | fast | active task context |

---

## Why I built this

I got tired of re-explaining my own project to Claude Code every morning.

I tried the existing memory servers. Most of them stored text fine. But every one expected me — or a carefully-tuned prompt — to decide *when* to store and *when* to retrieve. That's not a knowledge base. That's a database with an MCP wrapper.

Mnemos is what I built to make it actually automatic. `mnemos setup claude`, restart the editor, and the knowledge base compiles itself.

---

## Autopilot setup — one command per client

```bash
mnemos setup claude       # writes CLAUDE.md, .claude/hooks.json, .mcp.json
mnemos setup cursor       # writes .cursorrules, .mcp.json
mnemos setup kiro         # writes .kiro/steering/mnemos.md, .kiro/mcp.json
mnemos setup gemini-cli   # writes GEMINI.md, .gemini/settings.json, .mcp.json
mnemos setup codex        # writes ~/.codex/config.toml (global, CLI + VSCode)
```

Flags: `--global` (install for all projects), `--force` (overwrite existing), `--project <id>` (override project ID).

> **Codex note:** Codex uses a single global config shared between the CLI and VSCode extension. `--global` and `--local` flags are ignored — it always writes to `~/.codex/config.toml`. Restart both Codex CLI and the VSCode extension after setup.

---

## CLI reference

```bash
mnemos init                                           # first-time setup
mnemos store "..."                                    # store (auto-pipeline)
mnemos search "auth"                                  # hybrid search
mnemos list --project myapp                           # list memories
mnemos get <id>                                       # fetch by id
mnemos update <id> --content "..."                    # update
mnemos delete <id>                                    # soft delete
mnemos relate <src> <tgt> --type supersedes           # typed relation
mnemos stats                                          # storage + quality stats
mnemos maintain                                       # decay + stale + GC
mnemos serve                                          # MCP server (stdio)
mnemos version

# Autopilot setup
mnemos setup claude | cursor | kiro | gemini-cli | codex [--global] [--force] [--project <id>]

# Passive autopilot daemon
mnemos autopilot status
mnemos autopilot run [--dry-run] [--project <id>]
mnemos autopilot report [--project <id>]

# Backfill
mnemos backfill summaries --project <id> [--dry-run] [--limit N]

# Hook subcommands (called by clients, not manually)
mnemos hook session-start
mnemos hook prompt-submit
mnemos hook session-end
```

---

## What mnemos is not

- **Not a chatbot memory SaaS.** For user-preference recall in customer support bots, use [Mem0](https://mem0.ai).
- **Not a temporal knowledge graph database.** For valid-at/invalid-at reasoning over business facts, use [Zep](https://getzep.com).
- **Not a cloud product.** There is no mnemos cloud. There will never be one.
- **Not framework-specific.** MCP-native. Works with anything that speaks MCP.

Mnemos does one thing: **give agents a knowledge base that compiles itself.**

---

## Roadmap

See [ROADMAP.md](ROADMAP.md). Short version:

- **v1.1.x (shipped):** full auto-pipeline, MMR context assembly, file-aware retrieval, passive autopilot daemon, benchmark framework, Codex support
- **v1.2 (next):** public value benchmark (dogfooding), npm wrapper, demo GIF, HN launch
- **v1.3 (planned):** team memory via git — shared `.mnemos/shared/` for teammate knowledge
- **v2.0+ (TBD):** cross-project memory scopes, memory compaction, driven by user feedback

---

## Community

- [GitHub Discussions](https://github.com/s60yucca/mnemos/discussions)
- [GitHub Issues](https://github.com/s60yucca/mnemos/issues)

---

## License

MIT
