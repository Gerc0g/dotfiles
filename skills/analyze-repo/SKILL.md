---
name: analyze-repo
description: "Deep architectural analysis of a single repo. Reads source code to derive module structure, design patterns, external integrations, key abstractions, non-obvious conventions. Writes draft to docs/design.md and proposed docs/adr/. Use when: onboarding new repo, after major refactor, deepening AGENTS.md after `onboard` finished basic metadata. Triggers: \"analyze repo\", \"разобрать репо\", \"проанализируй код\", \"что за репозиторий\"."
---

# Analyze Repo Architecture

Generates **repo-level design documentation** by reading source code in detail.

## When to trigger

- User just cloned/created repo and wants deep dive
- After `onboard` filled basic metadata — to add architectural depth
- After major refactor — re-sync design.md with reality
- User unfamiliar with codebase wants summary

## Prerequisites

cwd must be a **repo root** (has `.git/` or manifest file). If detected as
product/company level — STOP, suggest `analyze-product` instead.

## Workflow

Read `analysis-plan.md` for detailed checklist.

High-level flow:

1. **Inventory.** Manifest, README, Dockerfile, Makefile/scripts, entry point,
   tests structure, .env.example, CI config.

2. **Map module structure.** Walk `src/` (or repo's source root). Identify
   directories + their responsibilities by reading 1-2 files per dir.

3. **Identify entry points.** Where execution starts:
   - HTTP routes / handlers
   - CLI commands
   - Background workers / consumers
   - Scheduled tasks

4. **Trace external integrations.** From code:
   - HTTP clients → which services
   - DB clients → which databases / queries patterns
   - Message bus → producers/consumers, topics
   - Caches, queues, external APIs

5. **Identify key abstractions.**
   - Custom types (Result, Either, Maybe patterns)
   - Base classes / interfaces / protocols
   - Repository / factory / decorator patterns
   - DI containers
   - Error handling style

6. **Find non-obvious conventions** by analyzing recent commits:
   - `git log --oneline -50` → look for "fix" commits
   - Read the fix diffs → identify patterns the codebase enforces
   - Look for comments like `# IMPORTANT`, `# NOTE`, `# WARNING`

7. **Identify hot paths.** Most-called endpoints, most-imported modules,
   performance-critical code.

8. **Analyze tests structure.** Unit / integration / e2e split, mocking
   patterns, fixtures.

9. **Draft `docs/design.md`** (in repo cwd). Structure:
   - Purpose (1 paragraph)
   - Architecture style (layered / hexagonal / function-based)
   - Module structure (tree + responsibilities)
   - Entry points
   - External dependencies (table)
   - Key abstractions (with code examples)
   - Non-obvious conventions (the "why" behind patterns)
   - Hot paths
   - Tests architecture
   - Known issues

10. **Identify candidate ADRs** for repo-level decisions:
    - "Why Result type instead of exceptions"
    - "Why this DB schema choice"
    - "Why no ORM" / "Why this ORM"
    - Propose `docs/adr/0001-<slug>.md` files

11. **Optionally update repo AGENTS.md `## Non-obvious patterns` section**
    with top 3-5 most-important patterns from Phase 6. ASK user before editing
    AGENTS.md.

12. **Show drafts to user.** Don't auto-save. Ask:
    - "Save docs/design.md as-is? (y/n/edit)"
    - "Create N ADRs? (y/n/list)"
    - "Update AGENTS.md `## Non-obvious patterns` with these 3 items? (y/n)"

## Rules

- READ-ONLY by default. No edits to source code.
- Write only to `./docs/` directory and (with explicit ask) `./AGENTS.md`.
- Never `git commit` without explicit user ask.
- Be honest about uncertainty: mark inferences as "looks like X" or
  "appears to follow Y", suggest user confirms.
- For pattern detection — read enough code to be confident, not 1 example.
- Pair "I observed pattern X" with "user, confirm this is intentional?"

## Output structure

```
~/Desktop/Prokectfiles/<co>/<prod>/<repo>/
├── AGENTS.md            ← optionally updated (Non-obvious patterns section)
├── docs/                ← NEW
│   ├── design.md        ← main output
│   └── adr/
│       ├── 0001-<topic>.md
│       └── ...
├── src/                 ← unchanged (read-only)
└── tests/               ← unchanged (read-only)
```

## Quality bar

A good design.md answers, in <300 lines:
- "What does this repo do at a glance?"
- "How is the code organized and why?"
- "What external systems does it talk to?"
- "What are the 'gotchas' I need to know to contribute safely?"
- "Where do I look to understand X?"

If can't answer these — analysis is incomplete. Ask user follow-up questions.

## When to re-run

- After 50+ commits since last analysis (significant code drift)
- After adding new module / external integration
- Before architectural refactor (capture current state first)
