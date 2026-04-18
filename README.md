# 🧠 Mnemos

> **Stop burning $10/day on AI context. Give your AI coding agents long-term memory for free.**

Mnemos is a blazing-fast, persistent memory engine for AI coding agents (Kiro, Claude Code, Cursor, Windsurf). It acts as a "second brain" via the Model Context Protocol (MCP), saving you money and frustration.

---

## 💸 The Reality of AI Coding

**❌ Without Mnemos:**
Your session grows to 50k tokens. The AI gets confused and forgets earlier instructions. You clear the chat to save money. Next session, you have to re-explain your project structure, CSS conventions, and old bugs all over again. 
**Result:** Wasted time and high API bills.

**✅ With Mnemos:**
Your AI learns architecture decisions, bug root causes, and project conventions *once*. Mnemos automatically deduplicates and stores it. Next session, Mnemos precisely injects only the relevant 2k tokens of context. 
**Result:** Infinite memory continuity across sessions for pennies.

---

## 🚀 Why it's a Must-Have

* **Zero Bullshit Stack:** Single Go binary. Embedded pure-Go SQLite (FTS5). Zero runtime dependencies. No Docker. No Python. No Node.
* **MCP-Native:** Designed specifically for the Model Context Protocol. Plugs straight into your favorite agents.
* **1-Click Autopilot:** Instantly wires hooks, `.cursorrules`, and MCP configs for Claude Code, Cursor, and Kiro.
* **Smart Lifecycle:** Built-in 3-tier deduplication, relevance decay, and garbage collection. It only remembers what actually matters.
* **Hybrid Search:** Fast local FTS5 keyword search + optional Semantic Vector search (Ollama/OpenAI) using Reciprocal Rank Fusion (RRF).
* **✨ NEW - Knowledge Synthesis:** Introduces the `mnemos_compile` tool for the AI to distill fragmented memories into comprehensive, long-term architectural documents.
* **✨ NEW - Intelligent Hooks:** Advanced heuristic prompt filtering (detects technical terms) and session-end memory reminders ensure your agent never misses an important insight.
* **✨ NEW - Passive Autopilot:** A background daemon that silently analyzes your memory store every 15 minutes — detecting stale compiled articles, auto-linking co-referenced memories, and surfacing findings directly in your next session context.
* **✨ NEW - Memory Backfill:** Auto-summarize and backfill past conversations directly into the memory store using `mnemos backfill`.
* **✨ NEW - Retrieval Quality Benchmarks:** Built-in reproducible benchmark suite to measure MMR diversity and precision/recall across simulated session scenarios.

---

## ⚡ Quick Start

**macOS**
```bash
brew install s60yucca/tap/mnemos
```

**Linux**
```bash
curl -fsSL https://raw.githubusercontent.com/s60yucca/mnemos/main/install.sh | bash
```

**Windows** (PowerShell)
```powershell
irm https://raw.githubusercontent.com/s60yucca/mnemos/main/install.ps1 | iex
```

> Corporate machines with restricted execution policy:
> ```powershell
> powershell -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/s60yucca/mnemos/main/install.ps1 | iex"
> ```

Then on any platform:
```bash
mnemos init
mnemos serve
```

