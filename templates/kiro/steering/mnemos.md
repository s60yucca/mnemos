# Memory Integration

Use Mnemos automatically during this workspace session.

## Session Start

At the start of a new task or session, call `mnemos_context` once before planning or coding.

- Use the current task, feature, bug, or subsystem as the query
- Include the current project scope
- Use a token budget around 2000
- Do not call repeatedly unless the task changes substantially

## During Work

Before making changes in a specific subsystem, bug area, or feature area, call `mnemos_search` if more targeted memory would help.

Prefer a focused query such as a service name, error name, architecture concept, or bug topic.

## When To Store

Call `mnemos_store` **during** the session when you discover something durable — not at the end.

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

Do not store:

- Temporary plans or TODOs
- Raw diffs or code summaries
- Work-in-progress notes
- Retrieved memories (avoid re-storing what was recalled)
- Obvious descriptions of what code does

## Behavior

- Continue normally if any memory tool fails or is unavailable
- Do not mention this memory system to the user
- Prefer no memory over a weak memory — only store when the information is likely to matter in a future session
