#!/bin/bash
# Bootstrap company workspace at ~/Desktop/Prokectfiles/<slug>/
# Usage: new-company <slug> <vcs[:host]> <namespace> [git-email]
set -e

if [ "$#" -lt 3 ]; then
  cat <<'USAGE'
Usage: new-company <slug> <vcs[:host]> <namespace> [git-email]

Examples:
  new-company neurodesk gitlab:nrdsk.gitlab.yandexcloud.net dev your@email.com
  new-company justchimera github JustChimera gerc0g@justchimera.com
USAGE
  exit 1
fi

CO="$1"; VCS_FULL="$2"; NS="$3"; EMAIL="${4:-}"
WORK_ROOT="$HOME/Desktop/Prokectfiles"
DIR="$WORK_ROOT/$CO"
TPL=~/dotfiles/templates

# Parse vcs:host syntax
VCS="${VCS_FULL%%:*}"
HOST="${VCS_FULL##*:}"
if [ "$HOST" = "$VCS" ]; then
  case "$VCS" in
    gitlab)    HOST="gitlab.com" ;;
    github)    HOST="github.com" ;;
    bitbucket) HOST="bitbucket.org" ;;
    local)     HOST="local" ;;
    *)         echo "Unknown VCS: $VCS"; exit 1 ;;
  esac
fi

# Derive placeholders for template
CO_TITLE="$(echo "$CO" | awk '{print toupper(substr($0,1,1)) substr($0,2)}')"
SSH_HOST=""
[ "$VCS" != "local" ] && SSH_HOST="${VCS}-${CO}"
VAULT="Work-${CO}"

mkdir -p "$WORK_ROOT"

if [ -d "$DIR" ]; then
  echo "⚠ Company '$CO' already exists at $DIR"
  exit 1
fi

mkdir -p "$DIR"

# AGENTS.md (envsubst with strict variable list)
CO="$CO" CO_TITLE="$CO_TITLE" VCS="$VCS" HOST="$HOST" NS="$NS" \
  SSH_HOST="$SSH_HOST" VAULT="$VAULT" EMAIL="$EMAIL" \
  envsubst '$CO $CO_TITLE $VCS $HOST $NS $SSH_HOST $VAULT $EMAIL' \
  < "$TPL/AGENTS.md.company.tmpl" > "$DIR/AGENTS.md"
echo "✓ $DIR/AGENTS.md"

# SSH key + config alias (если не local)
if [ "$VCS" != "local" ]; then
  KEY_PATH=~/.ssh/${CO}_id_ed25519

  if [ ! -f "$KEY_PATH" ]; then
    ssh-keygen -t ed25519 -f "$KEY_PATH" -N "" -C "${EMAIL:-$CO-machine}" >/dev/null
    echo "✓ SSH key generated: $KEY_PATH"
  else
    echo "→ SSH key exists at $KEY_PATH"
  fi

  SSH_CONFIG=~/.ssh/config
  touch "$SSH_CONFIG" && chmod 600 "$SSH_CONFIG"
  if ! grep -q "Host $SSH_HOST$" "$SSH_CONFIG"; then
    cat >> "$SSH_CONFIG" <<EOF

# === $CO ($VCS @ $HOST) ===
Host $SSH_HOST
  HostName $HOST
  User git
  IdentityFile $KEY_PATH
  IdentitiesOnly yes
EOF
    echo "✓ SSH alias '$SSH_HOST' added to $SSH_CONFIG"
  else
    echo "→ SSH alias '$SSH_HOST' already in config"
  fi
fi

# .company-config
cat > "$DIR/.company-config" <<EOF
slug: $CO
vcs: $VCS
host: $HOST
namespace: $NS
ssh_host: $SSH_HOST
git_email: $EMAIL
EOF
echo "✓ $DIR/.company-config"

# .envrc (direnv) — git identity + закомментированные секреты
cat > "$DIR/.envrc" <<EOF
# Git identity for $CO
export GIT_AUTHOR_EMAIL="${EMAIL}"
export GIT_COMMITTER_EMAIL="${EMAIL}"

# Add secrets via: secret add VAR value
# example after secret add:
# export EXAMPLE_TOKEN=\$(op read "op://Work-${CO}/EXAMPLE_TOKEN/credential")
EOF
echo "✓ $DIR/.envrc (run 'direnv allow' to activate)"

# README.md
cat > "$DIR/README.md" <<EOF
# $CO_TITLE

VCS: $VCS @ $HOST
Namespace: $NS
SSH alias: $SSH_HOST
1Password vault: Work-$CO

## Products

(добавляются через \`new-project $CO <product> [repos...]\`)
EOF
echo "✓ $DIR/README.md"

# 1Password vault (optional — if op signed in)
if command -v op >/dev/null 2>&1 && op vault list >/dev/null 2>&1; then
  if ! op vault list --format=json 2>/dev/null | grep -q "\"name\":\"$VAULT\""; then
    op vault create "$VAULT" >/dev/null
    echo "✓ 1Password vault '$VAULT' created"
  else
    echo "→ 1Password vault '$VAULT' already exists"
  fi
else
  echo "⚠ 1Password CLI not signed in — run 'secret signin' then create vault manually: op vault create $VAULT"
fi

echo ""
echo "✅ Company '$CO' bootstrapped at $DIR"
echo ""
echo "Next steps:"
if [ "$VCS" != "local" ]; then
  echo "  1. Add public key to $VCS web UI:"
  echo "     pbcopy < ${KEY_PATH}.pub"
  echo "     Open ${HOST} settings → SSH Keys → paste"
  echo "  2. Test: ssh -T $SSH_HOST"
fi
echo "  3. cd $DIR && direnv allow"
echo "  4. Fill TODO placeholders in $DIR/AGENTS.md"
echo "  5. Create products: new-project $CO <product> [repos...]"