**Alternative installs:** `go install github.com/s60yucca/mnemos/cmd/mnemos@latest` works on all platforms if you have Go. Manual binaries on the [Releases page](https://github.com/s60yucca/mnemos/releases).

---

## 🔌 1-Click Autopilot Integrations

Mnemos isn't just a dumb database. It actively injects memory into your workflow. Run one of these to wire it up instantly:

```bash
# For Kiro (Fully Tested & Highly Recommended)
mnemos setup kiro

# For Gemini CLI / Antigravity (Google)
mnemos setup gemini-cli

# For Cursor (Experimental - Community Testing)
mnemos setup cursor

# For Claude Code (Experimental - Community Testing)
mnemos setup claude
```

*Boom. Your agent now remembers everything automatically.* 
From now on:
1. **Session start:** Mnemos loads relevant context.
2. **During work:** Mnemos searches memory when the topic changes.
3. **Session end:** Mnemos safely stores the durable learnings.

*(Use `--global` to install for all projects, or `--force` to overwrite existing config files).*

---

## 🤖 Using with OpenClaw, Paperclip, or any MCP Client

If you are using emerging AI frameworks like **OpenClaw (Claw bot)**, **Paperclip**, or **Claude Desktop**, you can easily connect Mnemos manually. Mnemos speaks standard MCP over `stdio`. 

Just add this JSON snippet to your client's MCP configuration file (e.g., `openclaw.json`, `paperclip.config.json`, or `claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "mnemos": {
      "command": "mnemos",
      "args": ["serve"],
      "env": {
        "MNEMOS_PROJECT_ID": "my-awesome-project"
      }
    }
  }
}
```

---

## 🆚 Why Mnemos?

| Feature | claude-mem | engram | neural-memory | **mnemos** 🧠 |
|---|---|---|---|---|
| **MCP native** | ✅ | ✅ | ✅ | **✅** |
| **Zero BS Stack** | ❌ | ✅ | ❌ (pip) | **✅ (Single Go Binary)** |
| **1-Click Autopilot Setup** | ❌ | ❌ | ❌ | **✅** |
| **Hybrid Search (FTS + Vector)** | ❌ | ❌ | ❌ | **✅** |
| **Memory Decay / GC** | ❌ | ❌ | ✅ | **✅** |
| **Smart Deduplication** | ❌ | ❌ | ❌ | **✅ (3-tier)** |
| **Token-budget Context** | ❌ | partial | ❌ | **✅** |
| **Works w/ Gemini & Cursor**  | ❌ | ✅ | ✅ | **✅** |
| **Passive Autopilot** | ❌ | ❌ | ❌ | **✅** |

---

## 🧰 MCP Tools

| Tool | What it does |
|------|-------------|
| `mnemos_store` | Store a memory with optional type, tags, project scope |
| `mnemos_compile` | Distill knowledge into a compiled article from multiple sources |
| `mnemos_search` | Hybrid FTS + semantic search with RRF ranking |
| `mnemos_get` | Fetch a memory by ID |
| `mnemos_update` | Update content, summary, or tags |
| `mnemos_delete` | Soft-delete (recoverable via maintain) |
| `mnemos_relate` | Link two memories with a typed relation |
| `mnemos_context` | Assemble relevant memories via HybridSearch (FTS5 + vector similarity), MMR diversity ranking, and adaptive token packing |
| `mnemos_maintain` | Run decay, archival, and garbage collection |

---

## 🤖 Passive Autopilot

Once `mnemos serve` is running, the autopilot daemon starts automatically in the background. Every 15 minutes (configurable), it:

1. **Detects stale compiled articles** — finds articles whose source memories have been updated since compilation
2. **Auto-links co-referenced memories** — creates `relates_to` relations between memories that mention the same file paths, Go identifiers, CLI commands, or config keys
3. **Writes a concise report** — stored as a system memory, surfaced automatically at the start of your next session under "Autopilot Suggestions"

The daemon skips projects with no new activity since the last run (idle skip), so it never wastes cycles.

**CLI commands:**

```bash
mnemos autopilot status              # show daemon state, last run, next scheduled run
mnemos autopilot run                 # trigger an immediate run across all projects
mnemos autopilot run --dry-run       # run detectors without writing anything
mnemos autopilot run --project myapp # run only for a specific project
mnemos autopilot report              # print the latest autopilot report
mnemos autopilot report --project myapp
```

**Configuration** (`~/.mnemos/config.yaml`):

```yaml
autopilot:
  enabled: true
  interval: 15m
  initial_delay: 30s
  max_compiled_per_run: 50
  max_memories_per_run: 200
  contradiction_enabled: false   # experimental, off by default
  contradiction_threshold: 0.3
