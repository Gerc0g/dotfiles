# Personal Platform Commands

Single source of truth для custom commands.

Format:
<command-name>
Что: короткое описание
Запуск: как вызывать
Примеры: (опц.)
Файлы: (опц.)
Env: (опц.)
Зависит от: (опц.)
См. также: (опц.)
Группируется по `### Category`.

---

### Discovery

## help
**Что:** Список или детали custom commands
**Запуск:** `help` (список) или `help <cmd>` (детали)

## ?
**Что:** Alias для help (короткая запись)
**Запуск:** `?` или `? <cmd>`

---

### Profiles

## agent
**Что:** Переключает codex/claude профиль в текущей shell (env vars)
**Запуск:** `agent {legacy|fresh|wiki|status}`
**Примеры:**
- `agent fresh` — основной dev профиль (по умолчанию)
- `agent wiki` — dedicated wiki профиль (используется автоматически в wikipedik)
- `agent legacy` — старый профиль с истории
- `agent status` — показать активные env vars
**Env vars:** `CODEX_HOME`, `CLAUDE_CONFIG_DIR`
**Файлы:** `~/.codex`, `~/.codex-new`, `~/.codex-wiki`, `~/.claude`, `~/.claude-new`

---

### Workspaces

## wikipedik
**Что:** tmux session с двумя codex для wiki работы (dev + research vaults параллельно)
**Запуск:** `wikipedik`
**Layout:** верх — dev vault, низ — research vault
**Файлы:** `~/Desktop/WikiPedik/{dev,research}/`
**Env:** Использует профиль `~/.codex-wiki/` (auto)
**См. также:** `agent wiki`


## agents
**Что:** Open tmux session for neurodesk/agents
**Запуск:** `agents`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/neurodesk/agents/`
**Repos:** barrier cerebellum cortex nerve synapse vox

---

### Bootstrap (новые компании/проекты)

## new-company
**Что:** Создаёт компанию: папка + AGENTS.md + SSH ключ + SSH config alias + 1Password vault + .envrc template + git identity
**Запуск:** `new-company <slug> <vcs[:host]> <namespace> [git-email]`
**Примеры:**
- `new-company acme gitlab acme-engineering egor@acme.com` — обычный gitlab.com
- `new-company acme gitlab:gitlab.acme.io acme-eng egor@acme.io` — self-hosted GitLab
- `new-company beta github beta-org` — GitHub
- `new-company beta github:ghe.beta.io beta-org` — GitHub Enterprise
- `new-company personal local local` — local-only, без VCS
**Создаёт:**
- `~/work/<slug>/` со скелетом
- `~/.ssh/<slug>_id_ed25519` (SSH ключ)
- SSH config alias `<vcs>-<slug>` в `~/.ssh/config`
- 1Password vault `Work-<slug>` (если op signed in)
- `.envrc` с git identity и закомментированными секретами
**Manual после:** добавь публичный ключ в VCS web UI, `secret add` нужные токены, `direnv allow`
**См. также:** `new-project`, `secret add`

## new-project
**Что:** Создаёт продукт в существующей компании: клонит репы через SSH alias, ставит AGENTS.md на 3 уровнях (company → product → repo), добавляет tmux launcher и документирует
**Запуск:** `new-project <company> <product> [repo1] [repo2] ...`
**Примеры:**
- `new-project acme payments backend frontend shared-schemas`
- `new-project beta auth api workers`
- `new-project personal sandbox mini-tool`
**Создаёт:**
- `~/work/<company>/<product>/` со скелетом
- AGENTS.md в product и в каждом репе
- Shell function `<product>` в `~/dotfiles/shell/30-projects.zsh`
- Запись в COMMANDS.md
**Зависит от:** company должна быть создана через `new-company`
**Использует:** SSH alias из `.company-config` для git clone

---

### Secrets (1Password)

## secret
**Что:** Wrapper над 1Password CLI с auto-helper'ами: создание items, чтение, авто-добавление export строк в .envrc
**Запуск:**
- `secret signin` — войти в 1Password
- `secret add <VAR> <VALUE>` — создать item + auto-append в .envrc (company auto-detect)
- `secret add <VAR> <VALUE> <company>` — для конкретной компании
- `secret edit <VAR> <NEW_VALUE>` — обновить существующий item
- `secret get <op://path>` — прочитать секрет
- `secret list <vault>` — items в vault
**Auto-uppercase:** имя VAR автоматически → UPPER_CASE
**Auto-detect company:** ищет ближайший `.company-config` walking up from cwd
**Хранение:** items в 1Password vault `Work-<company>` (создаётся автоматически в new-company)
**Зависит от:** `op` CLI, 1Password app signed in (Settings → Developer → Integrate with CLI)
**См. также:** `new-company` (auto-vault), direnv в `.envrc`

