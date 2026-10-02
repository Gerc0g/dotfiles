# Repo-level questions

Derive verifiable facts first, then ask for human knowledge. Work from the
canonical repo card and its checkout; `.git` may be a file in a worktree.
Auto-filling prose does not confirm onboarding items.

Honor any item focus from the HQ task. A Git repo with no commits/source is a
valid onboarding target: document what is absent, ask about intended purpose,
and distinguish future choices from implemented facts. Missing design docs,
manifests, Makefile or tests do not require generating them to proceed.

## Read the sources

Read only relevant files that exist:

- README, package manifests and lockfiles;
- Makefile and declared package scripts;
- existing `docs/design.md`, entry points, representative modules and tests;
- Dockerfile and CI definitions for supporting evidence;
- tracked env examples and variable names in config code.

Do not read/print real secrets or use `.env*` values as documentation input.
Do not run install, bootstrap, infrastructure or migration commands merely to
fill the card. Current source outranks stale design docs; record discrepancies.

## Fill existing sections

| Exact template heading | Evidence to use |
|---|---|
| `## What this repo does` | README and entry points; one factual sentence |
| `## Stack` | Manifest/runtime versions, framework, important deps, DB |
| `## Commands` | Actual Make targets first, then existing package scripts |
| `## Key files` | Real entry/config/handler/test paths, about 5–10 files |
| `## Local dev` | Required variable names, actual ports, applicable dev-stack services |
| `## Env contract` | Existing rules and documented variable names, no values |
| `## Non-obvious patterns` | Source-backed conventions and the user's knowledge |
| `## Boundaries` | Existing explicit rules and human decisions |

- Replace only resolved placeholders. Leave `TODO: <specific missing fact>`
  where evidence is unavailable; do not substitute “not found” to hide a gap.
- Do not invent a combined verify command. Document the actual target/script
  and its scope. If it has not been run, do not claim validation passed.
- If Makefile or a useful target is absent, propose a separate change. Only
  implement it when the user has authorized that work; card filling alone is
  not authorization. Preserve `## Testing contract`.
- A `DATABASE_URL` reference alone does not prove which DB is used. If the
  shared infra is appropriate, suggest `dev-stack connect` and `dev-stack doctor`.
  Do not execute connect here: it writes `.envrc` and may create a database.
- Do not add a Conventions snippet section: the current template has
  `## Non-obvious patterns`. Preserve existing useful custom content.

Save factual edits using the SKILL.md protocol. For a full-document payload,
start with the current card content and apply exact scoped replacements; never
regenerate the file from a template.

## Ask only unresolved questions

1. **Purpose:** show the factual role sentence for review when not already
   supplied by the user.
2. **Commands / stack / key files:** show the evidence-backed draft. Ask for
   human review of the whole backend `commands` item, not only one command.
3. **Environment:** clarify missing requirements in `Local dev` and `Env
   contract`; obtain review of the complete `environment` item.
4. **Non-obvious patterns:** «Какие правила здесь неочевидны из обычного чтения
   кода? Есть подтверждённые исключения или исторические причины?»
5. **Boundaries:** «Какие особые области требуют согласования сверх уже
   записанных правил? Если таких нет, можно явно это зафиксировать».

Items 4–5 belong to the same backend `boundaries` item; do not confirm it while
part of that content remains unknown. A “не знаю” answer leaves a TODO. An
explicit “дополнительных ограничений нет” records an actual human decision.

## Finish

Re-read the HQ card and report saved changes, backend status, and pending
missing/review items. Do not delete HTML comments broadly, look for an embedded
Codex bootstrap prompt, or launch another agent: the template contains no such
prompt and this workflow does not require one. No automatic commit follows.
