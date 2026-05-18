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

mkdir -p "$WORK_ROOT"

if [ -d "$DIR" ]; then
  echo "⚠ Company '$CO' already exists at $DIR"
  exit 1
fi

mkdir -p "$DIR"

# AGENTS.md
CO="$CO" VCS="$VCS" NS="$NS" envsubst < "$TPL/AGENTS.md.company.tmpl" > "$DIR/AGENTS.md"
echo "✓ $DIR/AGENTS.md"

# SSH key + config alias (если не local)
SSH_HOST=""
if [ "$VCS" != "local" ]; then
  KEY_PATH=~/.ssh/${CO}_id_ed25519
  SSH_HOST="${VCS}-${CO}"
  
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
    echo "✓ SSH alias '$SSH_HOST' → $HOST"
  fi
fi

# .company-config
cat > "$DIR/.company-config" <<EOF
slug: $CO
vcs: $VCS
host: $HOST
namespace: $NS
ssh_host: ${SSH_HOST:-local}
git_email: ${EMAIL:-}
EOF
echo "✓ $DIR/.company-config"

# .envrc
cat > "$DIR/.envrc" <<EOF
# direnv config for '$CO'

${EMAIL:+export GIT_AUTHOR_EMAIL=$EMAIL}
${EMAIL:+export GIT_COMMITTER_EMAIL=$EMAIL}

# Секреты: добавляй через 'secret add VARNAME value'
EOF
echo "✓ $DIR/.envrc"

# README
cat > "$DIR/README.md" <<EOF
# $CO

**VCS:** $VCS @ $HOST
**Namespace:** $NS
**SSH alias:** ${SSH_HOST:-(none)}
**Email:** ${EMAIL:-(not set)}
**1Password vault:** Work-$CO

## Products
(добавляются через \`new-project $CO <product> ...\`)
EOF
echo "✓ $DIR/README.md"

# 1Password vault
if command -v op >/dev/null 2>&1 && op account list >/dev/null 2>&1; then
  op vault create "Work-$CO" >/dev/null 2>&1 && echo "✓ 1Password vault 'Work-$CO' created" \
    || echo "→ 1Password vault 'Work-$CO' already exists"
fi

echo ""
echo "✅ Company '$CO' bootstrapped at $DIR"
echo ""
echo "━━━ Manual steps ━━━"
echo ""

if [ "$VCS" != "local" ]; then
  echo "1. Добавь публичный ключ в $VCS account на $HOST:"
  echo ""
  cat "$KEY_PATH.pub"
  echo ""
  case "$VCS" in
    gitlab)    echo "   → https://$HOST/-/user_settings/ssh_keys" ;;
    github)    echo "   → https://$HOST/settings/ssh/new" ;;
    bitbucket) echo "   → https://$HOST/account/settings/ssh-keys/" ;;
  esac
  echo ""
  echo "2. Тест SSH:  ssh -T git@$SSH_HOST"
  echo ""
  
  case "$VCS" in
    gitlab) echo "3. Login glab CLI:  glab auth login --hostname $HOST" ;;
    github) echo "3. Login gh CLI:    gh auth login --hostname $HOST --git-protocol ssh" ;;
  esac
  echo ""
fi

echo "4. Секреты + direnv:"
echo "   cd $DIR"
echo "   secret add OPENAI_API_KEY <value>  # опц."
echo "   direnv allow"
echo ""
echo "5. Создавай продукты:"
echo "   new-project $CO <product> [repo1] [repo2] ..."
