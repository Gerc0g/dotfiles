---
name: wiki-synthesize
description: Synthesize repeated lessons and gotchas across repos into product-level shared patterns. Use after inbox-drain flags synthesis candidates or when the user asks to find cross-repo patterns.
---

# wiki-synthesize

Curator-side skill. Finds repeated patterns across repo memories and promotes them into product shared memory.

## Scope

Primary target:
- `~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/`

Inputs:
- `<prod>/_synthesis-candidates.md`
- `<prod>/repos/*/lessons.md`
- `<prod>/repos/*/gotchas.md`
- `<prod>/repos/*/debugging-stories.md`

Outputs:
- `<prod>/shared/patterns.md`
- optionally `<prod>/shared/gotchas.md`
- backlinks in source repo lesson/gotcha files
- `<prod>/index.md`
- `<prod>/log.md`

## Workflow

1. Resolve `<co>/<prod>` from user input or current WikiPedik path.
2. Read synthesis candidates first; then scan repo memory files.
3. Group entries by shared tags, similar titles, repeated root causes, and repeated fix rules.
4. Promote only patterns that appear in at least 2 repos or were explicitly requested by the user.
5. Present proposed promotions before writing:
   - pattern title;
   - source repo entries;
   - target file;
   - confidence.
6. After approval, append a concise section to `shared/patterns.md`.
7. Add backlinks from source entries to the shared pattern.
8. Append an event to `<prod>/log.md`.

## Pattern Template

```markdown
## <pattern title>

**Status:** validated
**Last verified:** YYYY-MM-DD
**Seen in:** repo-a, repo-b
**Tags:** #tag-a #tag-b

**Pattern:**
- <what repeats>

**Reusable rule:**
- <what to do next time>

**Evidence:**
- [[repos/repo-a/lessons#anchor]]
- [[repos/repo-b/gotchas#anchor]]

**Promotion path:**
- If this becomes binding, draft an ADR in the affected repo/product.
```

## Boundaries

- Do not invent a product rule from one repo only.
- Do not write company-level memory; company promotion is a separate explicit step.
- Do not modify canonical repo docs or ADRs.
- If a pattern becomes binding, suggest ADR promotion instead of treating wiki as canonical truth.

