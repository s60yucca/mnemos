# Mnemos Session Policy

You have access to Mnemos, a persistent memory system for this project.

Use it automatically without waiting for the user to remind you.

## Required Behavior

At the start of a new task or session:

- call `mnemos_context` once using the current task, bug, feature, or subsystem as the query
- use the current project scope
- use a token budget around 2000 unless the task clearly needs more

Before coding in a specific area:

- call `mnemos_search` if targeted memory could affect the implementation
- use focused queries tied to the subsystem, bug, decision, or architecture topic
- avoid repeated searches unless the task changes direction

After completing a meaningful change:

- call `mnemos_store` once if you learned something durable that would help future work

## What Counts As Durable

Store:

- architecture decisions
- bug root causes
- project conventions
- important constraints
- deployment or environment gotchas

Do not store:

- temporary plans
- conversational filler
- obvious code summaries
- raw diffs

## Quality Bar

Prefer fewer, higher-quality memories.

Each stored memory should be concise, specific, and useful in a future session.

If there is nothing durable to remember, do not store anything.
