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
**Что:** Красивый full-screen TUI для текущего pressure: RAM/swap/compressor/wired, active agent slots, tmux-сессии и top offenders. Показывает `OK/WARN/CRITICAL` относительно локального лимита активных агентов.
**Запуск:**
- `status` — открыть live dashboard, auto-refresh каждые 5 сек.
- `status --once` — напечатать snapshot без TUI.
**Настройки:** `STATUS_LOCAL_AGENT_LIMIT=2` — локальный лимит Claude+Codex процессов для alerting.
**Выход:** `q`, `Esc` или `Ctrl+C`. Refresh вручную — `r`.

---

### Main flow

## agent-workspace
**Что:** управление изолированными git worktree под задачи агента. Логика в ядре (`hq workspace`, алиас `hq ws`); shell-обёртка нужна только чтобы перейти в каталог и открыть редактор. Ярлыки продуктов (`agents synapse ticket-quality`, `healler epic-04`) вызывают это автоматически.
**Запуск:**
- `agent-workspace start <co> <prod> <repo> <task>` — создать worktree, перейти в него и открыть в редакторе.
- `agent-workspace open <wt-path> | <co> <prod> <repo> <id>` — вернуться в существующий worktree (нового id не создаётся).
- `agent-workspace list` — список worktree с id/task/branch/state/dirty.
- `agent-workspace stale [days]` — worktree без коммитов N+ дней (по умолчанию 3) для ручного разбора; `start` подсказывает их количество.
- `agent-workspace ready <co> <prod> <repo> <id>` — пометить clean+pushed worktree готовым к уборке.
- `agent-workspace remove <co> <prod> <repo> <id>` — удалить clean worktree по короткому id.
- `agent-workspace cleanup [--days N] [--dry-run]` — удалить только clean + pushed + ready/merged worktree.
- `agent-workspace prune-branches [--dry-run]` — удалить ветки `agent/*`, не привязанные ни к одному worktree и при этом merged в base или полностью pushed.
**Редактор:** вход в worktree открывает его в VS Code. `HQ_EDITOR` меняет бинарник, `HQ_OPEN_EDITOR=0` отключает открытие целиком — так выставлено на сервере, где GUI нет.
**Git:** base branch = `dev`, иначе `main`; каталог = короткий id; ветка = `agent/<task>-<id>`; `user.name`/`user.email` берутся из конфига компании.
**Salvage:** перед удалением worktree незакоммиченные `docs/epics/*.md` спасаются в WikiPedik `repos/<repo>/_salvage/<id>/`. Изменённые tracked-файлы по-прежнему блокируют удаление.
**Автоудаления нет:** worktree удаляется только явно (`remove`) или через `cleanup` (нужен флаг ready и 7 дней). Автоматическая уборка при старте когда-то дважды снесла живую работу и была убрана.
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
**Что:** перейти в зону вольта WikiPedik (`dev` по умолчанию, также `research` и `brand`) и показать число незакоммиченных файлов.
**Запуск:** `wikipedik`
**Панели:**
- `~/Desktop/WikiPedik/dev` — project-memory curator (`wiki sync/status/synthesize`).
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
- `wiki` — создать worktree в `neurodesk/wiki/nrdsk_wiki` и перейти в него.
- `wiki sync <company[/product[/repo]]>` — curator drain: `_inbox.md` → `lessons.md` / `gotchas.md` / etc. через `inbox-drain`.
- `wiki sync --commit <scope>` — drain + локальный commit WikiPedik changes. Заранее dirty файлы разрешены только внутри указанного scope.
- `wiki sync --push <scope>` — drain + commit + push.
- `wiki status <company[/product[/repo]]>` — отчёт по pending inbox, curated pages, synthesis candidates.
- `wiki synthesize <company/product>` — найти повторяющиеся lessons/gotchas и предложить product shared patterns.
- `wiki bootstrap <company> [product] [repo]` — создать WikiPedik skeleton + repo symlinks.
- `wiki-commit <scope>` — вручную закоммитить только changes внутри WikiPedik scope.
- `wiki autocommit` / `wiki-autocommit` — catch-all commit всего dirty вольта (без push, с secret-scan). Автоматического вызывающего сейчас НЕТ (запускать вручную; `--if-due` оставлен для будущего планировщика).
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
**Что:** Открыть воркспейс chimera/aetheria
**Запуск:** `aetheria`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/aetheria/`
**Repos:** Aetheria-App aetheria-frontend Aetheria-Manifests Aetheria-AI

## homeless
**Что:** Открыть воркспейс chimera/homeless
**Запуск:** `homeless`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/homeless/`
**Repos:** Jarvis Homeless Healler Hiring-Radar-Bot

## neuroslop4ik
**Что:** Открыть воркспейс chimera/neuroslop4ik
**Запуск:** `neuroslop4ik`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/neuroslop4ik/`
**Repos:** NeuroSlop4ik

## comonline
**Что:** Открыть воркспейс chimera/comonline
**Запуск:** `comonline`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/chimera/comonline/`
**Repos:** ComaOnline-Sources

## clients
**Что:** Открыть воркспейс SupportOps-Core/clients
**Запуск:** `clients`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/SupportOps-Core/clients/`
**Repos:** adminka

## cerebro
**Что:** Открыть воркспейс neurodesk/cerebro
**Запуск:** `cerebro`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/neurodesk/cerebro/`
**Repos:** cerebro

## <company> (neurodesk / chimera / SupportOps-Core)
**Что:** Вход с уровня компании: продукт → репо → worktree. Команда на каждую компанию с `.company-config` регистрируется автоматически.
**Запуск:** `neurodesk` (выбор продукта, затем репо) · `neurodesk agents` (выбор репо) · `neurodesk agents synapse` (сразу worktree)
**Иерархия:** компания (`neurodesk`) → продукт (`agents`) → продукт-шорткат сразу (`agents`).


## pizduk
**Что:** Открыть воркспейс chimera/pizduk
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
**Что:** Создать продукт в компании: клонит репы + AGENTS.md заготовки + ярлык продукта + sync VS Code Project Manager.
**Запуск:** `new-project <company> <product> [--ns=<override>] [repo1 repo2 ...]`
**См. также:** `new-company`

---

### Agent context setup

## agent-skill
**Что:** Управление repo-owned skills: создать шаблон, установить symlinks в оба профиля (`~/.claude/skills`, `~/.codex/skills`), проверить frontmatter и links.
**Запуск:**
- `agent-skill list` — показать skills из `~/dotfiles/skills` и статус установки в обоих профилях
- `agent-skill new <name>` — создать `~/dotfiles/skills/<name>/SKILL.md`
- `agent-skill install` — поставить symlinks каждого skill в оба профиля
- `agent-skill doctor` — проверить YAML frontmatter, symlinks и dangling-ссылки в профилях
**Используй:** когда добавляешь/меняешь reusable skill. Для agent-led изменений сначала используй `skill-maintainer`.

## dev-stack
**Что:** Единый локальный Docker-стек инфры для всех проектов. Сервисы: Postgres :5432, Redis :6379, MinIO :9000/9001, Qdrant :6333, ClickHouse :8123, Prometheus :9090, Grafana :3030, Loki :3100, Tempo :3200, OTel Collector :4317/4318, Langfuse :3001, Ollama :11434, Metabase :3002, CloudBeaver :8978 (+ Traefik/Homepage). Стек работает локально на этой машине; удалённые хосты вернутся вместе с серверным слоем (`hq server`). Телеметрия PUSH через OTLP (Prometheus сам не скрейпит). Источник правды: `services/dev-stack/`.
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
