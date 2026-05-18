#!/bin/bash
# Bootstrap a project within an existing company
# Usage: new-project <company-slug> <product> [repo1] [repo2] ...
set -e

if [ "$#" -lt 2 ]; then
  cat <<'USAGE'
Usage: new-project <company-slug> <product> [repo1] [repo2] ...

Examples:
  new-project acme payments backend frontend shared-schemas
  new-project beta auth api workers
  new-project personal sandbox mini-tool

Company must exist first. Run: new-company <slug> <vcs> <namespace>
USAGE
  exit 1
fi

CO="$1"; PROD="$2"
shift 2
REPOS=("$@")

DIR=~/work/$CO
PDIR="$DIR/$PROD"
TPL=~/dotfiles/templates
CFG="$DIR/.company-config"

# Check company exists
if [ ! -f "$CFG" ]; then
  echo "⚠ Company '$CO' not found. Run: new-company $CO <vcs> <namespace>"
  exit 1
fi

# Read company config
VCS=$(awk '/^vcs:/ {print $2}' "$CFG")
NS=$(awk '/^namespace:/ {print $2}' "$CFG")

mkdir -p "$PDIR"

# Product-level AGENTS.md
CO="$CO" PROD="$PROD" REPOS="${REPOS[*]}" envsubst < "$TPL/AGENTS.md.product.tmpl" > "$PDIR/AGENTS.md"
echo "✓ $PDIR/AGENTS.md"

# Remote URL prefix
case "$VCS" in
  gitlab)    REMOTE="git@gitlab.com:$NS" ;;
  github)    REMOTE="git@github.com:$NS" ;;
  bitbucket) REMOTE="git@bitbucket.org:$NS" ;;
  local)     REMOTE="" ;;
  *)         echo "Unknown VCS: $VCS"; exit 1 ;;
esac

# Clone repos + repo AGENTS.md
for repo in "${REPOS[@]}"; do
  if [ ! -d "$PDIR/$repo" ]; then
    if [ -n "$REMOTE" ]; then
      git clone "$REMOTE/$repo.git" "$PDIR/$repo" || { echo "✗ Failed to clone $repo"; continue; }
    else
      mkdir -p "$PDIR/$repo" && (cd "$PDIR/$repo" && git init)
    fi
    echo "✓ cloned $repo"
  fi
  if [ ! -f "$PDIR/$repo/AGENTS.md" ]; then
    REPO="$repo" CO="$CO" PROD="$PROD" envsubst < "$TPL/AGENTS.md.repo.tmpl" > "$PDIR/$repo/AGENTS.md"
  fi
done

# tmux launcher
LAUNCHER=~/dotfiles/shell/30-projects.zsh
[ ! -f "$LAUNCHER" ] && echo "# Project tmux launchers (auto-generated)" > "$LAUNCHER"

if ! grep -q "^${PROD}() {" "$LAUNCHER"; then
  FIRST="${REPOS[0]:-}"
  cat >> "$LAUNCHER" <<EOF

${PROD}() {
  tmux kill-session -t ${PROD} 2>/dev/null
  tmux new -d -s ${PROD} -c ${PDIR}/${FIRST}
  tmux send-keys -t ${PROD}:0 'export CODEX_HOME=\$HOME/.codex-new; clear; codex' Enter
  tmux split-window -v -t ${PROD}:0 -c ${PDIR}/${FIRST}
  tmux send-keys -t ${PROD}:0.1 'git status; echo' Enter
  tmux attach -t ${PROD}
}
EOF
  echo "✓ added '${PROD}' launcher to shell/30-projects.zsh"
fi

# Document in COMMANDS.md
DOC=~/dotfiles/docs/COMMANDS.md
if ! grep -q "^## ${PROD}$" "$DOC"; then
  cat >> "$DOC" <<EOF

## ${PROD}
**Что:** Open tmux session for ${CO}/${PROD}
**Запуск:** \`${PROD}\`
**Файлы:** \`${PDIR}/\`
**Repos:** ${REPOS[*]}
EOF
fi

echo ""
echo "✅ ${CO}/${PROD} bootstrapped"
echo ""
ls "$PDIR"
echo ""
echo "Next: source ~/.zshrc && ${PROD}"
