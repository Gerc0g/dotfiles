# dotfiles

Personal Internal Developer Platform.

## Quick start (новый Mac)

```bash
git clone git@github.com:Gerc0g/dotfiles.git ~/dotfiles
~/dotfiles/install.sh
source ~/.zshrc
```

## Daily use

- `?` — список всех custom commands
- `? <cmd>` — детали команды

## Структура

- `shell/` — zsh functions (auto-sourced через `_loader.zsh`)
- `docs/COMMANDS.md` — single source of truth для commands
- `templates/` — AGENTS.md и другие шаблоны
- `scripts/` — bootstrap helpers (new-project, etc)
- `services/` — local dev stack (Docker Compose)
