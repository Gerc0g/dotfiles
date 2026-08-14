# PLATFORM — Egor's local developer infrastructure


Universal capabilities + rules. Applies to all repos, all companies.
Company- or repo-level files may add more or override.

## Local infrastructure

dev-stack (Docker Compose), running locally on this machine. Remote targets come
back with the server layer (`hq server`), which is not implemented yet.
Canonical list: `dev-stack urls`. Source of truth: `services/dev-stack/`.
Connect a repo (self-onboard): `cd <repo> && dev-stack connect` writes the `.envrc`
endpoints + creates the DB `<product>__<repo>`; `dev-stack doctor` checks reachability
and wiring. Telemetry is PUSHED via OTLP — Prometheus does not scrape your app.
`dev-stack up/down/...`).

- Postgres `localhost:5432` (user `dev`, pass `dev`, default db `postgres`)
- Redis `localhost:6379` (no auth)
- MinIO S3 API `http://localhost:9000` (key `dev`, secret `devsecret123`), console `:9001`
- Qdrant `http://localhost:6333` (REST), gRPC `:6334`
- ClickHouse `http://localhost:8123` (user `dev`, pass `dev`)
- Prometheus `http://localhost:9090`
- Grafana `http://localhost:3030` (`dev`/`dev`) — Prometheus/Loki/Tempo pre-wired
- Loki `http://localhost:3100` (logs), Tempo `http://localhost:3200` (traces)
- OTel Collector ingest `localhost:4317` (gRPC) / `:4318` (http) → fans out to Loki/Tempo/Prometheus
- Langfuse `http://localhost:3001` (LLM observability)
- Ollama `http://localhost:11434` (local models)
- Metabase `http://localhost:3002` (BI), CloudBeaver `http://localhost:8978` (DB viewer, pg+clickhouse)

Lifecycle (shell, not in repo):
- `dev-stack up [svc...]` / `dev-stack down` / `dev-stack status` / `dev-stack urls`
- `dev-stack psql [db]` / `dev-stack redis-cli` / `dev-stack clickhouse`
- `dev-stack db-create <name>` / `dev-stack db-drop <name>`

Rules — repo needs:
- Postgres → `localhost:5432`, not embedded. Vector store → Qdrant `:6333`.
- Redis → `localhost:6379`. OLAP/analytics → ClickHouse `:8123`.
- S3-compatible storage → MinIO `:9000` (`boto3` `endpoint_url=http://localhost:9000`, key=`dev`, secret=`devsecret123`)
- LLM tracing → Langfuse `:3001`. Telemetry (metrics/logs/traces) → OTLP to OTel Collector `:4317`.
- Local LLM → Ollama `:11434`.

Don't:
- Add Postgres/Redis/MinIO/Qdrant/ClickHouse/Langfuse to a repo's docker-compose.yml
- Suggest `brew install postgresql`/`redis` etc. — use the shared stack
- Suggest cloud AWS S3 for local dev (use MinIO)

Stack down → suggest `dev-stack up`.

## Secrets

The secrets layer is being redesigned; `hq secret` is a stub for now. The
invariants stay binding: NO plaintext secrets in `.env`, NO secrets in git.
1Password vaults remain company-scoped (`Work-<company-slug>`) and the future
layer keeps scoped item names (`_company__<VAR>`, `<product>__<VAR>`,
`<product>__<repo>__<VAR>`) — the contract lives in the `core/secret`
package doc.

Commands:
- `secret signin` — 1Password login (works as before)
- everything else answers "not implemented" until the new layer lands

Rule: repo needs new env var → instruct user to run
`secret add --repo VAR value`.
Don't write secret values to `.env`/`.envrc` manually.
Do assume env available at runtime: `os.environ['VAR']` / `process.env.VAR`.

## Environment loading

direnv auto-loads `.envrc` on `cd`. Chain: company → product → repo via
`source_up`.

After edit: `direnv allow`. Don't use `.env`. Project-shipped `.env.example` is
documentation, not config.

## Git protocol

Universal rules. Adapted from steipete/agent-scripts + Anthropic best practices.

Safe by default:
- `git status` / `git diff` / `git log`
- Read-only operations always OK

### Commit frequency: small + often

**Principle: many small bombs, not one Fat Man.**

- Commit after EVERY logical change, not at end of session
- "Logical change" = one rename / one bug fix / one feature addition / one refactor step
- Anthropic official: "Commit early, commit often. Treat git as your safety net."
- Steinberger: in parallel-agent sessions, commit every ~30 seconds of small atomic
  changes → collision rate near zero. Commit once per hour with big blocks → conflicts guaranteed
