---
name: wiki-context-pack
description: "Build a compact WikiPedik memory context pack for the current repo/product before planning. Use for nontrivial debug, design, migration, or cross-repo work."
---

# wiki-context-pack

Worker-side read-only skill. It gathers a small, relevant memory slice from WikiPedik symlinks inside the current repo.

## Scope Resolution

Run from a repo under:

```text
~/Desktop/Prokectfiles/<co>/<prod>/<repo>
```

Expected symlinks:

```text
docs/knowledge
docs/product-knowledge
docs/company-knowledge
```

If they are missing, report that `wiki-bootstrap-product <co> <prod> <repo>` is needed.

## Read Order

1. `docs/knowledge/hot.md`
2. `docs/knowledge/index.md`
3. `docs/product-knowledge/index.md`
4. `docs/product-knowledge/shared/patterns.md`
5. `docs/product-knowledge/shared/gotchas.md`
6. `docs/company-knowledge/privacy.md`
7. `docs/company-knowledge/shared/patterns.md` only if the task is cross-product or company-level

For debug/design/migration tasks, also run focused `rg --follow` over:

```text
docs/knowledge docs/product-knowledge docs/company-knowledge
```

using task keywords, error names, subsystem names, and tags.

## Output

Return a compact context pack:

```markdown
## Wiki Memory Context

Scope: <co>/<prod>/<repo>

### Hot
- ...

### Relevant Lessons
- <file#anchor>: <lesson>

### Product Patterns
- ...

### Gotchas / Open Questions
- ...

### Constraints
- Canonical docs still win over wiki memory.
- If wiki conflicts with docs/design.md or ADRs, treat wiki as stale.
```

## Boundaries

- Read-only. Do not write memory from this skill.
- Do not read another company namespace.
- Do not full-scan the vault unless the user explicitly asks.
- Do not treat wiki memory as canonical architecture.

