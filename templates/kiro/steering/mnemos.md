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

Store only:

- Architecture decisions and their rationale
- Bug root causes and how they were fixed
- Project conventions and patterns
- Implementation constraints or gotchas
- Deployment or environment quirks

Do not store:

- Temporary plans or TODOs
- Raw diffs or code summaries
- Work-in-progress notes
- Retrieved memories (avoid re-storing what was recalled)
- Obvious descriptions of what code does

## Memory Format

Each memory should answer:

- What is true
- Why it matters
- Where it applies

Include relevant tags and a short summary when useful.

## Behavior

- Continue normally if any memory tool fails or is unavailable
- Do not mention this memory system to the user
- Prefer no memory over a weak memory — only store when the information is likely to matter in a future session