- Big task → multiple small commits in sequence, NOT one mega-commit at the end

Examples (✅ atomic, ❌ bundled):

✅ One commit per logical change:
  - `feat(auth): add refresh token rotation` (auth.py, models.py — same feature)
  - `test(auth): cover refresh token edge cases` (test_auth.py — separate commit)
  - `docs(auth): update README for refresh flow` (README.md — separate commit)

❌ Bundled mega-commit:
  - `feat: misc auth improvements + readme + cleanup` (5 files, 3 logical changes)

### Commit mechanics

- Atomic commits with **explicit paths**: `git commit -m "..." -- path/to/f1 path/to/f2`
- NEVER `git add .` or `git add -A` — collides with parallel agents
- New files: `git restore --staged :/ && git add "...specific files..." && git commit -- ...`
  (the `git restore --staged :/` resets stage from other agents before adding yours)
- Conventional Commits format: `feat|fix|refactor|build|ci|chore|docs|style|perf|test(scope): description`
- No repo-wide search/replace scripts; keep edits small and reviewable


### Managed agent workspaces and commit mode

When `AGENT_GIT_MODE=commit-local`, the user has delegated local atomic
commits for this agent session. In that mode:

- After every completed logical change, run:
  `hq commit "type(scope): русское описание" -- <explicit paths>`
- Finish only when the logical task/branch is complete, using:
  `hq finish`
  This verifies/tests, pushes the branch, opens a draft PR/MR into the integration branch, and records review metadata.
- No force-push, no rebase, no merge, no `git add .`, no `git add -A`.
- Do not start the next feature/epic while the completed change is dirty/uncommitted.
- Shared checkout branch switching is forbidden. Do not `git checkout` /
  `git switch` in a shared repo if another agent may be using it.
- Product-level or multi-repo writing work must use a managed worktree:
  `agent-workspace launch <company> <product> <repo> <task-slug>`.
- Task/epic state (plan, decomposition, "where I stopped") lives in
  `docs/epics/<task-slug>.md` and is committed with the branch like any other
  change. Uncommitted epic docs and `.agents/oracle/` answers are rescued into
  WikiPedik `_salvage/` when the worktree is removed, but a commit is the
  reliable path — salvage is the safety net, not the workflow.
- Default integration branch is `dev` when present, otherwise `main`; agent
  branches are named `agent/<task-slug>` and merge/PR back into the integration
  branch.

### Language: commit messages in Russian

Default convention: Conventional Commits prefix in English (tooling-friendly),
description and body in Russian.

Rules:
- Prefix (`feat|fix|refactor|chore|docs|test|perf|build|ci|style`): always English
- Scope in parens: English snake_case (`auth`, `api`, `db`, `barrier`) — matches code dirs/modules
- Description after colon: Russian, императив, lowercase, без точки в конце
- Body (if any, through blank line): Russian, multi-line ok
- Code identifiers (function names, file paths, env vars, error names): English как в коде
- Issue refs always when known: `(closes #123)` or `(NRDSK-456)` or `(refs #234)`

Examples (✅):

```
feat(auth): добавить ротацию refresh-токенов

При логине теперь выдаём пару access+refresh, refresh живёт 30 дней.
Каждый обмен ротирует токен — старый инвалидируется через jti-blacklist
в Redis с TTL = remaining lifetime.

Closes #234
```

```
fix(api): убрать N+1 запрос в /users endpoint
refactor(db): вынести connection pool в отдельный модуль
docs(readme): обновить инструкцию по запуску dev-stack
test(integration): покрыть error-flow для barrier→cortex
chore(deps): обновить ruff до 0.6.5
```

Anti-patterns (❌):
- `функция(аутентификация): добавление ротации` — ломает tooling (changelog generators, semantic-release)
- `feat: stuff` — нет scope, нет смысла
- `fix: исправил баги.` — нет scope, точка в конце, plural без специфики
- `WIP` — не использовать на final commit (только локально, squash перед merge)

Company-level may override (e.g. fully English commits for international team).
Default for both neurodesk + justchimera: this Russian convention.

### Branch policy

Integration branches — NEVER commit directly:
- `main` — production. Auto-deploys to PROD. Receives only `dev → main` release merges.
- `dev` — staging. Auto-deploys to STAGE. **All feature branches merge HERE first.**
- Feature branches — ephemeral, work happens here

Lifecycle:
- One branch = one task / one feature / one bug. NOT "misc fixes" / "monday-work".
- Branched from latest `dev`:
  `git checkout dev && git pull --ff-only && git checkout -b feat/...`
