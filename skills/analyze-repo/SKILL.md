---
name: analyze-repo
description: "Deep architectural analysis of a single repo when the user asks for analyze repo, разобрать репо, проанализируй код, or an architectural deep dive. Derive modules, integrations and conventions from source; draft design.md and save only within authorized scope. Separate from basic entity onboarding; ADR creation is optional."
---

# Analyze Repo Architecture

Generates **repo-level design documentation** by reading source code in detail.

## When to trigger

- User explicitly wants a deep dive into a cloned/created repo
- User requests architectural depth after basic onboarding
- After major refactor — re-sync design.md with reality
- User unfamiliar with codebase wants summary

## Resolve the analysis scope

Deep analysis is separate from basic context filling. Do not run it merely
because AGENTS.md or design.md is missing; `onboard-agents-md` can use available
facts without generating architecture docs.

1. Use an explicit HQ task scope when supplied; otherwise resolve the current
   entity with `hq ctx --plain`. For a registered repo, read
   `hq entity show <company/product/repo> --json` and use its canonical `path`.
   Verify any task cwd matches that card on the selected machine.
2. A worktree resolves to its owning canonical repo for shared documentation.
   `.git` may be a directory or file. A manifest alone is not HQ registration.
   Do not silently analyze one branch and write shared conclusions from another.
3. If the user explicitly selected a standalone repo outside HQ, analyze that
   exact checkout and keep output there. No HQ card/state is implied; direct
   AGENTS.md updates are outside this skill for an unregistered repo.
4. Record the inspected path, branch/HEAD when available, and relevant dirty
   state. Do not switch branches, reset files or require a first commit. If
   there is no source yet, explain that limit and avoid a fictional architecture.
5. A product request belongs to `analyze-product`; company context belongs to
   `onboard-agents-md`. Report an unavailable target rather than substituting
   another scope or provisioning a repository.

All paths below are relative to the resolved checkout, not an incidental cwd.

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

9. **Draft `docs/design.md`** in the resolved repo. Read and preserve relevant
   existing documentation before updating it. Structure:
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
    - List candidates; do not create ADR files unless that work is authorized

11. **Optional AGENTS.md update:** propose the most useful non-obvious patterns.
    When the existing request authorizes this bounded context edit, use the HQ
    revision protocol in [onboard-agents-md](../onboard-agents-md/SKILL.md).
    Preserve other text; do not directly overwrite AGENTS.md or confirm inferred
    facts as human decisions. Otherwise include the proposal in the report.

12. **Deliver within the requested write scope.** If saving `docs/design.md`
    was explicitly authorized, save it and show the resulting changes without
    asking for the same authorization again. For read-only analysis, present
    the concrete draft and ask only if the user wants it saved. ADRs are a
    separate optional output; inspect existing numbering and use Draft status
    for proposed decisions. Never infer permission to create ADRs from a basic
    onboarding or design-doc request.

## Rules

- READ-ONLY by default. No edits to source code.
- Write only authorized documentation in the resolved repo; AGENTS.md changes
  use HQ revision checks. Before saving a document, re-read it if another writer
  may have changed it; preserve unrelated edits.
- Never `git commit` without explicit user ask.
- Be honest about uncertainty: mark inferences as "looks like X" or
  "appears to follow Y", suggest user confirms.
- For pattern detection — read enough code to be confident, not 1 example.
- Distinguish observed behavior from inferred intent. Ask about intent only
  when it materially affects the result; do not ask the user to re-confirm
  every source fact or an already authorized documentation write.

## Output structure

```
<resolved-repo-path>/
├── AGENTS.md            ← optionally updated (Non-obvious patterns section)
├── docs/                ← existing or created within authorized scope
│   ├── design.md        ← main output
│   └── adr/
│       ├── <next-number>-<topic>.md  ← only if separately authorized
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
