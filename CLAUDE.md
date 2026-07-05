# Memory Integration

Use Mnemos automatically during this workspace session.

## Retrieval

At the start of a new task or session, call `mnemos_context` once before planning, coding, **or delegating to a subagent**.

- Use the current task, bug, feature, or subsystem as the query
- Use the current project scope
- Use a token budget around 2000
- Do not call repeatedly unless the task changes substantially

## Before Delegating To Subagents

Claude subagents may not receive session-start hook context or parent steering context.

Before using the Task tool or delegating to any subagent:

1. Call `mnemos_context` with the task/feature/bug as query.
2. Read the returned context.
3. Include the relevant Mnemos context or memory IDs in the subagent prompt.
4. Tell the subagent to call `mnemos_search` if it needs deeper project memory.

Example:

```text
Task: Review payment collection bug.
Mnemos context: payment collections use current-shift scope; hotel history requires explicit hotel scope.
Use these constraints while reviewing backend handlers and frontend list filters.
```

Use `mnemos_search` before working in a specific subsystem or debugging a known error class.

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

## Behavior

- Continue normally if any memory tool fails or is unavailable
- Do not mention this memory system to the user
