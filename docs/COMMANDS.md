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

### System status

## status
**Что:** Красивый full-screen TUI для текущего pressure: RAM/swap/compressor/wired, active agent slots, tmux sessions, test/oracle panes и top offenders. Показывает `OK/WARN/CRITICAL` относительно локального лимита активных агентов.
**Запуск:**
- `status` — открыть live dashboard, auto-refresh каждые 5 сек.
- `status --once` — напечатать snapshot без TUI.
**Настройки:** `STATUS_LOCAL_AGENT_LIMIT=2` — локальный лимит Claude+Codex процессов для alerting.
**Выход:** `q`, `Esc` или `Ctrl+C`. Refresh вручную — `r`.

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
**Что:** Запустить agent workspace для одного репо с 4-pane layout. Daily code work всегда идёт через managed git worktree, не через shared main/dev checkout.
**Запуск:**
- `launch` — interactive picker: company → product → repo
- `launch <co>` — указал компанию, пикер для product + repo, default task `work`
- `launch <co> <prod>` — пикер для repo, default task `work`
- `launch <co> <prod> <repo>` — direct repo, default task `work`
- `launch <co> <prod> <repo> <task>` — fully direct with custom task
**Panes:**
- 🧠 **plan** — codex gpt-5.4 high reasoning (architect / planner)
- 💻 **code** — claude code (implementer)
- 🧪 **test** — `test-tui`: reads `Makefile`, `.agents/commands.toml`, and package manifests; saves run logs to `.agents/test-runs/`
- 🔮 **oracle** — `oracle-tui`: prompt + local answer history in `.agents/oracle/`, backed by `@steipete/oracle`
**Зависит от:** tmux, codex, claude, oracle (`npm i -g @steipete/oracle`)


## agent-workspace
**Что:** нижнеуровневое управление managed git worktrees. Обычные project shortcuts (`agents synapse ticket-quality`, `healler epic-04`) вызывают это автоматически.
**Запуск:**
- `agent-workspace launch <co> <prod> <repo> <task>` — создать worktree и открыть стандартный 4-pane layout.
- `agent-workspace list` — список agent worktrees с id/task/branch/state/dirty.
- `agent-workspace status` — `git status` по всем agent worktrees.
- `agent-workspace cleanup [--days N] [--dry-run]` — удалить только clean + pushed + ready/merged worktrees.
- `agent-workspace open <wt-path> | <co> <prod> <repo> <id>` — переоткрыть стандартный 4-pane layout на СУЩЕСТВУЮЩЕМ worktree (без создания нового id). Метаданные берутся из `.agent-workspace`.
- `agent-workspace reopen-all [company]` — универсально поднять detached-сессию на каждом worktree без живой tmux-сессии (после `tmux kill-server`): и managed (`.agent-workspace`), и ручные `git worktree add` — для всех co/prod/repo. Identity из пути `.worktrees/<co>/<prod>/<repo>/<id>`; вложенные репо/submodule (`.git`-директория) пропускаются. Опциональный фильтр по компании. Дальше `tmux attach -t <name>`.
- `agent-workspace ready <co> <prod> <repo> <id>` — вручную пометить clean+pushed worktree готовым к cleanup.
- `agent-workspace remove <co> <prod> <repo> <id>` — удалить clean worktree по короткому id.
- `agent-workspace stale [days]` — active worktrees без tmux-сессии и коммитов N+ дней (default 3) для ручного triage; `start` подсказывает их количество.
- `agent-workspace prune-branches [--dry-run]` — удалить `agent/*` ветки, не привязанные ни к одному worktree и при этом merged в base или полностью pushed; всё остальное keeping с причиной. `remove`/`cleanup` делают то же для ветки удаляемого worktree автоматически.
**Git:** base branch = `dev`, иначе `main`; path = короткий id; branch = `agent/<task>-<id>`.
**Salvage:** перед удалением worktree (`remove`/`cleanup`/`reap`) oracle-ответы (`.agents/oracle/*.md`) и незакоммиченные `docs/epics/*.md` спасаются в WikiPedik `repos/<repo>/_salvage/<id>/`; куратор разбирает их при `wiki sync`. Untracked epic-доки после спасения удаляются (worktree становится removable), изменённые tracked-файлы по-прежнему блокируют remove.
**Reap при закрытии tmux:** `launch` вешает `session-closed` hook на managed-сессию → при закрытии окна вызывается `agent-workspace reap <wt>`: salvage + удаление worktree ТОЛЬКО если он clean+pushed (или `ready`); dirty/unpushed работа остаётся нетронутой (её подберут `stale`/`cleanup`). Лог: `~/Library/Logs/agent-workspace-reap.log`.
**Индикатор в статусбаре:** managed-сессия показывает в status-right живой reap-индикатор (обновление 10с): `●synced` (зелёный) — закрытие удалит worktree; `●3✎ 2↑ hold` (жёлтый) — есть незакоммиченное (✎) / незапушенное (↑), закрытие сохранит. Команда: `agent-workspace reap-status <wt>`.
**VS Code:** start/launch/ready/remove/cleanup автоматически обновляют Project Manager через `vscode-projects-sync`.