```

---

## 🛠️ CLI Mastery

Mnemos comes with a powerful CLI for when you want to get your hands dirty:

```bash
mnemos store "JWT uses RS256, tokens expire in 1h"    # store a memory manually
mnemos search "authentication"                        # hybrid search
mnemos search "auth" --mode text                      # text-only search
mnemos list --project myapp                           # list memories
mnemos get <id>                                       # fetch by id
mnemos update <id> --content "updated text"           # update
mnemos delete <id>                                    # soft delete
mnemos delete <id> --hard                             # permanent delete
mnemos relate <src-id> <tgt-id> --type depends_on     # create relation
mnemos stats --project myapp                          # storage stats
mnemos maintain                                       # force decay + GC
```

---

## ⚙️ Advanced Configuration & Embeddings

Mnemos works fully offline with zero configuration using FTS5 text search. 
If you want **Semantic Search** (finding memories by meaning, not just keywords), you can easily hook up Ollama or OpenAI.

Edit `~/.mnemos/config.yaml`:

**Local & Free (Ollama):**
```yaml
embeddings:
  provider: ollama
  base_url: http://localhost:11434
  model: nomic-embed-text
  dims: 768
```

**OpenAI:**
```yaml
embeddings:
  provider: openai
  model: text-embedding-3-small
  dims: 1536
  api_key: sk-...
```

---

## ⚡ Performance

Benchmarked on macOS (Apple Silicon), SQLite WAL mode, cold process start per operation:

* `store` (new): **24-57 ms** (includes dedup check)
* `search` hybrid (RRF): **~40 ms**
* Context assembly (token budget packing): **< 1 ms**
* Binary size: **~12 MB**
* Runtime dependencies: **0**

---

## 📊 Retrieval Quality Benchmarks

mnemos ships with a built-in benchmark suite that measures retrieval quality across five simulation scenarios — no LLM calls, no external services, fully reproducible.

Run it yourself:

```bash
go build -tags benchmark -o mnemos-bench ./cmd/mnemos
./mnemos-bench benchmark run
```

Results on the built-in scenarios (FTS5 only, `embedding.NoopProvider`):

```
┌──────────────────────────────────────────────────────────────────────┐
│  mnemos Retrieval Quality Benchmark                                  │
│                                                                      │
│  Precision@K and F1 across session scenarios                         │
│                                                                      │
│  cold-start-to-warm (20 sessions)                                    │
│  F1:  0.37 (steady @s1)                                              │
│  P:   0.37  R: 0.37                                                  │
│  Eff: 6% of store                                                    │
│                                                                      │
│  mistake-prevention (5 sessions)                                     │
│  F1:  0.00 (steady @s1)                                              │
│  P:   0.00  R: 0.00                                                  │
│  Eff: 0% of store                                                    │
│  MPR: 100%                                                           │
│                                                                      │
│  context-precision (10 sessions)                                     │
│  F1:  0.00 (steady @s1)                                              │
│  P:   0.00  R: 0.00                                                  │
│  Eff: 0% of store                                                    │
│                                                                      │
│  cross-session-transfer (10 sessions)                                │
│  F1:  0.20 (steady @s1)                                              │
│  P:   0.30  R: 0.15                                                  │
│  Eff: 11% of store                                                   │
│                                                                      │
│  correction-supersedes (10 sessions)                                 │
│  F1:  0.20 (steady @s1)                                              │
│  P:   0.30  R: 0.15                                                  │
│  Eff: 11% of store                                                   │
│  MPR: 100%                                                           │
│  CorrRate: 30%                                                       │
└──────────────────────────────────────────────────────────────────────┘
```

**What the metrics mean:**
- `F1` — harmonic mean of precision and recall (higher = better retrieval)
- `P / R` — precision and recall against declared ground-truth memory IDs
- `Eff` — fraction of total store tokens selected (lower = more selective)
- `MPR` — Mistake Prevention Rate: fraction of known-wrong memories kept out of context
- `CorrRate` — fraction of tasks where corrective memories ranked above the gotcha they supersede

Save results as JSON for later comparison:

```bash
./mnemos-bench benchmark run --output results.json
./mnemos-bench benchmark report --input results.json
```

---

## 🌐 REST API

Need to connect Mnemos to something else? Run it as a REST API:

```bash
mnemos serve --rest --port 8080
```
*(Supports standard `GET`, `POST`, `PATCH`, `DELETE` on `/memories`, `/search`, `/stats`, etc.)*

---

## 🏗️ Build from Source

```bash
git clone https://github.com/s60yucca/mnemos
cd mnemos
make build    # → bin/mnemos
```

## 📜 License
MIT
