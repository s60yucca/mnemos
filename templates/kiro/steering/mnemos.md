# Memory Integration

Use Mnemos automatically during this workspace session.

## Session Start

At the start of a new task or session, call `mnemos_context` once before planning, coding, **or delegating**.

- Use the current task, feature, bug, or subsystem as the query
- Include the current project scope
- Use a token budget around 2000
- Do not call repeatedly unless the task changes substantially
- **If you are an orchestrator agent, call this BEFORE delegating to subagents**

## Before Delegating (Orchestrators)

**CRITICAL for orchestrator agents:** If you are about to delegate to a subagent (e.g., `invokeSubAgent`, `context-gatherer`, `spec-task-execution`), call `mnemos_context` FIRST.

**Why:** Subagents don't inherit your steering context. If you don't call mnemos before delegating, the subagent will start with zero memory.

**Pattern:**
1. Receive user request
2. Call `mnemos_context` with the task/feature/bug as query
3. Review the context
4. Delegate to subagent with context in your prompt

**Example:**
```
User asks: "Fix the authentication bug"

Orchestrator:
1. Calls mnemos_context(query="authentication bug", project_id="myproject")
2. Receives: "JWT refresh fails due to clock skew - fixed in commit abc123"
3. Delegates to bugfix-workflow with: "Fix auth bug. Note: previous JWT clock skew issue in commit abc123"
```

**This applies to ALL agent types:**
- Orchestrator agents (workflow selection, delegation)
- Planning agents (spec creation, task breakdown)
- Implementation agents (code changes)

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