---

### Direnv (env auto-loading)

## direnv
**Что:** Автоматически подгружает env vars из `.envrc` при `cd` в папку
**Hook активен:** да (в `shell/40-direnv.zsh`)
**Чтобы активировать .envrc в папке:** `direnv allow` (один раз)
**Чтобы перезагрузить после правки:** `direnv reload`
**Чтобы отключить:** `direnv deny`
**См. также:** `secret add` (auto-добавляет export строки)

---

### Setup (one-time)

## bootstrap.sh
**Что:** Полная установка platform на свежем Mac
**Запуск:** `~/dotfiles/bootstrap.sh`
**Делает:**
- Ставит Homebrew если нет
- `brew bundle` из Brewfile (все CLI, casks, fonts)
- Симлинкает ghostty/tmux configs из repo
- Создаёт `~/.codex-new`, `~/.codex-wiki`, `~/.claude-new` папки
- Создаёт WikiPedik vault skeleton (если нет)
- Подключает loader в `~/.zshrc`
- Печатает manual steps remaining (login agents, SSH keys, etc.)
**Когда:** Первый раз на новом компе после `git clone`
**Idempotent:** Можно перезапускать, не сломает существующее

---

### Troubleshooting

## op-not-signed-in
**Что:** Решение если `op` CLI не подключен
**Шаги:**
- В 1Password app: Settings → Developer → "Integrate with 1Password CLI" ON
- В терминале: `eval "$(op signin)"`
- Тест: `op vault list`

## tmux-prefix-broken
**Что:** Решение если Ctrl-b не работает (русская раскладка на Ghostty)
**Workaround:** Используй English layout перед tmux командами
**Permanent fix:** Karabiner Elements remap русских Ctrl+letter → US Ctrl+letter

## direnv-not-loading
**Что:** .envrc не подгружается при cd
**Проверки:**
- `direnv allow` выполнен в этой папке?
- `eval "$(direnv hook zsh)"` в shell config? (должно быть в `shell/40-direnv.zsh`)
- Нет ли ошибок в `.envrc`? (запусти `direnv reload` для verbose)

## ssh-permission-denied
**Что:** Не клонирует репо через SSH
**Проверки:**
- Публичный ключ добавлен в GitLab/GitHub?
- Тест: `ssh -T git@<vcs>-<co>` (alias из company config)
- Если alias не работает: `cat ~/.ssh/config | grep <vcs>-<co>`

### Local infrastructure

## dev-stack
**Что:** Shared local dev stack (Postgres + Redis + Grafana + Prometheus) через Docker Compose
**Запуск:**
- `dev-stack up` — поднять все сервисы
- `dev-stack down` — остановить (данные сохраняются)
- `dev-stack status` — что запущено
- `dev-stack logs [service]` — логи
- `dev-stack psql [db]` — открыть psql
- `dev-stack redis-cli` — открыть redis-cli
- `dev-stack db-create <name>` — создать БД
- `dev-stack db-drop <name>` — удалить БД (с подтверждением)
- `dev-stack nuke` — снести всё с данными (с подтверждением)
**Сервисы:**
- Postgres `localhost:5432` (dev/dev)
- Redis `localhost:6379`
- Prometheus `http://localhost:9090`
- Grafana `http://localhost:3030` (dev/dev)
**Подключение из проектов:** в `.envrc` пиши `export DATABASE_URL=postgresql://dev:dev@localhost:5432/<db>`
**Идемпотентно:** stack можно держать постоянно или поднимать только когда работаешь — ~500 MB RAM в idle
**Зависит от:** Docker (OrbStack)
**Файлы:** `~/dotfiles/services/dev-stack/`

## legacy
**Что:** Open tmux session for neurodesk/legacy
**Запуск:** `legacy`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/neurodesk/legacy/`
**Repos:** agent_core intent_srv photo_handler vector_ingest

## saas
**Что:** Open tmux session for neurodesk/saas
**Запуск:** `saas`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/neurodesk/saas/`
**Repos:** backend frontend watchtower widget

## infra
**Что:** Open tmux session for neurodesk/infra
**Запуск:** `infra`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/neurodesk/infra/`
**Repos:** infra

## wiki
**Что:** Open tmux session for neurodesk/wiki
**Запуск:** `wiki`
**Файлы:** `/Users/_gerc0g/Desktop/Prokectfiles/neurodesk/wiki/`
**Repos:** nrdsk_wiki
