---
name: source-ingest
description: "Ingest an external source, URL, file, or pasted note into WikiPedik source notes. Use for curator-side raw source capture, not repo memory curation."
---

# source-ingest

Curator-side skill. Adds external reference material to WikiPedik without mixing it into project canonical docs.

## Target

In an HQ server Research task, use `sources/<source-slug>.md` under the
current research workspace. Resolve the scope with `hq ctx --plain` when
available. Do not use the workstation paths below inside a container, follow
symlinks outside the workspace, or request company credentials. Company/project
association is a reference for later owner review, never a cross-scope write.

Workstation default target:

```text
~/Desktop/WikiPedik/dev/10-wiki/sources/<source-slug>.md
```

For company/product-specific sources, use:

```text
~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/sources/<source-slug>.md
```

only when the user explicitly scopes the source to that product.

## Workflow

1. Read the source:
   - local file path;
   - URL via available web tools;
   - pasted text.
2. Summarize before writing:
   - 3-5 key takeaways;
   - entities/concepts introduced;
   - likely project/product relevance;
   - confidence and source quality.
3. Ask the user what to emphasize if the source is broad or ambiguous.
4. Write a source note with frontmatter and a concise body.
5. Link to related project memory only as references; do not edit repo memory directly.

## Source Note Template

```markdown
---
type: source
source_kind: url|file|text
title: "<title>"
url: "<url if any>"
created: YYYY-MM-DD
confidence: low|medium|high
scope: general|<co>/<prod>
---

# <title>

## Summary

## Key Takeaways

## Relevant Concepts / Entities

## Possible Project Links

## Open Questions
```

## Boundaries

- Do not ingest secrets, credentials, raw production logs, or customer-identifying data.
- Do not write `lessons.md`, `gotchas.md`, or `_inbox.md`; use `inbox-drain` / `lesson-append` for project memory.
- Do not create canonical docs or ADRs from this skill.