## vscode-projects-sync
**Что:** Синхронизировать `~/Desktop/Prokectfiles/<company>/<product>/<repo>` и активные `~/Desktop/Prokectfiles/.worktrees/...` с VS Code Project Manager (`alefragnani.project-manager`). По умолчанию Project Manager становится зеркалом платформенных product/repo/worktree entries; старые внешние записи удаляются. Управляемые записи помечаются тегом `dotfiles`, перед ручной записью делает backup `projects.json`.
**Naming:** repo entries называются коротко (`synapse`), product entries — `Product: company/product`, worktree entries — `repo @ id · task`; company/product/state/branch доступны через tags.
**Запуск:**
- `vscode-projects-sync` — добавить/обновить product + repo + worktree entries и убрать внешние старые entries.
- `vscode-projects-sync --dry-run` — показать изменения без записи.
- `vscode-projects-sync --check` — вернуть exit 1 если файл не синхронизирован.
- `vscode-projects-sync --repos-only` — синхронизировать только репозитории.
- `vscode-projects-sync --products-only` — синхронизировать только продукты.
- `vscode-projects-sync --no-worktrees` — не добавлять agent worktree entries.
- `vscode-projects-sync --preserve-external` — сохранить проекты, добавленные вручную вне dotfiles.
**Файл:** `~/Library/Application Support/Code/User/globalStorage/alefragnani.project-manager/projects.json`

## agent-commit
**Что:** локальный атомарный commit для agent worktree.
**Запуск:**
- `agent-commit.sh "feat(scope): русское описание" -- <explicit paths>` — commit после завершённого logical change.
**Правила:** explicit paths only; no `git add .`, no force, no rebase, не push.

## agent-finish
**Что:** финал agent-задачи: verify/test, push branch, открыть draft PR/MR в integration branch, записать review metadata.
**Запуск:**
- `agent-finish.sh` — основной финальный flow.
- `agent-finish.sh --base dev --title "feat(scope): описание"` — override base/title.
- `agent-task-push.sh` — низкоуровневый fallback push без PR/MR.
**Правила:** push не на каждый commit; merge не выполняется автоматически.

## wikipedik
**Что:** tmux с тремя codex-панелями + меню памяти для Obsidian vault: project memory, research и personal brand tracker.
**Запуск:** `wikipedik`
**Панели:**
- `~/Desktop/WikiPedik/dev` — project-memory curator (`wiki sync/status/synthesize`).
- **меню памяти** (`tools/wiki-tui`, bubbletea; launcher `scripts/wiki-menu.sh`) — композер промптов: выбираешь действие (status/sync/synthesize/lint/ingest) и scope (компания→продукт→репо, стрелки/цифры) — готовый промпт ложится в буфер и впечатывается в инпут dev-чата, фокус прыгает туда, ты жмёшь Enter. Механика (rules-sync+hot-refresh, push, git status, bootstrap репо без памяти) выполняется сразу с выводом. Шпаргалка: `docs/wikipedik-cheatsheet.md`.
- `~/Desktop/WikiPedik/research` — research/study agent для источников, статей, концептов и study notes.
- `~/Desktop/WikiPedik/Personal Brand` — изолированный brand-agent для идей, черновиков, inbound и weekly metrics.
**Правила brand-agent:**
- Пишет и отвечает на русском; English content делает только по явной просьбе и отдельной копией ` - EN.md`.
- SessionStart hook напоминает после изменений сделать commit+push vault state в GitHub.
- Git покрывает весь vault `~/Desktop/WikiPedik`: `dev`, `research`, `Personal Brand` и Obsidian metadata.
- Commit message: English Conventional Commit type/scope + русское описание.

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
- `wiki autocommit` / `wiki-autocommit` — catch-all commit всего dirty вольта (без push, с secret-scan); `launch` вызывает его раз в сутки через `--if-due`.
- `wiki-hot-refresh <co>[/<prod>[/<repo>]]` — детерминированно пересобрать `hot.md` из curated-страниц + дайджест «Binding rules» (fallback, если куратор пропустил свою фазу).
- `wiki rules-sync [<co>[/<prod>[/<repo>]]]` — материализовать правила вольта (`repos/<repo>/rules/`, `shared/rules/`) в path-scoped `.claude/rules/wiki-*` симлинки (основные чекауты + активные worktrees); правила без `paths:` отклоняются; удалённые из вольта — prune.
- `agent-session-digest.py --source claude|codex|all [--since D] [--until D] [--project substr]` — сжать JSONL-сессии в markdown-дайджесты для майнинга куратором (`10-wiki/sources/sessions/_digests/`); secrets редактируются, объём падает с MB до десятков KB.
- `wiki-git <args...>` — выполнить `git` внутри root vault `~/Desktop/WikiPedik`.
**Примеры:**
- `wiki sync neurodesk/wiki/nrdsk_wiki`
- `wiki sync --commit neurodesk/wiki/nrdsk_wiki`
- `wiki-git status`
- `wiki status neurodesk`
- `wiki synthesize neurodesk/agents`

