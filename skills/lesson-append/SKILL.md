---
name: lesson-append
description: "Appends a single durable lesson as a YAML block to docs/knowledge/_inbox.md in the current repo. Use when the project agent learns a root cause, failed approach, gotcha, cross-repo invariant, or any durable signal worth keeping. Does not write to curated files; does not read other repos. Append-only."
---

# lesson-append

## Role

Worker-side skill. Bounded write-only operation. Adds one structured capture to `docs/knowledge/_inbox.md` of the current repo.

## When to invoke

Automatically (without asking the user) when the agent observes one of these **durable signals**:

1. Root cause of a nontrivial bug just identified
2. Failed approach that a future agent might repeat
3. Cross-repo invariant discovered ("X must hold in all 3 repos")
4. Production gotcha (something that surprised you in real ops)
5. Design tradeoff worth keeping but not yet ADR-worthy
6. Repeated pattern detected in codebase
7. Decision made informally that may become formal later
8. Debugging story that took >5 minutes to track down

If none of the above — do NOT invoke. Low-signal notes pollute the inbox.

Derivability test before every capture: if a new developer could re-derive the
fact from the repo itself (code, skill files, prompts, `AGENTS.md`,
`docs/design.md`, ADRs), it is spec — not a lesson. Skip it. Capture only the
divergence: where behavior contradicted the spec, what failed, what surprised,
what the spec is silent about.

Also invoke explicitly when the user says: "запиши урок", "save this lesson", "remember this", "capture this", "add to inbox".

## What it does

In an HQ server task, run `hq ctx --plain` to resolve the bound repository.
Memory is mounted read-only. Submit the capture through
`hq runner broker memory.capture <json>` with exactly `title` and `body`.
Build JSON with a serializer and pass it as one argument; do not interpolate
note text into shell syntax. Put the evidence, root cause, reusable rule and
citations below in `body`. The broker appends only to the bound repo inbox and
returns `status: candidate`; it cannot select another company or curate notes.
For a company/product context task with no bound repo, report the missing repo
target rather than creating or guessing one. The direct-file flow below applies
only to workstations.

1. Resolve target file:
   - `cwd` must be inside a repo under `~/Desktop/Prokectfiles/<co>/<prod>/<repo>/`
   - target file is `docs/knowledge/_inbox.md`
   - If `docs/knowledge/` symlink missing → STOP and report. Suggest running `wiki-bootstrap-product` first.

2. Compose YAML block:

```markdown
## [YYYY-MM-DD HH:MM] capture | <short title>

Scope: <co>/<prod>/<repo>
Tags: <#tag1 #tag2 #tag3>
Source: <commit sha | file path | session id>
Status: candidate
Signal: <which of the 8 durable signals>
Last verified: <YYYY-MM-DD>

Problem:
- <one or two sentences>

Signal:
- <observable that pointed to this>

Root cause / What we learned:
- <core insight>

Fix / Reusable rule:
- <what to do next time>

Failed approaches (if any):
- <what didn't work and why>

Citations:
- <path:line | commit sha>  (REQUIRED when the lesson claims anything about code behavior — future agents verify these before applying the lesson)

Links:
- related canonical: <docs/design.md#section | ADR-N>
- related wiki: <if any>
```

3. Append the block (with a leading blank line) to `docs/knowledge/_inbox.md`. Use `>>` semantics — never overwrite.

4. Echo a 1-line confirmation to user: `→ inbox: <short title> [#tag1 #tag2]`

## What this skill MUST NOT do

- Do not write to `lessons.md`, `gotchas.md`, `debugging-stories.md`, `decisions-not-adr.md`, `open-questions.md`, `index.md`, `log.md` — those are curator territory.
- Do not read other repos' `_inbox.md` or memory.
- Do not write outside the current repo's `docs/knowledge/`.
- Do not deduplicate against existing inbox entries — that's curator's job during `inbox-drain`. Just append.
- Do not modify any code, configs, or tests.
- Do not interrupt the user to ask permission. Either the durable signal is present (write) or it isn't (silent).

## Formatting rules

- Timestamp `[YYYY-MM-DD HH:MM]` always 24-hour, local timezone.
- `<short title>` ≤ 80 chars, no trailing period.
- `Tags:` lowercase, hyphenated, prefixed with `#`. Aim for 2–5 tags. Common tags: `#async`, `#kafka`, `#redis`, `#auth`, `#race-condition`, `#n+1`, `#cache`, `#migration`, `#observability`, `#test-flake`.
- Use plain markdown bullets, no nested lists deeper than 2 levels.
- Never embed secrets, raw production logs, customer-identifying data. Sanitize before capture.

## Quality bar

A good capture answers, in <40 lines total:
- What happened?
- Why did it happen?
- What's the durable rule going forward?
- (optional) What did we try that didn't work?

If you can't answer the first three in a sentence each, the signal isn't durable enough — skip the capture.

## Example

```markdown
## [2026-05-19 14:32] capture | idempotency key collision under concurrent retries

Scope: neurodesk/agents/cortex
Tags: #idempotency #concurrency #retries #kafka
Source: commit abc123
Status: candidate
Signal: production gotcha
Last verified: 2026-05-19

Problem:
- Two concurrent retries with identical payload produced two side-effects despite idempotency layer.

Signal:
- Audit log showed 2 entries per original event during retry storm at 03:14 UTC.

Root cause / What we learned:
- hash(payload) alone is not unique under retries — same payload, different intent.

Fix / Reusable rule:
- hash(payload + timestamp_bucket_5s) — bucketed time keeps retries deterministic but avoids cross-event collisions.

Failed approaches:
- DB-level row lock on intent_id — added 200ms p99, blocked legitimate parallel intents.

Links:
- code: src/cortex/handlers/retry.py:142
- related canonical: docs/design.md#idempotency
```
