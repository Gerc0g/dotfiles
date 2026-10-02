---
name: fill-agents-md
description: "Condense existing design/architecture docs into repo or product AGENTS.md through HQ. Use after analyze-repo/analyze-product when docs exist, or for populate AGENTS.md from design and заполнить AGENTS по design.md. General entity filling uses onboard-agents-md. Generated facts never imply human-confirmed onboarding."
---

# Fill AGENTS.md from existing docs

Condense verified facts into the existing context document. This is a
non-interactive content update, not a human onboarding confirmation.

This skill is a shortcut when design docs already exist, not the universal
entry point for entity cards. Use `onboard-agents-md` for guided company,
product or repo filling, including empty repositories and missing docs.

## Resolve scope and evidence

1. Use the explicit HQ task scope when supplied; otherwise resolve it with
   `hq ctx --plain`. Read `hq entity show <scope> --json` and parse output as
   data. Use the returned `kind`, `document.path`, `document.content` and
   `document.revision`; state the canonical target before editing. Check a
   supplied cwd against the card. Preserve any requested item focus; do not
   fill unrelated sections merely because more evidence is available.
2. Markers are `.company-config`, `.product-config`, or `.git` (directory **or
   file**). Do not classify a directory as a product just from its path shape.
   Worktrees resolve to the owning repo card: inspect its canonical checkout
   for shared context, rather than importing unmerged branch-specific facts.
3. At repo level, read `docs/design.md`; at product level, read
   `docs/ARCHITECTURE.md` from the canonical entity path. If the required
   document is missing, this shortcut is unavailable. Continue a user-authorized
   guided filling request with `onboard-agents-md`; suggest deep analysis only
   when architectural documentation is actually wanted. Do not create missing
   design docs or guess their contents just to satisfy this prerequisite.
4. Company-level human decisions belong to `onboard-agents-md`; do not auto-fill
   them. If the HQ entity command/scope is unavailable, report the exact gap
   instead of bypassing its revision/state checks.
5. Compare design claims with current manifests, Makefile, entry points and
   tracked env examples. Source code is read-only; do not inspect secret values.

## Repo fields

Fill only existing placeholders in these exact template sections:

| Heading | Content |
|---|---|
| `## What this repo does` | One sentence supported by design and current entry points |
| `## Stack` | Actual language/runtime, framework, key deps and DB |
| `## Commands` | Existing Make targets first, then declared scripts; no invented verify chain |
| `## Key files` | Real, useful entry/config/handler/test paths |
| `## Local dev` | Required variable names, ports and applicable shared infrastructure |
| `## Env contract` | Existing explicit rules; never values of secrets |
| `## Non-obvious patterns` | Source-backed conventions, not generic advice |
| `## Boundaries` | Already documented restrictions, not inferred human policy |

Preserve Testing contract, WikiPedik memory, hierarchy, references, lessons and
custom prose. Missing Make targets can be proposed separately; this skill does
not authorize creating a Makefile. Recommend `dev-stack connect` only when
applicable; do not execute it or any other setup/infra command during filling.

## Product fields

Read the card's registered repo children and relevant existing repo design docs.
Fill existing placeholders under:

- `## What this is`: factual purpose. Status needs explicit reliable evidence;
  uncertain status stays TODO. Commit recency never implies production.
- `## Repos and roles`: preserve `Repo | Role | Stack hint`, using real repos.
- `## Data flow (only if non-obvious)`: describe supported dependencies. Absence
  of documentation is not evidence that the repos are independent.
- `## Cross-repo conventions`: actual shared schema locations and consumers.
- `## Boundaries (product-specific)`: documented restrictions only.

Do not create sections absent from the current document or replace human text
with a generated summary. Older custom headings are preserved; do not rename
them merely to match the current template. If required sections/document are
missing, use the scoped repair workflow in `onboard-agents-md` under the existing
fill authorization instead of silently doing nothing or replacing the file.

## Save through HQ

Apply exact changes to the current document string and preserve unrelated
content. Unknown facts remain `TODO: <specific missing evidence>`; retain
angle-bracket placeholders until resolved. An evidence-backed “not applicable”
is acceptable; replacing all unknowns with comments to eliminate TODOs is not.

Save the authorized factual edit with:

```text
hq entity update <scope> --json-base64 <base64-of-UTF8-JSON>
```

If no factual edits are needed, skip the write and report the current card.

The JSON contains the latest revision and **one** edit:

```json
{"revision":"<document.revision>","content":"<complete updated document>"}
```

Use a serializer and base64 encoder, not shell interpolation of document text.
On `HQ_ENTITY_REVISION_CONFLICT`, reload the card and reapply the intended
changes while preserving the other writer's edits. Use each returned revision
for subsequent requests. Do not directly edit `.hq/entity.json`.

Do **not** send `confirm` requests from this auto-fill workflow. Human review
belongs to `onboard-agents-md` or the user's explicit review of exact items.
Do not claim `complete` from saved text, TODO counts, or successful generation.

## Report

Report the scoped content changes, source evidence, remaining unknowns and the
reloaded card's actual onboarding status/items. Generated facts may now be in
`review`; that is expected. Keep context concise without deleting useful
human-authored instructions to meet an arbitrary line count. No auto-commit,
source edits, setup commands, or other artifacts are part of this skill.
