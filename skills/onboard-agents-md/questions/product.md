# Product-level questions

Use `hq entity show <company/product> --json` and the current AGENTS.md. The
headings below match `templates/AGENTS.md.product.tmpl`. First collect facts from
the card and available repo sources; architecture docs are optional evidence.
Then ask one unresolved question at a time within any requested item focus.
Save through HQ and retain unknowns as TODO.

## Purpose and status

**Section:** `## What this is`.

Ask separately when needed:

1. «Что делает продукт, для кого и какую задачу решает — одной строкой?»
2. «Какой подтверждённый статус: production, staging, experimental или legacy?»

Do not infer deployment status from recent commits, repository activity, or a
default. The backend `purpose` item covers both purpose and the Status line.

## Repositories and roles

**Section:** `## Repos and roles`.

Read the card's `children` and available `<repo>/docs/design.md` / manifests.
Only registered Git repositories belong in the table; arbitrary directories
are not repositories. Git worktrees may carry a `.git` file.

Draft `Repo | Role | Stack hint` rows from evidence. Ask about unclear roles:
«Правильно ли описана роль этого репозитория? Что нужно уточнить?»

Source-derived rows remain unconfirmed until the user reviews the whole
`repositories` item. A currently empty product may be documented as having no
registered repos after checking the card; this factual edit still awaits human
review. Do not invent future repositories or run provisioning to fill the table.

## Data flow and contracts

**Sections:** `## Data flow (only if non-obvious)`, `## Cross-repo conventions`.

Use existing `docs/ARCHITECTURE.md` as evidence, then ask unresolved questions:

1. «Как данные и запросы проходят между репозиториями? Есть нетривиальные связи?»
2. «Где общие схемы/DTO и какие потребители меняются вместе с API?»

Record a short flow and actual schema/consumer locations. If the user confirms
independent repos or no special flow, say so in the existing section; removal
of this optional section is also supported. Missing evidence is not proof that
there are no dependencies.

Keep the existing testing and env contract. For an applicable repo, the current
infra recommendation is `dev-stack connect`, followed by `dev-stack doctor`;
this walkthrough documents the recommendation and does not execute it.

The `contracts` item covers all relevant data-flow and conventions content.

## Boundaries

**Section:** `## Boundaries (product-specific)`.

Ask: «Какие области и операции требуют отдельного согласования: общие схемы,
миграции, конфиги, изменения сразу в нескольких репозиториях?»

Use the user's current authorization and concrete project rules. Do not create
new blanket approval requirements or erase existing ones. Unknown restrictions
remain TODO. Confirm the whole `boundaries` item only when reviewed.

## Finish

Report the reloaded card's status, unresolved/review items, and the scoped
document changes. The template has no Local development, Procedural workflows,
or References sections: do not invent those sections during placeholder fill.
Do not change the automatic `coordinates` item, force completion, or commit.
