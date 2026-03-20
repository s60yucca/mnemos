# Mnemos Steering

Use Mnemos automatically during this workspace session.

## Session Start

At the start of a new task or session, call `mnemos_context` once before planning or coding.

Guidelines:

- use the current task, feature, bug, or subsystem as the query
- include the current project scope
- use a moderate token budget such as 2000
- do not call repeatedly unless the task changes substantially

## During Work

Before making changes in a specific subsystem, bug area, or feature area, call `mnemos_search` if more targeted memory would help.

Prefer a focused query such as:

- service or subsystem name
- error name
- architecture concept
- bug topic

## When To Store

After a meaningful completed change, call `mnemos_store` once if you learned something durable that would help in a later session.

Store only:

- architecture decisions
- bug root causes
- project conventions
- deployment quirks
- important implementation constraints

Do not store:

- temporary plans
- trivial summaries
- work-in-progress notes
- obvious code descriptions

## Store Style

Keep memories compact and specific.

A good memory should explain:

- what is true
- why it matters
- where it applies

If useful, include tags and a short summary.

## Avoid Noise

Prefer no memory over a weak memory.

Only store when the information is likely to matter in a future session.
