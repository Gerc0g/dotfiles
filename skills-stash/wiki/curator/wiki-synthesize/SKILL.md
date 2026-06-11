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

## Rule Promotion (память → поведение)

When a pattern or lesson is BOTH binding (must-follow, not advice) AND
attachable to concrete code paths, propose promoting it to a path-scoped rule.
Rules are the ladder step between shared patterns and ADR.

Where rules live (source of truth, synced into repos by `wiki-rules-sync`):
- repo rule: `repos/<repo>/rules/<slug>.md`
- product rule (binding for all repos of the product): `shared/rules/<slug>.md`

Rule file template — `paths:` frontmatter is MANDATORY (a rule without it
would load into every session and bloat context; rules-sync refuses such
files):

```markdown
---
description: <one line, imperative>
paths: ["src/metrics/**", "**/prometheus*"]
---

# <rule title, imperative>

<2-6 lines: the binding rule, why it exists, what breaks otherwise.>

Source: [[lessons#<anchor>]] / [[../shared/patterns#<anchor>]]
Last verified: YYYY-MM-DD
```

Promotion bar (ALL must hold):
1. The lesson recurred or caused real damage at least twice, OR the user explicitly asks.
2. It is a must-follow constraint, not a preference.
3. It maps to concrete file globs.
4. User confirms the promotion.

After writing a rule file, tell the user to run `wiki rules-sync <co>/<prod>`
and `wiki-hot-refresh <co>/<prod>/<repo>` (the rule digest goes into hot.md
for codex). If a rule later becomes canonical, promote to ADR and turn the
rule body into a pointer.

## Boundaries

- Do not invent a product rule from one repo only.
- Do not write company-level memory; company promotion is a separate explicit step.
- Do not modify canonical repo docs or ADRs.
- If a pattern becomes binding, suggest ADR promotion instead of treating wiki as canonical truth.
- Never create a rule file without `paths:` frontmatter and user confirmation.

