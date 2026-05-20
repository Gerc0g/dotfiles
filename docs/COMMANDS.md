# Personal Platform Commands

Single source of truth для custom commands.

Format: `## <command>` + `**Что:**` `**Запуск:**` `**Файлы:**` etc.
Группируется по `### Category` (порядок секций определяет порядок в `?`).

---

### Discovery

## help
**Что:** Список команд по категориям. `help <cmd>` для деталей.
**Запуск:** `help` или `help <cmd>`

## ?
**Что:** Короткий alias для `help`.
**Запуск:** `?` или `? <cmd>`

---

### Main flow

## setup-context
**Что:** End-to-end pipeline: refresh-templates → batch-setup (codex анализ кода → docs/design.md + docs/ARCHITECTURE.md + ADRs) → batch-fill-agents (codex сжимает доки в AGENTS.md). ETA для компании из 5 продуктов: ~25-50 мин в параллели.
**Запуск:**
- `setup-context <co>` — вся компания, все продукты параллельно
- `setup-context <co> <prod>` — один продукт
- `setup-context <co> <prod> <repo>` — один репо
**Зависит от:** codex CLI, skills/{analyze-product, analyze-repo, fill-agents-md}

## complete-onboard
**Что:** Interactive Q&A wizard для TODO которые setup-context не может вывести из кода — Stance, PII, Network policy, Docs URL (company-level). Запускать ПОСЛЕ setup-context. ETA ~15-30 мин.
**Запуск:**
- `complete-onboard <co>` — все уровни (но фактически работы только на company после setup-context)
- `complete-onboard <co> <prod>` — продукт-уровень
- `complete-onboard <co> <prod> <repo>` — репо-уровень
**Зависит от:** codex CLI, skills/onboard-agents-md

## batch-fill-agents
**Что:** Перегенерить AGENTS.md из готовых `docs/design.md` / `docs/ARCHITECTURE.md` без повторного анализа кода. Параллельно для всех product/repo. ETA ~5-15 мин.
**Запуск:**
- `batch-fill-agents <co>` — вся компания
- `batch-fill-agents <co> <prod>` — один продукт
**Используй:** когда руками поправил design.md/ARCHITECTURE.md и хочешь обновить AGENTS.md, без перезапуска setup-context.
**Логи:** `~/dotfiles/logs/fill-agents-<co>-<ts>.log`
**См. также:** `fill-agents-md`, `setup-context`

---

### Workspaces

## launch
**Что:** Запустить tmux session для одного репо с 4-pane layout (Steinberger pattern). Атомарность работы = repo (по ресерчу).
**Запуск:**
- `launch` — interactive picker: company → product → repo
- `launch <co>` — указал компанию, пикер для product + repo
- `launch <co> <prod>` — пикер для repo
- `launch <co> <prod> <repo>` — direct
**Panes:**
- 🧠 **plan** — codex gpt-5.4 high reasoning (architect / planner)
- 💻 **code** — claude code (implementer)
- 🧪 **test** — `test-tui`: reads `Makefile`, `.agents/commands.toml`, and package manifests; saves run logs to `.agents/test-runs/`
- 🔮 **oracle** — `oracle-tui`: prompt + local answer history in `.agents/oracle/`, backed by `@steipete/oracle`
**Зависит от:** tmux, codex, claude, oracle (`npm i -g @steipete/oracle`)

## wikipedik
**Что:** tmux с двумя codex: dev + research wiki vaults параллельно. (Obsidian vaults, не код-репы.)
**Запуск:** `wikipedik`

## wiki
**Что:** WikiPedik project-memory commands. Без аргументов запускает wiki product repo, с подкомандами управляет curator flow.
**Запуск:**
- `wiki` — открыть `neurodesk/wiki/nrdsk_wiki` через стандартный launch layout.
- `wiki sync <company[/product[/repo]]>` — curator drain: `_inbox.md` → `lessons.md` / `gotchas.md` / etc. через `inbox-drain`.
- `wiki sync --commit <scope>` — drain + локальный commit WikiPedik changes. Заранее dirty файлы разрешены только внутри указанного scope.
- `wiki sync --push <scope>` — drain + commit + push.
- `wiki status <company[/product[/repo]]>` — отчёт по pending inbox, curated pages, synthesis candidates.
- `wiki synthesize <company/product>` — найти повторяющиеся lessons/gotchas и предложить product shared patterns.
- `wiki bootstrap <company> [product] [repo]` — создать WikiPedik skeleton + repo symlinks.
- `wiki-commit <scope>` — вручную закоммитить только changes внутри WikiPedik scope.
- `wiki-git <args...>` — выполнить `git` внутри `~/Desktop/WikiPedik/dev`.
**Примеры:**
- `wiki sync neurodesk/wiki/nrdsk_wiki`
- `wiki sync --commit neurodesk/wiki/nrdsk_wiki`
- `wiki-git status`
- `wiki status neurodesk`
- `wiki synthesize neurodesk/agents`

---

### Bootstrap

## new-company
**Что:** Создать компанию: workspace + SSH key + 1Password vault + AGENTS.md заготовка.
**Запуск:** `new-company <slug> <vcs[:host]> <namespace> [email]`
**См. также:** `new-project`

