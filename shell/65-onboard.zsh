# shell function `onboard` — interactive AGENTS.md TODO filler
#
# Target: ~/dotfiles/shell/65-onboard.zsh
# Source-load: автоматически через `~/dotfiles/shell/_loader.zsh`
#
# ### Workspaces
#
# ## onboard
# **Что:** Launch codex with onboard-agents-md skill to interactively fill AGENTS.md TODOs
# **Запуск:**
# - `onboard` — onboards current dir (auto-detect level)
# - `onboard <co>` — cd into ~/Desktop/Prokectfiles/<co> first, then onboard
# - `onboard <co> <prod>` — cd into product
# - `onboard <co> <prod> <repo>` — cd into repo
# **Зависит от:** codex CLI, ~/dotfiles/skills/onboard-agents-md/SKILL.md
# **См. также:** `new-company`, `new-project`

onboard() {
  local target

  case $# in
    0) target="$PWD" ;;
    1) target="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$1" ;;
    2) target="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$1/$2" ;;
    3) target="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$1/$2/$3" ;;
    *) echo "Usage: onboard [company] [product] [repo]"; return 1 ;;
  esac

  if [ ! -d "$target" ]; then
    echo "⚠ Path not found: $target"
    return 1
  fi

  cd "$target" || return 1

  if [ ! -f "AGENTS.md" ]; then
    echo "⚠ AGENTS.md not found in $(pwd)"
    echo "  Run 'new-company <slug> ...' or 'new-project <co> <prod> ...' first."
    return 1
  fi

  # Detect level
  local level="repo"
  [ -f .product-config ] && level="product"
  [ -f .company-config ] && level="company"

  # Count TODOs
  local todo_count
  todo_count=$(grep -cE '(^|>)\s*TODO|<!-- TODO:' AGENTS.md 2>/dev/null || echo 0)

  echo "→ Onboarding $level AGENTS.md at $(pwd)"
  echo "  $todo_count TODO placeholders found"
  echo "  Launching codex with onboard-agents-md skill..."
  echo ""

  export CODEX_HOME="$HOME/.codex-setup"
  export CLAUDE_CONFIG_DIR="$HOME/.claude-setup"

  # Initial prompt — skill подхватит автоматически по триггеру "заонбордить"
  codex "Use skill onboard-agents-md. Detect current level ($level), read questions/$level.md, then walk me through filling TODO placeholders in AGENTS.md one question at a time."
}
