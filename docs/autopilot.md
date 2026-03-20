# Mnemos Autopilot

Mnemos does not control the agent. It is an MCP server. That means automatic memory usage only happens when the MCP client or agent instructions consistently trigger Mnemos at the right times.

This document defines a practical "autopilot" pattern for Claude Code and Kiro.

## Goal

Make Mnemos run with minimal human reminding:

1. load memory at session/task start
2. search memory before coding when the task is specific
3. store durable learnings after meaningful work
4. avoid storing noise

## What Mnemos Can And Cannot Do

Mnemos can:

- expose tools like `mnemos_context`, `mnemos_search`, and `mnemos_store`
- expose prompts like `load_context` and `save_session`
- return relevant memory when asked

Mnemos cannot:

- force Claude Code or Kiro to call those tools
- detect task boundaries on its own
- automatically store memory without a client-side instruction or hook

So "autopilot" must be implemented as client steering plus a simple usage policy.

## Core Policy

Use the same policy in both Claude Code and Kiro:

### At session start

- call `mnemos_context` once with a broad project/task query
- use a moderate token budget like `1500-3000`

### Before editing code

- if the task is concrete, call `mnemos_search` with the specific subsystem or error
- only call again when the task changes meaningfully

### After meaningful completion

Call `mnemos_store` only for durable information such as:

- architecture decisions
- bug root causes
- project conventions
- deployment or environment quirks
- non-obvious implementation constraints

Do not store:

- temporary plans
- raw diffs
- obvious summaries of code that can be re-read cheaply
- task chatter

## Claude Code

Claude Code generally needs stronger session instructions than Kiro. The practical approach is:

1. install Mnemos as an MCP server
2. add a reusable session instruction block
3. optionally keep a project-local prompt file checked into the repo

Recommended behavior:

- session open: call `mnemos_context`
- before implementation: call `mnemos_search` if the task references a subsystem, feature, bug, or file area
- session end or after major step: call `mnemos_store` once if there is a durable learning

Use the template in [templates/claude/SYSTEM_PROMPT.md](../templates/claude/SYSTEM_PROMPT.md).

## Kiro

Kiro is a better fit for autopilot because steering files are first-class.

Recommended setup:

1. install Mnemos as an MCP server
2. enable auto-approve for read-oriented tools
3. add a steering file to the workspace

Suggested auto-approve set:

- `mnemos_context`
- `mnemos_search`
- `mnemos_get`

Keep `mnemos_store` manual at first if you want to watch write quality. After the prompts are stable, auto-approve writes too.

Use the template in [templates/kiro/steering/mnemos.md](./templates/kiro/steering/mnemos.md).

## Recommended Defaults

Use these defaults first:

- session-start context budget: `2000`
- search limit: `5-8`
- store only once per meaningful completed change
- project scope: set `MNEMOS_PROJECT_ID` per workspace

## Phase Plan

### Phase A: Prompt Autopilot

- ship templates for Claude Code and Kiro
- improve README setup instructions
- make Mnemos easy to include in project setup

### Phase B: Safer Writes

- strengthen store heuristics in prompts
- prefer one high-quality stored memory over many low-value ones
- optionally add a `mnemos_should_store` helper tool later

### Phase C: Better Retrieval

- add file/symbol-aware retrieval only after autopilot usage is working
- add session/diff-oriented context assembly later

## Product Direction

The immediate product problem is not richer storage. It is reliable invocation.

If you fix autopilot first, everything else compounds:

- more memory captured
- better future retrieval
- better benchmark story
- less user reminding

If you skip autopilot and keep adding schema/features, Mnemos stays impressive but underused.