## new-project
**Что:** Создать продукт в компании: клонит репы + AGENTS.md заготовки + tmux launcher.
**Запуск:** `new-project <company> <product> [--ns=<override>] [repo1 repo2 ...]`
**См. также:** `new-company`, `setup-context`

---

### Agent context setup

## refresh-templates
**Что:** Перегенерить AGENTS.md из шаблонов (~5 сек, envsubst подставляет имена, без codex).
**Запуск:** `refresh-templates <co> [<prod>] [<repo>]`
**Используй:** когда изменил templates в `~/dotfiles/templates/`.

## analyze-repo
**Что:** Codex анализирует код одного репа → пишет `docs/design.md` + ADRs. ~5-15 мин.
**Запуск:** `analyze-repo [<co> <prod> <repo>]` или из cwd репа.

## analyze-product
**Что:** Codex анализирует все репы продукта → пишет `docs/ARCHITECTURE.md` + ADRs. ~10-20 мин. Запускать ПОСЛЕ analyze-repo на всех репах.
**Запуск:** `analyze-product [<co> <prod>]` или из cwd продукта.

## fill-agents-md
**Что:** Codex читает `docs/design.md` (репо) или `docs/ARCHITECTURE.md` (продукт) и сжимает в `AGENTS.md`. AUTO-SAVE. ~30-60 сек на файл.
**Запуск:** `fill-agents-md` (cwd) | `fill-agents-md <co> <prod>` | `fill-agents-md <co> <prod> <repo>`
**Зависит от:** существующий `docs/design.md` или `docs/ARCHITECTURE.md`.
**См. также:** `batch-fill-agents`, `analyze-repo`

## onboard
**Что:** Q&A wizard для одного уровня — заполняет TODO которые код не покажет (Stance, PII, Network, Docs URL).
**Запуск:**
- `onboard <co>` — company
- `onboard <co> <prod>` — product
- `onboard <co> <prod> <repo>` — repo
**См. также:** `complete-onboard`, `fill-agents-md`

## batch-setup
**Что:** Параллельно: analyze-repo × N репов + analyze-product. Per-product batch. ~30-60 мин.
**Запуск:** `batch-setup [<co> <prod>]` или из cwd продукта.
**См. также:** `setup-context`

## agents-status
**Что:** Snapshot состояния анализа: активные codex, последний лог, прогресс по docs/.
**Запуск:** `agents-status` или `agents-status <co>`
**См. также:** `agents-watch`, `agents-sessions`

## agents-watch
**Что:** Live мониторинг status — refresh каждые 10 сек.
**Запуск:** `agents-watch` или `agents-watch <co>`. Выход — Ctrl+C.
**См. также:** `agents-status`

## agents-tail
**Что:** Stream последнего batch-setup/batch-fill-agents лога (tool calls, reasoning, edits).
**Запуск:** `agents-tail` или `agents-tail <co>`. Выход — Ctrl+C (batch продолжит работать в фоне).
**См. также:** `agents-sessions`

## agents-sessions
**Что:** Live dashboard ВСЕХ codex сессий — PID, skill, репо, elapsed, прогресс. Сессии гаснут по мере завершения.
**Запуск:** `agents-sessions` или `agents-sessions <co>`. Refresh каждые 3 сек.
**См. также:** `agents-tail`, `agents-watch`

---

### Profiles

## agent
**Что:** Переключить codex/claude профиль (legacy / new / wiki) в текущей shell.
**Запуск:** `agent <profile>` или `agent status`

---

### Local infrastructure

## dev-stack
**Что:** Локальный Docker stack: Postgres :5432, Redis :6379, Prometheus :9090, Grafana :3030, MinIO :9000, MLflow :5000.
**Запуск:**
- `dev-stack up` / `dev-stack down` / `dev-stack status`
- `dev-stack psql [db]` / `dev-stack redis-cli`
- `dev-stack db-create <name>` / `dev-stack db-drop <name>`

---

### Secrets

## secret
**Что:** 1Password wrapper. `secret add VAR value` создаёт item в vault и добавляет export в `.envrc` (company auto-detect).
**Запуск:**
- `secret signin` — login
- `secret add <VAR> <VALUE>` — create item + append .envrc
- `secret get op://<vault>/<VAR>/credential`
- `secret list <vault>`

---

### Direnv

## direnv
**Что:** Авто-загрузка env vars из `.envrc` при `cd`.
**Запуск:** `direnv allow` (один раз на папку), `direnv reload`, `direnv deny`

---

### Setup

## bootstrap.sh
**Что:** Полная установка platform на свежем Mac (brew packages + codex/claude + симлинки + профили). Один раз.
**Запуск:** `~/dotfiles/bootstrap.sh`

---

### Troubleshooting

## op-not-signed-in
**Что:** Если `op` CLI не подключен → Settings → Developer → Integrate with CLI ON, потом `secret signin`.

## tmux-prefix-broken
**Что:** Если Ctrl-b не работает в tmux (Ghostty + русская раскладка). Workaround: English layout перед prefix.

## direnv-not-loading
**Что:** Если `.envrc` не подгружается → проверь `direnv allow`, hook в shell, ошибки.

## ssh-permission-denied
**Что:** Если git clone не работает → проверь публичный ключ в GitLab/GitHub Settings → SSH Keys.
