# Commands Reference

Single source of truth for all custom commands.

Format:
<command-name>
Что: короткое описание
Запуск: как вызывать
Файлы: какие файлы трогает (опц.)
Env: какие env vars (опц.)
См. также: связанные команды (опц.)
---

## help
**Что:** Список или детали custom commands
**Запуск:** `help` (список) / `help <cmd>` (детали)

## ?
**Что:** Alias для help (короткая запись)
**Запуск:** `?` / `? <cmd>`

## agent
**Что:** Переключает codex/claude профиль в текущей shell
**Запуск:** `agent {legacy|fresh|wiki|status}`
**Env vars:** `CODEX_HOME`, `CLAUDE_CONFIG_DIR`
**См. также:** `wikipedik`

## wikipedik
**Что:** Открывает tmux session с двумя codex (dev + research vaults)
**Запуск:** `wikipedik`
**Файлы:** `~/Desktop/WikiPedik/{dev,research}/`
**Env:** Использует профиль `~/.codex-wiki/`
**См. также:** `agent wiki`


## new-company
**Что:** Bootstrap новой компании (one-time setup): AGENTS.md, .envrc template, .company-config
**Запуск:** `new-company <slug> <vcs> <namespace>`
**Примеры:**
  - `new-company acme gitlab acme-engineering`
  - `new-company beta github beta-org`
  - `new-company personal local local`
**Файлы:** Создаёт `~/work/<slug>/` со скелетом
**См. также:** `new-project` (добавление продуктов в компанию)

## new-project
**Что:** Bootstrap нового проекта внутри существующей компании
**Запуск:** `new-project <company> <product> [repo1] [repo2] ...`
**Примеры:**
  - `new-project acme payments backend frontend shared-schemas`
  - `new-project beta auth api workers`
**Файлы:** Создаёт `~/work/<company>/<product>/`, обновляет `shell/30-projects.zsh` и `COMMANDS.md`
**Зависит от:** company должна быть создана через `new-company`
**См. также:** `new-company`

## bootstrap.sh
**Что:** Полная установка platform на свежем Mac (brew, packages, configs, dirs, loader)
**Запуск:** `~/dotfiles/bootstrap.sh`
**Когда использовать:** Первый раз на новом компе после `git clone`
**Файлы:** Brewfile, ghostty/config, tmux/tmux.conf, ~/.zshrc, ~/.codex-*/, ~/Desktop/WikiPedik/
**Notes:** Идемпотентный — можно запускать повторно, не сломает существующий setup

## secret
**Что:** Wrapper над 1Password CLI с auto-helper'ами для секретов
**Запуск:**
  - `secret signin` — войти в 1Password
  - `secret add <VAR> <VALUE>` — создать item + добавить в .envrc (company auto-detect из cwd)
  - `secret add <VAR> <VALUE> <company>` — для конкретной компании
  - `secret edit <VAR> <NEW_VALUE>` — обновить существующий
  - `secret get <op://path>` — прочитать
  - `secret list <vault>` — items в vault
**Auto-uppercase:** имя переменной автоматически приводится к UPPER_CASE
**Auto-detect company:** ищет ближайший `.company-config` walking up from cwd
**Зависит от:** `op` CLI, 1Password app signed in
**См. также:** `new-company` (создаёт пустой vault), direnv
