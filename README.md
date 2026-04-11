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

---

## ⚡ Quick Start

Get up and running in 30 seconds:

```bash
# 1. Install via curl (macOS/Linux)
curl -fsSL https://raw.githubusercontent.com/mnemos-dev/mnemos/main/install.sh | bash

# Or via Homebrew (macOS)
brew install s60yucca/tap/mnemos

# 2. First-time setup
mnemos init

# 3. Start the MCP server (runs in background/stdio)
mnemos serve
```

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

## 🌐 REST API

Need to connect Mnemos to something else? Run it as a REST API:

```bash
mnemos serve --rest --port 8080
```
*(Supports standard `GET`, `POST`, `PATCH`, `DELETE` on `/memories`, `/search`, `/stats`, etc.)*

---

## 🏗️ Build from Source

```bash
git clone https://github.com/mnemos-dev/mnemos
cd mnemos
make build    # → bin/mnemos
```

## 📜 License
MIT
