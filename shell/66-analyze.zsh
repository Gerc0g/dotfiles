# shell functions: analyze-product, analyze-repo
#
# Target: ~/dotfiles/shell/66-analyze.zsh
#
# ### Workspaces
#
# ## analyze-product
# **Что:** Deep analysis of product architecture. Generates docs/ARCHITECTURE.md + docs/adr/ from code.
# **Запуск:**
# - `analyze-product` — current dir (must be product dir)
# - `analyze-product <co> <prod>` — cd to product first
# **Зависит от:** codex CLI, ~/dotfiles/skills/analyze-product/SKILL.md
# **См. также:** `onboard`, `new-project`, `analyze-repo`
#
# ## analyze-repo
# **Что:** Deep analysis of single repo. Generates docs/design.md + docs/adr/ from source code.
# **Запуск:**
# - `analyze-repo` — current dir (must be repo with .git/)
# - `analyze-repo <co> <prod> <repo>` — cd to repo first
# **Зависит от:** codex CLI, ~/dotfiles/skills/analyze-repo/SKILL.md
# **См. также:** `onboard`, `new-project`, `analyze-product`

analyze-product() {
  local target
  case $# in
    0) target="$PWD" ;;
    2) target="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$1/$2" ;;
    *) echo "Usage: analyze-product [<company> <product>]"; return 1 ;;
  esac

  if [ ! -d "$target" ]; then
    echo "⚠ Path not found: $target"
    return 1
  fi

  cd "$target" || return 1

  if [ ! -f .product-config ]; then
    echo "⚠ Not a product directory (no .product-config in $(pwd))"
    echo "  Run 'new-project <co> <prod>' first, or cd to product dir."
    return 1
  fi

  echo "→ Analyzing product at $(pwd)"
  echo "  Skill: analyze-product"
  echo "  Output: docs/ARCHITECTURE.md + docs/adr/"
  echo "  Launching codex..."
  echo ""

  export CODEX_HOME="$HOME/.codex-setup"
  export CLAUDE_CONFIG_DIR="$HOME/.claude-setup"

  codex "Use skill analyze-product. cwd is product dir. Read all repos in subdirectories, identify services + inter-service communication + data flow + tech stack. Draft docs/ARCHITECTURE.md and propose docs/adr/ files. Don't auto-save — show me drafts first."
}

analyze-repo() {
  local target
  case $# in
    0) target="$PWD" ;;
    3) target="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$1/$2/$3" ;;
    *) echo "Usage: analyze-repo [<company> <product> <repo>]"; return 1 ;;
  esac

  if [ ! -d "$target" ]; then
    echo "⚠ Path not found: $target"
    return 1
  fi

  cd "$target" || return 1

  if [ ! -d .git ]; then
    echo "⚠ Not a git repo at $(pwd)"
    return 1
  fi

  echo "→ Analyzing repo at $(pwd)"
  echo "  Skill: analyze-repo"
  echo "  Output: docs/design.md + docs/adr/"
  echo "  Launching codex..."
  echo ""

  export CODEX_HOME="$HOME/.codex-setup"
  export CLAUDE_CONFIG_DIR="$HOME/.claude-setup"

  codex "Use skill analyze-repo. cwd is repo root. Read source code, manifest, README, recent git log. Derive module structure, key abstractions, external dependencies, non-obvious patterns. Draft docs/design.md and propose docs/adr/ files. Don't auto-save — show me drafts first."
}
