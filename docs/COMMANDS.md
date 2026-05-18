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
