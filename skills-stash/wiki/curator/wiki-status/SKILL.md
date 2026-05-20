---
name: wiki-status
description: "Show WikiPedik project-memory status: inbox counts, curated pages, synthesis candidates, stale health files, and recent logs. Use when the user asks for wiki status, memory health, or pending wiki work."
---

# wiki-status

Curator-side read-mostly skill. Produces a terminal report and optionally appends a short summary to `health.md` when the user asks.

## Scope

Resolve one of:
- `<co>`
- `<co>/<prod>`
- `<co>/<prod>/<repo>`
- current path under `~/Desktop/WikiPedik/dev/20-projects/`

Never cross company boundaries unless the user explicitly names multiple companies.

## What To Report

For each scoped product/repo:
- `_inbox.md` candidate/drained/rejected counts;
- latest entries in `lessons.md`, `gotchas.md`, `debugging-stories.md`, `decisions-not-adr.md`, `open-questions.md`;
- `_synthesis-candidates.md` count;
- last log event from `log.md`;
- `health.md` last lint marker;
- missing expected files or broken symlinks.

## Output Format

```markdown
# Wiki Status: <scope>

## Overview
- Repos: N
- Inbox candidates: N
- Curated lessons: N
- Gotchas: N
- Open questions: N
- Synthesis candidates: N

## Pending Work
- ...

## Recent Activity
- ...

## Health
- ...

## Recommendation
- Run `wiki sync <scope>` if inbox candidates exist.
- Run `wiki-synthesize <scope>` if synthesis candidates exist.
- Run `wiki-lint <scope>` if health is stale.
```

## Boundaries

- Do not modify wiki files unless the user explicitly asks to append a status summary.
- Do not perform lint, sync, or synthesis from this skill.
- Do not read raw session histories; use `agent-history-ingest` for that.
