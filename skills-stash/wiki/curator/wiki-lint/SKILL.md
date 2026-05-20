---
name: wiki-lint
description: Audit WikiPedik project memory for broken links, stale lessons, privacy leaks, duplicate captures, and promotion candidates. Use for curator-side wiki health checks.
---

# wiki-lint

Curator-side skill. Audits WikiPedik memory and writes a health report when requested.

## Scope

Resolve one of:

```text
~/Desktop/WikiPedik/dev/20-projects/<co>
~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>
~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/repos/<repo>
```

Never cross company boundaries unless the user explicitly names multiple companies.

## Checks

Errors:
- broken wikilinks to missing pages;
- missing expected repo memory files;
- `docs/knowledge` symlink points outside the expected company/product;
- possible secrets: API keys, tokens, `.env` values, private keys;
- cross-company references.

Warnings:
- `_inbox.md` has more than 10 candidate entries;
- stale entries with `Last verified` older than 90 days;
- duplicate headings/tags across lessons and gotchas;
- wiki memory contradicts `docs/design.md`, `docs/ARCHITECTURE.md`, or ADRs;
- product/company memories look writable from worker flow without clear guardrails.

Info:
- repeated lessons that should be synthesized to product/shared;
- decisions-not-ADR that look binding enough for ADR promotion;
- orphan pages not linked from `index.md`.

## Output

Report in the terminal first:

```markdown
# Wiki Lint: <scope>

## Summary
- Errors: N
- Warnings: N
- Info: N

## Findings

## Suggested Fixes

## Promotion Candidates
```

If the user asks to save the report, append a concise entry to:

```text
<scope>/health.md
```

and append an event to the nearest `log.md`.

## Boundaries

- Do not modify curated memory unless the user confirms the exact fixes.
- Do not modify canonical repo docs.
- Do not delete inbox entries.
- Redact suspected secrets in reports.

