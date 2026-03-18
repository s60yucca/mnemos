## Memory (mnemos)

You have access to a persistent memory engine via MCP tools. Use it automatically — do not wait for the user to ask.

### Rules

**At the start of every session:**
Call `mnemos_context` to load relevant memories before doing any work:
- query: a short description of what the user is asking about
- project_id: "mnemo-dev"
- max_tokens: 2000

**While working:**
Call `mnemos_store` whenever you learn something worth remembering:
- Architecture decisions
- Bug fixes and their root causes
- Tech stack details (libraries, versions, patterns)
- User preferences and conventions
- Anything that would be useful to know next session

**At the end of a session (when user says done / goodbye / thanks):**
Call `mnemos_store` to save a brief summary of what was accomplished.

### Tool usage

Always pass `project_id: "mnemo-dev"` on every call.

```
mnemos_context  → load context at session start
mnemos_store    → save important findings
mnemos_search   → search when you need to recall something specific
mnemos_relate   → link related memories (e.g. a bug depends_on an architecture decision)
mnemos_maintain → run occasionally to keep memory clean (decay + GC)
```

### What to store

Store things like:
- "Auth uses JWT RS256, tokens expire in 1h, refresh handled by middleware"
- "Fixed infinite loop in decay engine — was caused by missing nil check on LastAccessedAt"
- "User prefers table output over JSON for CLI commands"
- "SQLite WAL mode, max 1 writer, embeddings disabled by default"

Do not store:
- Temporary task state
- Things already in the codebase (code speaks for itself)
- Obvious facts