- Ephemeral only. No long-running `release/*` / `hotfix/*` unless repo CI requires.
- Don't reuse merged branches. Delete after merge.
- If repo has only `main` (no `dev` set up yet): use `main` as integration branch.
  Rare in our setup — most repos have both. ASK user if uncertain.

Naming (kebab-case, English, what the change does):
- `feat/add-rate-limit`
- `fix/n-plus-1-users-api`
- `refactor/extract-db-pool`
- `chore/bump-ruff-0.6`
- With ticket ID: `feat/NRDSK-123-add-webhook-validation`

Branch type ↔ commit type: matches. Branch `feat/xxx` → commits start with `feat(...)`.

Working with existing branch:
- Active PR/MR branch → continue on it, DON'T rebase against main mid-review
- Stale branch (>2 weeks no commits) → ASK user: resume? close? rebase?
- Branch with conflicts → STOP, show conflict files, ask before resolving

Who creates branches:
- Agent CAN create feature branches for new tasks (when scope is clear)
- Agent CANNOT delete branches without explicit ask (even merged)
- Agent CANNOT switch to other branches without explicit task need
- Branch change in active session → tell user before doing it

### Merge policy: feature → dev

Default strategy: **squash-and-merge** for feature → `dev`.
- One PR/MR = one squashed commit in `dev` → clean history
- Feature branch's atomic commits preserved on the feature branch (audit trail)
- Exception: truly logical multi-step PRs may use regular merge — ASK user first

