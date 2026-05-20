# Repo-level questions

Walkthrough для repo AGENTS.md. **Главная разница с company/product**: большая
часть наполнения должна делаться **автоматически из кода**, не вопросами user'у.

Стратегия: **сначала auto-fill** из манифеста и кода, **потом** только спросить
у пользователя то, что код не покажет (non-obvious patterns, history-specific
quirks).

---

## Step 1. Auto-fill из кода

Read следующие файлы (если есть):

- `README.md` — описание репо
- `pyproject.toml` / `package.json` / `go.mod` / `Cargo.toml` — manifest
- `Makefile` / scripts in `package.json` / `pyproject.toml` `[project.scripts]`
- `Dockerfile` / `docker-compose.yml`
- `.env.example` / `.env.template`
- `.gitlab-ci.yml` / `.github/workflows/*.yml`
- Entry points: `src/main.py`, `cmd/server/main.go`, `src/index.ts`, etc.
- `tests/` или `test/` структура

Из них заполнить sections:

### Stack
- Language version (из manifest)
- Framework (из dependencies)
- Top 3-5 key deps (только важные, не все)
- Database (из docker-compose / env vars / config)
- Runtime (uvicorn / gunicorn / node / go / etc.)

### Commands
Реальные команды (не угадывая):
- Treat `Makefile` as the canonical executable interface when present.
- Install — из manifest: `uv sync` / `npm install` / `go mod download`
- Dev (hot reload) — из package.json scripts / Makefile / pyproject scripts
- Build, Test, Lint, Format, Typecheck — то же
- Full verify — собрать lint + test + typecheck в одну команду
- If `Makefile` is absent but the repo has runnable code, propose/add a minimal one with stable targets:
  `install`, `dev`, `test`, `test-one`, `lint`, `format`, `typecheck`, `verify`.
- `verify` must be local and non-destructive: no migrations, deploys, broad cleanup, or external state mutation.
- If no automated tests exist yet, make `test` explicit instead of ambiguous (docs/smoke check or clear "no automated tests configured" message).

### Key files / entry points
- Entry point: actual path найденный в коде
- API routes: scan `**/*router*.py`, `**/*routes*.ts`, `**/*handler*.go`
- Config: scan `config.py`, `settings.py`, `.env*`, `<repo>/config/`
- Models / schemas: scan `models/`, `schemas/`, `types/`
- Tests: scan `tests/`, `test/`, `__tests__/`, `*_test.go`

### Local development
- Required env vars: из `.env.example` + grep `os.environ`/`process.env`/`os.Getenv`
- dev-stack DB: если в коде есть `DATABASE_URL` → подсказать `dev-stack db-create <name>`
- Redis: если в коде `redis://localhost`
- Port: из uvicorn / express / Gin config

### Conventions
- Один представительный snippet 3-10 строк — пройтись по `git log --oneline -20`,
  найти recent commit "fix" → посмотреть pattern. Или взять представительный
  endpoint / handler.

Apply: заменить ALL `<TODO: ...>` маркеры в этих секциях актуальными данными.

**Не выдумывать.** Если данные не нашлись в коде — оставить `not found in repo`.

---

## Step 2. Ask user only what code doesn't show

После auto-fill — show diff пользователю:
> "Auto-filled sections: Stack / Commands / Key files / Local dev / Conventions snippet.  
> Осталось 3-4 вопроса на то что код не покажет:"

### Q1. Non-obvious patterns

**Section:** `## Conventions (only non-obvious)` → bullets ниже snippet'а

Ask:
> "Есть ли counterintuitive patterns в этом репо? Примеры:  
> - `All API methods return Result<T>, NEVER throw`  
> - `Migrations are handwritten, NEVER alembic --autogenerate`  
> - `Use lib/http apiClient, never raw fetch/requests`  
> Расскажи знаешь ли что-то такое для этого репо."

Если "не знаю" — оставить TODO, потом сам допишешь когда узнаешь.

### Q2. Repo-specific dangerous areas

**Section:** `## Boundaries` → `⚠️ Ask first: Touch <TODO: ...>`

Ask:
> "Есть ли особо опасные места в этом репо помимо migrations / .env*? Что нужно ask first?  
> Примеры: `kafka topic schemas`, `payment_processing/*`, `feature_flags/*`  
> Или 'none' если ничего особого."

---

## Step 3. Final cleanup

1. Remove bottom HTML comment block with codex bootstrap prompt:
   ```bash
   sed -i '' '/^<!--$/,/^-->$/d' AGENTS.md
   ```
   (но только если этот блок уже не нужен — после первого fill его удаляют)

2. `grep -n TODO AGENTS.md` — final count
3. Если осталось > 0 → list, ask "оставить как есть или продолжить?"

---

## Alternative: full codex bootstrap (one-shot)

Вместо пошагового — можно делегировать codex'у полный auto-fill через bootstrap
prompt который лежит в конце сгенерированного repo AGENTS.md:

> "Read AGENTS.md and the rest of this repo: README.md, the package manifest
> (pyproject.toml / package.json / go.mod / Cargo.toml), Dockerfile if any,
> Makefile or scripts, entry points (main.py / cmd/main.go / src/index.ts),
> tests/ structure, .env.example, .gitlab-ci.yml or .github/workflows/.
>
> Fill out AGENTS.md sections marked <TODO>:
> - Stack: real versions from manifest
> - Commands: exact commands; prefer Makefile targets as canonical, then pyproject/package scripts
> - Testing contract: keep Makefile + AGENTS in sync; update Makefile when tests/runners/gates change
> - Key files: real paths
> - Local development: env vars from .env.example / .envrc / config, ports
> - Conventions: ONE representative 3-10 line snippet from THIS repo
> - Non-obvious patterns: only counterintuitive things (look at recent git log for "fix" commits)
> - Boundaries: any repo-specific danger zones
>
> Rules:
> - Real values only. NO "TBD" if data is in repo.
> - If not found, write 'not found in repo'.
> - Keep it concise: context for sessions, not docs for humans.
> - Telegraph style. Commands and tables.
> - Don't modify: top comment block, Definition of Done structure, Lessons section.
>
> Show me the diff before saving."

После этого — Q1 и Q2 выше для human-knowledge только.

---

## Final step

1. `git diff AGENTS.md` summary
2. `grep -c TODO AGENTS.md` — should be 0
3. Suggested commit (в repo's git):
   > `docs(agents): заполнить repo AGENTS.md (auto + manual)`
4. NEVER auto-commit.