**Git sync policy:**
- `wiki sync` never commits by itself.
- `wiki sync --commit/--push` commits atomically by scope and uses company `git_email` from `.company-config` when present.
- WikiPedik commits go directly to WikiPedik `main`; app repo `feature → dev → main` delivery rules do not apply there.
- Commit message format remains platform-wide: English Conventional Commit type/scope, Russian description.
- Daily agents write only `_inbox.md`; curator commands own curated pages and git sync.


## aetheria
**Что:** Open tmux session for chimera/aetheria
**Запуск:** `aetheria`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/aetheria/`
**Repos:** Aetheria-App aetheria-frontend Aetheria-Manifests Aetheria-AI

## homeless
**Что:** Open tmux session for chimera/homeless
**Запуск:** `homeless`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/homeless/`
**Repos:** Jarvis Homeless Healler Hiring-Radar-Bot

## neuroslop4ik
**Что:** Open tmux session for chimera/neuroslop4ik
**Запуск:** `neuroslop4ik`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/neuroslop4ik/`
**Repos:** NeuroSlop4ik

## comonline
**Что:** Open tmux session for chimera/comonline
**Запуск:** `comonline`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/comonline/`
**Repos:** ComaOnline-Sources

## clients
**Что:** Open tmux session for SupportOps-Core/clients
**Запуск:** `clients`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/SupportOps-Core/clients/`
**Repos:** adminka

## cerebro
**Что:** Open tmux session for neurodesk/cerebro
**Запуск:** `cerebro`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/neurodesk/cerebro/`
**Repos:** cerebro

## <company> (neurodesk / chimera / SupportOps-Core)
**Что:** Вход с уровня компании: продукт → репо → launch. Команда на каждую компанию с `.company-config` регистрируется автоматически.
**Запуск:** `neurodesk` (выбор продукта, затем репо) · `neurodesk agents` (выбор репо) · `neurodesk agents synapse` (сразу launch)
**Иерархия:** компания (`neurodesk`) → продукт (`agents`) → продукт-шорткат сразу (`agents`).