Ready to merge feature → dev (Definition of Done for MR/PR):
1. All CI checks green
2. Self-review passed (`git diff dev..HEAD` looks intentional)
3. Manual QA done (for backend/frontend behavior changes)
4. At least one human review (mandatory for neurodesk; optional for justchimera)
5. No unresolved review comments
6. Branch up-to-date with `dev` (rebase or merge-in if behind):
   `git fetch origin && git rebase origin/dev` (only if branch not yet pushed for review)
   OR `git merge origin/dev` (if PR already in review — don't rebase pushed)

Who merges:
- Agent CANNOT merge without explicit user ask
- Use platform helpers when asked: `gh pr merge --squash --delete-branch` /
  `glab mr merge --squash --remove-source-branch`
- Set MR/PR target branch = `dev`, never `main`
- Auto-merge ok ONLY if user explicitly enabled + CI required checks configured

After merge — cleanup ritual:
1. `git checkout dev`
2. `git pull --ff-only` (pull squashed commit)
3. `git branch -d feat/xxx` (delete local; `-D` if force needed — ASK)
4. Remote branch usually auto-deleted by GitHub/GitLab after merge
5. `git status` → must show clean tree on `dev`
6. Report to user: what merged, что задеплоится на stage, what's next

Conflicts during merge:
- STOP, don't auto-resolve
- Show user: which files, which lines, ours-vs-theirs context
- Ask whether to take ours/theirs/manual edit
- After resolution: `git add <resolved-files>` (explicit paths), continue merge

Rebase vs merge:
- Local branch not yet pushed → rebase OK (cleaner history)
- Pushed/shared branch → NEVER rebase (rewrites history other agents/people see)
- Force-push only if user explicitly says "force" or "force-push"

### Release: dev → main (production deploy)

Separate flow from feature merge. Triggered when stage is verified and ready for prod.

Who triggers:
- USER initiates (or release manager)
- Agent does NOT release on its own — even if `dev` looks green
- Agent CAN prepare release (changelog, version bump) when asked

Pre-release checklist (ALL must be true):
1. `dev` deployed to stage and **verified by user** (smoke tested, manual QA)
2. CHANGELOG.md updated with version + changes since last release
3. Version bumped in manifest (`pyproject.toml` / `package.json`)
4. No `WIP` / `hotfix-tmp` commits in `dev` history since last release
5. No open critical bugs against `dev`

Release options (ASK user which):

**A. Fast-forward merge** (linear history, default if dev is straight ahead):
```
git checkout main
git pull --ff-only
git merge --ff-only dev
```

**B. Merge commit** (preserves "release marker" in history):
```
git checkout main
git pull --ff-only
git merge --no-ff dev -m "release: vX.Y.Z"
```

Tag the release:
```
git tag -a vX.Y.Z -m "vX.Y.Z release notes summary"
```

Push (ONLY with explicit user ask):
```
git push origin main
git push origin vX.Y.Z
```

After release:
- Verify prod deploy succeeded (CI/CD pipeline → check status)
- Smoke-check prod URLs (curl health endpoints) — ASK user before running
- Don't merge new features to `dev` for 10-15 min if release is in flight
- Update release notes in GitHub/GitLab UI (or `gh release create` / `glab release create`)

Hotfix to prod (skipping dev):
- Branch from `main` not `dev`: `git checkout main && git checkout -b hotfix/...`
- After fix verified: merge to `main` AND **back-port to `dev`** (else next release loses fix)
- Back-port: `git checkout dev && git merge main` OR cherry-pick the hotfix commit

If session was multi-step: end in branch/checkout user expects (not detached HEAD).

### Before EVERY commit (ritual)

1. `git status` — verify intent, see what's staged
2. `git diff --staged` — self-review the actual change
3. `git commit -m "<conv-commit>" -- <explicit paths>` — never `add -A`
4. (Optional) `git log --oneline -5` — confirm history looks right

### Push & remote

- NEVER run raw `git push` without explicit user ask. In agent worktrees, finish through `hq finish`; use `hq finish --no-review` only as a low-level fallback when PR/MR creation is unavailable or explicitly requested.
- NEVER `--force-push` unless user explicitly says "force"
- NEVER amend pushed commits unless user explicitly asks
- `git pull` ok when user asks ("pull and push" = consent for both)

### Destructive ops — FORBIDDEN unless explicit user approval

- `git reset --hard`
- `git restore` (state-changing form, not `git restore --staged`)
- `git clean -fd`
- `rm` on tracked files
- `git checkout` of old commit (detached HEAD)
- `git checkout -- <file>` (overwrites uncommitted changes)
- `git rebase` of pushed commits

If user issues a destructive command verbatim ("reset hard to HEAD~3") — that's consent.
Otherwise ASK.

### Multi-agent coordination

- Unrecognized changes in file you didn't touch → assume parallel agent, keep going
- If git operation leaves you unsure about in-flight work → STOP, ask user
- Never silently "fix" other agent's state by deleting or reverting
- Two agents staged same file → STOP, ask user which version to keep

### Sacred files — read-only always

- `.env*` (any env file)
- `.envrc` (don't write secrets directly; use `secret add`)
- `migrations/` after release shipped (handwritten, never re-generate or modify)
- `vendor/` (third-party code; patches go in `lib/vendor-overrides/`)
- `.gitignore` if pattern would expose tracked secrets

### `git checkout` of other branches

- Only for PR/MR review with explicit user instruction
- After review, return to user's expected branch
- Never leave the user in detached HEAD or wrong branch

## Git host conventions

SSH aliases per company: `<vcs>-<company-slug>` (e.g. `gitlab-neurodesk`,
`github-justchimera`). Each has own key `~/.ssh/<slug>_id_ed25519`.

Don't suggest raw `git@github.com:...` / `git@gitlab.com:...` URLs.
Do suggest alias form: `git@gitlab-neurodesk:dev/.../repo.git`.

## Project layout

```
~/Desktop/Prokectfiles/<company>/<product>/<repo>/   ← work
~/Desktop/WikiPedik/{dev,research}/                  ← Karpathy LLM Wiki
```

Each layer has AGENTS.md. Codex auto-discovers from root → cwd.

Config files:
- `<company>/.company-config` — slug/vcs/host/ns/ssh_host
- `<product>/.product-config` — slug/namespace
- `<company>/.envrc`, `<product>/.envrc`, `<repo>/.envrc` — direnv chain

## Tooling preferences

Universal defaults. Company-level may override.

- Python: 3.11+, use `uv` (NOT pip/poetry). `uv add`, `uv run`, `uv sync`.
- Node: 20 LTS, use `pnpm` (NOT npm/yarn unless `packageManager` field requires).
- Container: OrbStack (transparent docker / docker compose).
- DB migrations: Alembic for Python; idiomatic tool of stack for others.
- Lint/format (Python): `ruff check . --fix` + `black .` (or `ruff format .`).
- Lint/format (JS/TS): `eslint --fix` + `prettier --write` unless biome.
- Editor: terminal-first (codex/claude in tmux/Ghostty).

Don't suggest replacing existing tooling unless explicitly asked.

## Long-term knowledge: WikiPedik

When a learning is worth keeping across projects/sessions → `~/Desktop/WikiPedik/dev/`
(coding patterns, runbooks, lessons) or `~/Desktop/WikiPedik/research/`
(papers, articles). The Git repository is the full vault root:
`~/Desktop/WikiPedik`.

Open via `wikipedik` command (tmux with two codex profiles). Karpathy LLM Wiki pattern.

Don't dump everything in repo's AGENTS.md. Cross-cutting knowledge → wiki.

### Worker memory capture

For normal project agents in repos under `~/Desktop/Prokectfiles/<company>/<product>/<repo>`:

- Read path is automatic: `SessionStart` injects `docs/knowledge/hot.md` when the repo has been bootstrapped with WikiPedik symlinks.
- Write path is selective: when you identify a durable root cause, production gotcha, failed approach, reusable rule, informal decision, or cross-repo invariant, use the `lesson-append` skill to append one structured entry to `docs/knowledge/_inbox.md`.
- Before the final answer, run a memory checkpoint: if this task produced a durable root cause, production gotcha, failed approach, reusable rule, informal decision, or cross-repo invariant, use `lesson-append`; if not, write nothing.
- Do not write curated pages directly (`lessons.md`, `gotchas.md`, `debugging-stories.md`, product/shared pages). Curator handles that via `wiki sync <scope>`.
- If `docs/knowledge/_inbox.md` is missing, do not create ad hoc wiki paths. Tell the user to run `wiki bootstrap <company> <product>` or `wiki-bootstrap-product <company> <product>`.
- Skip low-signal notes. Captures should answer: what happened, why it happened, and what reusable rule follows.

Curator commands:
- `wiki sync <company[/product[/repo]]>` — drain `_inbox.md` into curated pages with confirmation.
- `wiki sync --commit <scope>` — drain and commit WikiPedik changes; pre-existing dirty files are allowed only inside the requested scope.
- `wiki sync --push <scope>` — drain, commit, and push WikiPedik changes. Use only when the user explicitly asked to sync with git.
- `wiki status <company[/product[/repo]]>` — report pending memory work.
- `wiki synthesize <company/product>` — promote repeated repo lessons into product shared patterns.

Wiki git policy:
- Wiki commits are atomic by scope: commit only the requested company/product/repo memory path, its product log, and the root project index when bootstrap changed it.
- `wiki-commit` / `wiki sync --commit` uses the company `git_email` from `~/Desktop/Prokectfiles/<company>/.company-config` when present.
- Daily agents do not auto-commit or auto-push memory. They append candidates only; curator commands handle git sync.
- Never push wiki changes unless the user explicitly asked for `--push` or a manual `git push`.
- WikiPedik commits go directly to `main`; do not use product delivery flow (`dev` as stage, `main` as prod, feature branches) for wiki curation. Push still happens only at explicit sync/task completion.
- WikiPedik commit messages follow the same language convention: English Conventional Commit type/scope, Russian description.

## Definition of Done (universal)

Task is complete when ALL apply:
1. Code changes self-reviewed (`git diff` of staged)
2. Tests pass (repo's command from its AGENTS.md)
3. Lint passes (repo's command)
4. Files committed atomically with explicit paths
5. Commit message Conventional Commits
6. Status reported to user with: what changed, what tests ran, any open questions

NEVER report "done" before running the actual verification commands.

Repo testing contract:
- `Makefile` is the preferred executable interface for local agents and test panes.
- Keep repo `AGENTS.md` command docs synced with `Makefile`.
- When changing test infrastructure, add or update stable targets first: `test`, `test-one`, `lint`, `format`, `typecheck`, `verify`.
- `verify` must stay local and non-destructive: no migrations, deploys, force cleanup, or external state mutation.
- If no automated tests exist yet, make `test` explicit instead of leaving the repo ambiguous.

## When Blocked

| Situation | Action |
|---|---|
| Tests fail 3× same way | STOP. Report failing test name + full output to user. |
| Missing dependency | Check `pyproject.toml` / `package.json`. Then ask. |
| Merge conflict | STOP. Show conflicting files. Don't auto-resolve. |
| Unfamiliar error in 3rd-party lib | Read lib docs (Read tool). Don't guess fix. |
| User instruction ambiguous | Ask ONE targeted clarifying question. |
| Disk full / network down | Report to user; don't retry blindly. |

NEVER:
- Delete files to resolve errors
- Bypass tests with `--no-verify` or skips
- Force-push to "fix" history
- Run destructive ops to "clean state"

## Discovery: `?` command

User has shell builtin `?` listing platform capabilities by category.
`? <cmd>` → details.

Canonical: `~/dotfiles/docs/COMMANDS.md`.

User asks "how do I X" where X is platform op → suggest `? X` or read COMMANDS.md.
Don't guess workflow that platform already encodes.

## Agent behavior notes

- Shared local infrastructure between repos/companies. Postgres may have data
  from other projects. Don't assume isolation.
- Additive over disruptive. New linter/test framework — ASK before replacing.
- Two-school stance: default to **Willison-style** (TDD, manual QA, never
  unreviewed PR). Company-level files may opt into Steinberger-style minimalism
  for low-stakes pet projects.
