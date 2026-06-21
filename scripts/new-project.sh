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

LAUNCHER=~/dotfiles/shell/30-projects.zsh
[ ! -f "$LAUNCHER" ] && echo "# Project launch shortcuts" > "$LAUNCHER"

if ! grep -q "^${PROD}() {" "$LAUNCHER"; then
  DEFAULT_ARG=""
  [ ${#REPOS[@]} -eq 1 ] && DEFAULT_ARG=" \"${REPOS[0]}\""
  cat >> "$LAUNCHER" <<EOF

${PROD}() {
  case \$# in
    0) launch "${CO}" "${PROD}"${DEFAULT_ARG} ;;
    1) launch "${CO}" "${PROD}" "\$1" ;;
    *) echo "Usage: ${PROD} [<repo>]"; return 1 ;;
  esac
}
EOF
  echo "✓ added '${PROD}' launcher (repo-level 4-window layout)"
fi

DOC=~/dotfiles/docs/COMMANDS.md
if [ -f "$DOC" ] && ! grep -q "^## ${PROD}$" "$DOC"; then
  ENTRY_FILE=$(mktemp)
  cat > "$ENTRY_FILE" <<EOF

## ${PROD}
**Что:** Open tmux session for ${CO}/${PROD}
**Запуск:** \`${PROD}\`
**Файлы:** \`${PDIR}/\`
**Repos:** ${REPOS[*]}
EOF

  TMP=$(mktemp)
  awk -v entry_file="$ENTRY_FILE" '
    function load_entry(    line, content) {
      content = ""
      while ((getline line < entry_file) > 0) content = content line "\n"
      close(entry_file)
      return content
    }
    BEGIN { entry = load_entry(); in_ws = 0; inserted = 0 }
    /^### Workspaces$/ { in_ws = 1 }
    /^---$/ && in_ws && !inserted {
      printf "%s", entry
      inserted = 1
      in_ws = 0
    }
    { print }
    END { if (!inserted) printf "%s", entry }
  ' "$DOC" > "$TMP" && mv "$TMP" "$DOC"
  rm -f "$ENTRY_FILE"
  echo "✓ added '${PROD}' to COMMANDS.md (Workspaces)"
fi

VSCODE_SYNC="$HOME/dotfiles/scripts/vscode-projects-sync.py"
if [ -x "$VSCODE_SYNC" ]; then
  if "$VSCODE_SYNC" --no-backup >/tmp/dotfiles-vscode-projects-sync.log 2>&1; then
    echo "✓ VS Code Project Manager synced"
  else
    echo "⚠ VS Code Project Manager sync skipped/failed"
    sed 's/^/  /' /tmp/dotfiles-vscode-projects-sync.log
  fi
fi

echo ""
echo "✅ ${CO}/${PROD} bootstrapped (ns: $NS)"
ls "$PDIR"
echo ""
echo "Next steps:"
echo "  1. source ~/.zshrc                          (подхватить ${PROD} launcher)"
echo "  2. onboard ${CO} ${PROD}                       (заполнить TODO в product AGENTS.md)"
echo "  3. analyze-product ${CO} ${PROD}               (опц: deep dive → docs/ARCHITECTURE.md)"
echo "  4. cd <repo> && onboard                     (для каждого репа)"
echo "  5. cd <repo> && dev-stack connect           (подключить к dev-stack: .envrc + БД)"
echo "  6. cd <repo> && analyze-repo                (опц: deep dive → docs/design.md)"
echo "  7. ${PROD}                                   (открыть tmux session)"