## pizduk
**Что:** Open tmux session for chimera/pizduk
**Запуск:** `pizduk`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/pizduk/`
**Repos:** Pizduk
---

### Bootstrap

## new-company
**Что:** Создать компанию: workspace + SSH key + 1Password vault + AGENTS.md заготовка.
**Запуск:** `new-company <slug> <vcs[:host]> <namespace> [email]`
**См. также:** `new-project`

## new-project
**Что:** Создать продукт в компании: клонит репы + AGENTS.md заготовки + tmux launcher + sync VS Code Project Manager.
**Запуск:** `new-project <company> <product> [--ns=<override>] [repo1 repo2 ...]`
**См. также:** `new-company`, `setup-context`

---

### Agent context setup

## agent-skill
**Что:** Управление repo-owned skills: создать шаблон, установить symlink в нужные runtime profiles, проверить frontmatter, links и isolation между daily/setup/wiki.
**Запуск:**
- `agent-skill list` — показать skills из `~/dotfiles/skills` и статус установки daily/setup/wiki
- `agent-skill new <name>` — создать `~/dotfiles/skills/<name>/SKILL.md`
- `agent-skill install` — поставить symlinks: setup skills в setup profiles, universal skills во все профили
- `agent-skill doctor` — проверить YAML frontmatter, symlinks и отсутствие setup-only skills в daily/wiki profiles
**Используй:** когда добавляешь/меняешь reusable skill. Для agent-led изменений сначала используй `skill-maintainer`.

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
**Что:** Переключить codex/claude профиль (legacy / fresh / setup / wiki) в текущей shell.
**Запуск:** `agent <profile>` или `agent status`

---

### Local infrastructure

## dev-stack
**Что:** Единый локальный Docker-стек инфры для всех проектов. Сервисы: Postgres :5432, Redis :6379, MinIO :9000/9001, Qdrant :6333, ClickHouse :8123, Prometheus :9090, Grafana :3030, Loki :3100, Tempo :3200, OTel Collector :4317/4318, Langfuse :3001, Ollama :11434, Metabase :3002, CloudBeaver :8978 (+ Traefik/Homepage). Host = `$DEV_STACK_HOST` (default **gerc0g** — домашний Ubuntu по Tailscale, 24/7; `localhost` для Mac-local). Remote-aware: управление идёт на хост через `ssh $DEV_STACK_SSH` (default ubuntu-server). Телеметрия PUSH через OTLP (Prometheus сам не скрейпит). Источник правды: `services/dev-stack/`.
**Запуск:**
- `dev-stack connect [prod repo]` — самоонбординг проекта: блок в ./.envrc + БД `<product>__<repo>` + direnv allow
- `dev-stack doctor` — достижимость стека + подключён ли текущий проект
- `dev-stack up [svc...]` (всё или точечно) / `dev-stack down` / `dev-stack status` / `dev-stack pull`
- `dev-stack urls` — все адреса (учитывает DEV_STACK_HOST)
- `dev-stack psql [db]` / `dev-stack redis-cli` / `dev-stack clickhouse`
- `dev-stack envrc [prod repo]` (печать блока) / `dev-stack db-ensure <name>` / `dev-stack db-create <name>` / `dev-stack db-drop <name>` / `dev-stack logs [svc]` / `dev-stack nuke`
**Файлы:** `services/dev-stack/docker-compose.yml`, `services/dev-stack/config/`, `shell/60-devstack.zsh`.

---

### Secrets

## secret
**Что:** 1Password wrapper. Один vault на компанию (`Work-<co>`), но item names и `.envrc` scoped по company/product/repo. Default scope определяется из cwd, включая managed worktree.
**Запуск:**
- `secret signin` — login
- `secret add --repo <VAR> <VALUE>` — item `<product>__<repo>__<VAR>` + repo `.envrc`
- `secret add --product <VAR> <VALUE>` — item `<product>__<VAR>` + product `.envrc`
- `secret add --company <VAR> <VALUE>` — item `_company__<VAR>` + company `.envrc`
- `secret name --repo <VAR>` — показать 1Password item name без значения
- `secret envline --repo <VAR>` — показать export line без значения
- `secret list <vault>`

---

### Direnv

## direnv
**Что:** Авто-загрузка env vars из `.envrc` при `cd`.
**Запуск:** `direnv allow` (один раз на папку), `direnv reload`, `direnv deny`

---

### Setup

## transcript-scrub
**Что:** Маскирует секреты в JSONL-транскриптах сессий (`~/.codex/sessions`, `~/.claude/projects`) и дайджестах вольта. Маскирует, не удаляет: оставляет узнаваемый префикс (`ghp_***MASKED***`), чтобы было видно, какой ключ утёк. Ловит github/gitlab/anthropic/openai/openrouter/grafana-sa/langsmith/aws/slack/npm/hf/stripe/google/sendgrid/telegram/jwt + bearer-токены, пароли в conn-string, `KEY=value`/`"api_key": "..."` присвоения; плейсхолдеры (`${VAR}`, `xxxx`, `REPLACE_ME`) пропускает. Сохраняет mtime (не ломает ingest-манифест), пропускает файлы моложе 24ч (живые сессии), JSON остаётся валидным, идемпотентен.
**Запуск:**
- автоматически — launchd-джоба `com.gerc0g.transcript-scrub` ежедневно в 03:30, лог `~/Library/Logs/transcript-scrub.log`.
- вручную — `python3 ~/dotfiles/scripts/transcript-scrub.py [--dry-run] [--min-age-hours N]`.
**Файлы:** `scripts/transcript-scrub.py`, `scripts/launchd/com.gerc0g.transcript-scrub.plist`.

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
