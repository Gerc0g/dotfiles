# Personal Platform Commands

Single source of truth for custom commands.

Format:
<command-name>
Что: короткое описание
Запуск: как вызывать
Примеры: (опц.)
Файлы: (опц.)
Env: (опц.)
См. также: (опц.)
Категории через `### Section`.

---

### Discovery

## help
**Что:** Список или детали custom commands
**Запуск:** `help [<cmd>]`

## ?
**Что:** Alias для help (короткая запись)
**Запуск:** `?` / `? <cmd>`

---

### Profiles

## agent
**Что:** Переключает codex/claude профиль в текущей shell
**Запуск:** `agent {legacy|fresh|wiki|status}`
**Env vars:** `CODEX_HOME`, `CLAUDE_CONFIG_DIR`

---

### Workspaces

## wikipedik
**Что:** tmux session с двумя codex для wiki (dev + research vaults)
**Запуск:** `wikipedik`
**Файлы:** `~/Desktop/WikiPedik/{dev,research}/`
**Env:** Профиль `~/.codex-wiki/`

---

### Bootstrap (новые компании/проекты)

## new-company
**Что:** Создаёт компанию: папка + AGENTS.md + SSH key + SSH config alias + 1Password vault + .envrc + git identity
**Запуск:** `new-company <slug> <vcs[:host]> <namespace> [git-email]`
**Примеры:**
- `new-company acme gitlab acme-engineering egor@acme.com`
- `new-company acme gitlab:gitlab.acme.io acme-eng egor@acme.io` (self-hosted)
- `new-company beta github beta-org`
- `new-company personal local local`
**Файлы:** `~/work/<slug>/`, `~/.ssh/<slug>_id_ed25519`, `~/.ssh/config`
**См. также:** `new-project`, `secret add`

## new-project
**Что:** Создаёт продукт в существующей компании: клонит репы через SSH alias, ставит AGENTS.md на 3 уровнях, добавляет tmux launcher
**Запуск:** `new-project <company> <product> [repo1] [repo2] ...`
**Примеры:**
- `new-project acme payments backend frontend shared-schemas`
- `new-project beta auth api workers`
**Зависит от:** company должна быть создана через `new-company`
**См. также:** `new-company`

---

### Secrets (1Password)

## secret
**Что:** Wrapper над 1Password CLI: создание items, чтение, авто-добавление в .envrc
**Запуск:**
- `secret signin` — login to 1Password
- `secret add <VAR> <VALUE> [company]` — создать item + добавить в .envrc
- `secret edit <VAR> <NEW_VALUE>` — обновить существующий
- `secret get <op://path>` — прочитать
- `secret list <vault>` — items в vault
**Auto-detect:** company определяется по `.company-config` walking up from cwd
**Зависит от:** `op` CLI, 1Password app signed in

---

### Setup (one-time)

## bootstrap.sh
**Что:** Полная установка platform на свежем Mac (Brew, packages, configs, dirs, loader)
**Запуск:** `~/dotfiles/bootstrap.sh`
**Когда:** Первый раз на новом компе после `git clone`
**Notes:** Идемпотентный — можно перезапускать
