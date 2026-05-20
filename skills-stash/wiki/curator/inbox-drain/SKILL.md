---
name: inbox-drain
description: Drains candidate YAML captures from repo-level _inbox.md files into the right curated wiki pages (lessons.md, gotchas.md, debugging-stories.md, decisions-not-adr.md, open-questions.md). Interactive — proposes target for each entry, awaits confirmation. Updates index.md and log.md. Use when the user asks to "drain inbox", "wiki sync", "synthesize inbox", or when inbox grows beyond ~10 entries.
---

# inbox-drain

## Role

Curator-side skill. Block-level operation on `_inbox.md` — moves candidate entries to their right curated home.

This skill is the **bridge between worker captures and curated wiki knowledge.**

## When to invoke

Explicitly only. Triggers:

- User says "drain inbox", "wiki sync", "разгреби inbox", "process captures"
- Curator notices `_inbox.md` > 10 entries (suggests it to user, doesn't act unprompted)
- Before sprint planning / product architecture review
- After complex debugging week

Do NOT invoke this skill automatically. It writes to curated files.

## Scope resolution

Required input:
- `<co>/<prod>/<repo>` triple — process this repo's inbox
- OR `<co>/<prod>` — process all repos in product
- OR `<co>` — process all repos in company

Resolution paths:
```
~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/repos/<repo>/_inbox.md
```

If `<repo>` not specified, iterate over all `repos/*/`.
Never cross company boundary. If wrong company name → STOP.

## Workflow

### Phase 1: Read inbox

Parse each `_inbox.md` (or all of them if product/company scope). Extract YAML blocks bounded by `## [` timestamps.

For each candidate entry, build:
```
{
  timestamp: ...,
  title: ...,
  scope: ...,
  tags: [...],
  status: candidate | drained | rejected,
  signal: ...,  // which of the 8 durable signals
  problem, root_cause, fix, failed_approaches, links: ...
}
```

Skip entries already marked `Status: drained` or `Status: rejected`.

### Phase 2: Deduplicate

For each entry, check if it duplicates or extends existing curated content:
- Check `lessons.md`, `gotchas.md`, `debugging-stories.md` headings + tags
- If high similarity (same tags + similar title) → propose merge

### Phase 3: Propose targets

For each candidate, decide target file by signal type:

| Signal type | Target file |
|---|---|
| Root cause of bug + debugging narrative | `debugging-stories.md` |
| Reusable rule / learned principle | `lessons.md` |
| Counter-intuitive trap | `gotchas.md` |
| Failed approach worth not repeating | `gotchas.md` |
| Cross-repo invariant | `lessons.md` (single repo) OR flag for `wiki-synthesize` (cross-repo) |
| Decision pre-ADR | `decisions-not-adr.md` |
| Open question | `open-questions.md` |
| Repeated pattern | flag for `wiki-synthesize` (product/shared) |
| Production gotcha | `gotchas.md` |

Present to user, one entry at a time:

```
[3/12] Entry: "idempotency key collision under concurrent retries"
       Signal: production gotcha
       Suggested target: gotchas.md
       Existing similar entry: NONE
       
       [a]ccept  [t]arget — choose different file  [m]erge — into existing  [r]eject  [s]kip  [q]uit
       Choice: _
```

### Phase 4: Apply

For each accepted entry:

1. Append structured content to target file (transform YAML → markdown section):

```markdown
## <short title>

**Date:** <YYYY-MM-DD>
**Scope:** <repo>
**Tags:** <tags>
**Status:** validated
**Last verified:** <today>
**Source:** <commit | session>

**Problem:** ...
**Root cause:** ...
**Fix:** ...
**Failed approaches:** ...
**Links:** ...
```

2. Update target file's local index/TOC if it has one.

3. Mark inbox entry as drained — change `Status: candidate` to `Status: drained` AND prepend `~~` strikethrough to the timestamp heading. Do NOT delete the entry immediately; keep history.

4. Update `docs/knowledge/index.md` — add line to "Recent" section.

5. Append to `docs/product-knowledge/log.md`:
   ```
   ## [YYYY-MM-DD HH:MM] drain | <repo> | <title>
   Target: <target file>#<anchor>
   Tags: ...
   ```

### Phase 5: Mark rejected

Rejected entries: change `Status: candidate` to `Status: rejected` + add `Rejected reason: <user reason>`.

Keep in inbox for audit. Don't delete.

### Phase 6: Cross-repo flag

If any entry was flagged for `wiki-synthesize` (pattern across repos), append to:
```
~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/_synthesis-candidates.md
```

This is input for `wiki-synthesize` skill to process next.

### Phase 7: Summary report

After loop:

```
Inbox drain complete.

Processed: 12 entries
  → lessons.md: 4
  → gotchas.md: 5
  → debugging-stories.md: 1
  → decisions-not-adr.md: 1
  → open-questions.md: 0
  → synthesis-candidates.md: 1 (cross-repo, needs wiki-synthesize)
  Rejected: 0
  Skipped: 1

Updated:
  - <list of repos' index.md updated>
  - <product>/log.md (1 event)

Next: run wiki-synthesize for cross-repo patterns.
```

## What this skill MUST NOT do

- Do not delete inbox entries — mark as drained/rejected, keep history.
- Do not invent content not in the inbox entry. Preserve what worker captured.
- Do not write to `product/shared/*` or `company/shared/*` directly. Cross-cutting promotion is the job of `wiki-synthesize`.
- Do not cross company boundary.
- Do not skip user confirmation. This is interactive by design.
- Do not modify code or canonical docs in repos.

## Notes

- Inbox entries that fail signal criteria (low-signal noise that worker shouldn't have captured): reject with reason "below signal threshold". Useful feedback for tuning the `lesson-append` skill in workers.
- If an entry references code that no longer exists (file deleted, function renamed), still curate but mark `Status: validated-but-historical` and note in `Last verified`.
- `_archive/` folder receives inbox files after they exceed N drained/rejected entries — but that's `wiki-rebuild` job, not this skill.
