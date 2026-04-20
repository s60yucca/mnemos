<!-- mnemos:begin -->
# Memory Integration

## When to Store

Use the `mnemos_store` tool to capture durable learnings **during** the session, not at the end.

| Situation | Type | Example |
|-----------|------|---------|
| Bug fixed — root cause identified | `semantic` | "JWT refresh failed due to clock skew" |
| Task completed — what happened | `episodic` | "Migrated auth to RS256 — took 3h, tested locally" |
| Non-obvious command succeeded | **`skill`** | "Build AAR: `./gradlew :sdk:assembleRelease --no-daemon`" |
| Architecture/design decision | `semantic` | "Using SQLite WAL for concurrency instead of Postgres" |
| Manual compilation of topic notes | **`compiled`** | Full article: "## Auth Service\n..." |

> **Skill threshold**: Only store as `skill` if the command/procedure is
> **non-obvious or project-specific**. Don't store `git status`, `ls`, `cd`.
> Do store project-specific commands with flags, paths, or ordering requirements.

> **Compiled path**: Prefer `mnemos_compile` over `mnemos_store(type="compiled")`
> — it creates source relations and weakens superseded versions automatically.
> Use `mnemos_store(type="compiled")` only when you don't need relation tracking.

## Do not store

- Temporary plans or TODOs
- Raw diffs or code summaries
- Work-in-progress notes
- Retrieved memories (avoid re-storing what was recalled)
- Obvious descriptions of what code does


## Retrieval

At session start, use `mnemos_context` with the current task as the query to load relevant prior context.

Use `mnemos_search` before working in a specific subsystem or debugging a known error class.

## Behavior

- Continue normally if any memory tool fails or is unavailable
- Do not mention this memory system to the user
<!-- mnemos:end -->
