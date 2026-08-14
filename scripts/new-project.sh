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

WORK_ROOT="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}"
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

# Product AGENTS.md (envsubst with strict variable list)
CO="$CO" PROD="$PROD" envsubst '$CO $PROD' \
  < "$TPL/AGENTS.md.product.tmpl" > "$PDIR/AGENTS.md"
echo "✓ $PDIR/AGENTS.md"

cat > "$PDIR/.product-config" <<EOF
slug: $PROD
namespace: $NS
EOF

if [ ! -f "$PDIR/.envrc" ]; then
  cat > "$PDIR/.envrc" <<EOF
source_up

# Product-scoped secrets are loaded here by:
#   secret add --product <VAR> <VALUE>
EOF
  echo "✓ $PDIR/.envrc (run 'direnv allow' in product dir if needed)"
fi

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
    REPO="$repo" CO="$CO" PROD="$PROD" envsubst '$CO $PROD $REPO' \
      < "$TPL/AGENTS.md.repo.tmpl" > "$PDIR/$repo/AGENTS.md"
  fi
  if [ ! -f "$PDIR/$repo/.envrc" ]; then
    cat > "$PDIR/$repo/.envrc" <<EOF
source_up

# Repo-scoped secrets are loaded here by:
#   secret add --repo <VAR> <VALUE>
# Shared dev infra (Postgres/Redis/Qdrant/…): wire endpoints + create the DB with
#   dev-stack connect
EOF
    echo "✓ $PDIR/$repo/.envrc (run 'direnv allow' in repo dir if needed)"
  fi
done

# Shell shortcuts are table-driven (shell/30-projects.zsh:_PROJECT_SHORTCUTS);
# the company-level function is registered automatically from .company-config,
# so the product is reachable as `<company> <product>` without any generated code.
DEFAULT_REPO=""
[ ${#REPOS[@]} -eq 1 ] && DEFAULT_REPO="${REPOS[0]}"

if command -v hq >/dev/null 2>&1; then
  if hq editor sync --no-backup >/tmp/dotfiles-editor-sync.log 2>&1; then
    echo "✓ VS Code Project Manager synced"
  else
    echo "⚠ VS Code Project Manager sync skipped/failed"
    sed 's/^/  /' /tmp/dotfiles-editor-sync.log
  fi
fi

echo ""
echo "✅ ${CO}/${PROD} bootstrapped (ns: $NS)"
ls "$PDIR"
echo ""
echo "Next steps:"
echo "  1. onboard ${CO} ${PROD}                       (заполнить TODO в product AGENTS.md)"
echo "  2. analyze-product ${CO} ${PROD}               (опц: deep dive → docs/ARCHITECTURE.md)"
echo "  3. cd <repo> && onboard                     (для каждого репа)"
echo "  4. cd <repo> && dev-stack connect           (подключить к dev-stack: .envrc + БД)"
echo "  5. cd <repo> && analyze-repo                (опц: deep dive → docs/design.md)"
echo "  6. ${CO} ${PROD}                             (создать worktree и начать работу)"
echo ""
echo "Опционально — короткий алиас: добавь строку в _PROJECT_SHORTCUTS"
echo "(shell/30-projects.zsh):  \"${PROD}:${CO}:${PROD}:${DEFAULT_REPO}\""
