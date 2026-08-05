---
name: fill-agents-md
description: "Fill AGENTS.md template TODOs at repo or product level by reading the already-generated docs/design.md and docs/ARCHITECTURE.md. Used after analyze-repo / analyze-product. Triggers: \"fill agents md\", \"заполни AGENTS\", \"populate AGENTS.md from design\", \"complete AGENTS.md template\"."
---

# Fill AGENTS.md from generated docs

Condenses existing `docs/design.md` / `docs/ARCHITECTURE.md` into the short
`AGENTS.md` "agent quick-reference" format.

## When to trigger

- After `analyze-repo` / `analyze-product` generated `docs/design.md` and
  `docs/ARCHITECTURE.md` — to convert the deep doc into a short AGENTS.md
- When AGENTS.md still has `TODO:` placeholders left from envsubst template
- When design.md was manually updated and AGENTS.md needs re-sync

## Prerequisites

- cwd must have `AGENTS.md` with TODOs (templated, not filled)
- cwd must have `docs/design.md` (repo level) OR `docs/ARCHITECTURE.md` (product level)
- If docs/ files are missing — STOP, suggest running analyze-repo / analyze-product first

## Detect level

- If cwd has `.git/` → **repo level** — read `docs/design.md`
- If cwd has `.product-config` (or matches `<co>/<prod>/`) → **product level** — read `docs/ARCHITECTURE.md`
- If cwd has `.company-config` → company level **— DO NOT touch.** Company AGENTS.md
  has human-only decisions (Stance, PII, Network, Docs URL). Tell user to use
  the `onboard-agents-md` skill instead.

## Workflow — REPO level

1. **Read AGENTS.md** in cwd. Identify sections to fill (anything starting with `TODO:`):
   - `## What this repo does`
   - `## Stack` (Language, Framework, Key deps, Database)
   - `## Commands` (Install, Dev, Test, Lint, Format, Typecheck, Full verify)
   - `## Key files` (Entry, Routes/handlers, Config, Tests)
   - `## Local dev` (env vars, dev-stack usage, port, run cmd, health check)
   - `## Non-obvious patterns`
   - `## Boundaries` 🚫 + ⚠️

2. **Read docs/design.md fully.** This has all the source info.

3. **Read manifest files for source-of-truth on stack/commands:**
   - `pyproject.toml` — Python version, framework, deps, scripts
   - `package.json` — Node version, framework, deps, scripts
   - `go.mod` — Go version, key deps
   - `.env.example` — required env vars
   - `Makefile` — additional commands
   - `Dockerfile` — port hints

4. **Fill each section from design.md + manifest:**
   - **What this repo does:** 1 sentence — extract from design.md "Purpose" paragraph
   - **Stack:** condense design.md "Tech Stack" / "Dependencies" to 4 lines
   - **Commands:** extract from package.json `scripts` / pyproject `[project.scripts]`
   - **Key files:** pick 5-10 most important files from design.md "Module structure"
   - **Local dev:** required env vars from `.env.example`, dev-stack mapping from design.md "External Dependencies"
   - **Non-obvious patterns:** top 3-5 from design.md "Non-obvious conventions"
   - **Boundaries:** extract from design.md "Boundaries" section

5. **AUTO-SAVE.** Write the filled AGENTS.md. Don't ask for confirmation.

6. **Preserve sections that are NOT TODO:**
   - `## Context hierarchy` table — keep as-is
   - `## Lessons (organic)` — keep as-is (already has comment placeholder)
   - Any existing custom content user added

## Workflow — PRODUCT level

1. **Read AGENTS.md.** Find TODO sections:
   - `## What this is` (+ Status line)
   - `## Repos and roles` (table)
   - `## Data flow` (only if non-obvious)
   - `## Cross-repo conventions` (Shared schemas/DTOs, API change protocol)
   - `## Boundaries` 🚫 + ⚠️

2. **Read docs/ARCHITECTURE.md fully.**

3. **For each subdirectory (repo) in product, read `<repo>/docs/design.md`** to
   get stack hint + role one-liner.

4. **Fill sections:**
   - **What this is:** 1 sentence from ARCHITECTURE.md Purpose
   - **Status:** infer from ARCHITECTURE.md or commit recency. If uncertain, mark `production` (default).
   - **Repos and roles table:** one row per repo, columns: `Repo | Role (1-line from its design.md) | Stack hint`
   - **Data flow:** only if ARCHITECTURE.md has clear inter-service flow; otherwise leave `(no non-obvious flow)`
   - **Cross-repo conventions:** shared schemas/types location if mentioned in ARCH.md; API protocol from ARCH.md "Boundaries"
   - **Boundaries:** transfer 🚫 / ⚠️ items from ARCH.md "Boundaries" section

5. **AUTO-SAVE.**

## Rules

- READ-ONLY for source code, manifests, design.md, ARCHITECTURE.md
- WRITE only to AGENTS.md in cwd
- AUTO-SAVE — don't ask. Skill is non-interactive.
- Don't add new sections — only fill existing TODOs
- Preserve markdown structure exactly (headings, table layouts, hierarchy table)
- If a section can't be filled from available data (e.g. no `Makefile`, no `pyproject.toml`)
  — leave a brief comment `<!-- not applicable / no data -->`, not the original TODO
- Be terse. AGENTS.md is meant to be ~80-120 lines max. Don't expand from design.md
  verbatim — condense.

## Quality bar

A good filled AGENTS.md answers, in <120 lines:
- "What does this repo/product do at a glance?" (1 sentence)
- "How do I run / test / verify it?" (commands)
- "Where do I look first?" (key files)
- "What env / ports / DBs?" (local dev)
- "What gotchas?" (non-obvious patterns)
- "What's off-limits?" (boundaries)

If filled AGENTS.md >150 lines — re-condense, you over-expanded.

## Output

Single file updated in-place: `./AGENTS.md`. No new files. No prompts.
