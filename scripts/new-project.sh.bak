#!/bin/bash
# Bootstrap project at ~/Desktop/Prokectfiles/<company>/<product>/
# Usage: new-project <company> <product> [--ns=<override>] [repo1] [repo2] ...
set -e

if [ "$#" -lt 2 ]; then
  cat <<'USAGE'
Usage: new-project <company> <product> [--ns=<override>] [repo1] [repo2] ...

Examples:
  new-project justchimera aetheria Aetheria-App aetheria-frontend
  new-project neurodesk agents --ns=dev/nrdsk_ai/agents barrier cerebellum
USAGE
  exit 1
fi

CO="$1"; PROD="$2"
shift 2

NS_OVERRIDE=""
if [[ "$1" == --ns=* ]]; then
  NS_OVERRIDE="${1#--ns=}"
  shift
fi

REPOS=("$@")

WORK_ROOT="$HOME/Desktop/Prokectfiles"
DIR="$WORK_ROOT/$CO"
PDIR="$DIR/$PROD"
TPL=~/dotfiles/templates
CFG="$DIR/.company-config"

if [ ! -f "$CFG" ]; then
  echo "⚠ Company '$CO' not found. Run: new-company $CO <vcs> <namespace>"
  exit 1
fi

VCS=$(awk '/^vcs:/ {print $2}' "$CFG")
HOST=$(awk '/^host:/ {print $2}' "$CFG")
NS_DEFAULT=$(awk '/^namespace:/ {print $2}' "$CFG")
SSH_HOST=$(awk '/^ssh_host:/ {print $2}' "$CFG")

NS="${NS_OVERRIDE:-$NS_DEFAULT}"

mkdir -p "$PDIR"

CO="$CO" PROD="$PROD" REPOS="${REPOS[*]}" envsubst < "$TPL/AGENTS.md.product.tmpl" > "$PDIR/AGENTS.md"
echo "✓ $PDIR/AGENTS.md"

cat > "$PDIR/.product-config" <<EOF
slug: $PROD
namespace: $NS
EOF

if [ -n "$SSH_HOST" ] && [ "$SSH_HOST" != "local" ]; then
  REMOTE="git@${SSH_HOST}:${NS}"
elif [ "$VCS" = "local" ]; then
  REMOTE=""
else
  REMOTE="git@${HOST}:${NS}"
fi

for repo in "${REPOS[@]}"; do
  if [ ! -d "$PDIR/$repo" ]; then
    if [ -n "$REMOTE" ]; then
      git clone "$REMOTE/$repo.git" "$PDIR/$repo" || { echo "✗ Failed: $repo (URL: $REMOTE/$repo.git)"; continue; }
    else
      mkdir -p "$PDIR/$repo" && (cd "$PDIR/$repo" && git init)
    fi
    echo "✓ cloned $repo"
  fi
  if [ ! -f "$PDIR/$repo/AGENTS.md" ]; then
    REPO="$repo" CO="$CO" PROD="$PROD" envsubst < "$TPL/AGENTS.md.repo.tmpl" > "$PDIR/$repo/AGENTS.md"
  fi
done

LAUNCHER=~/dotfiles/shell/30-projects.zsh
[ ! -f "$LAUNCHER" ] && echo "# Project tmux launchers" > "$LAUNCHER"

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
  echo "✓ added '${PROD}' launcher"
fi

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
echo "✅ ${CO}/${PROD} bootstrapped (ns: $NS)"
ls "$PDIR"
echo ""
echo "Next: source ~/.zshrc && ${PROD}"
