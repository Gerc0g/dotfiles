# dotfiles

Personal Internal Developer Platform.

## Quick start (новый Mac)

```bash
git clone git@github.com:Gerc0g/dotfiles.git ~/dotfiles
~/dotfiles/bootstrap.sh
source ~/.zshrc
```

`bootstrap.sh` поставит Homebrew, все пакеты из `Brewfile`, симлинкнет ghostty/tmux конфиги, создаст профили codex/claude, разложит ~/work и ~/Desktop/WikiPedik скелеты, добавит loader в .zshrc. В конце покажет список manual шагов (логин агентов, SSH ключи).

## Daily use

```bash
?              # список всех custom commands
? <cmd>        # детали команды
```

## Adding new things

| Что | Команда |
|---|---|
| Новая компания | `new-company <slug> <vcs> <namespace>` |
| Новый продукт | `new-project <company> <product> [repos...]` |
| Открыть wiki | `wikipedik` |
| Открыть продукт | `<product-name>` (created by new-project) |
| Переключить agent profile | `agent {legacy\|fresh\|wiki\|status}` |

## Структура

- `shell/` — zsh functions (auto-sourced через `_loader.zsh`)
- `ghostty/config` — Ghostty config (symlinked to ~/.config/ghostty/config)
- `tmux/tmux.conf` — tmux config (symlinked to ~/.tmux.conf)
- `docs/COMMANDS.md` — single source of truth для commands
- `templates/` — AGENTS.md и другие шаблоны
- `scripts/` — bootstrap helpers (new-company, new-project)
- `services/` — local dev stack (Docker Compose)
- `Brewfile` — declarative список brew пакетов

## Конвенция

Всё в одном месте → один `bootstrap.sh` на новом Mac → 5 минут до productive setup.
