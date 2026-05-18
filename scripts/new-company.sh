#!/bin/bash
# Bootstrap company: folder + AGENTS.md + 1Password vault + SSH + git identity
# Usage: new-company <slug> <vcs[:host]> <namespace> [git-email]
set -e

if [ "$#" -lt 3 ]; then
  cat <<'USAGE'
Usage: new-company <slug> <vcs[:host]> <namespace> [git-email]

Examples:
  new-company acme gitlab acme-engineering egor@acme.com
  new-company acme gitlab:gitlab.acme.io acme-eng egor@acme.io
  new-company beta github beta-org
  new-company beta github:ghe.beta.io beta-org
  new-company personal local local

Args:
  slug:       short id (folder, vault, SSH alias)
  vcs[:host]: gitlab | github | bitbucket | local
              with optional :host for self-hosted (gitlab:gitlab.acme.io)
  namespace:  org/group name
  git-email:  (optional) email for commits in this company
USAGE
  exit 1
fi

CO="$1"; VCS_FULL="$2"; NS="$3"; EMAIL="${4:-}"
DIR=~/work/$CO
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

if [ -d "$DIR" ]; then
  echo "⚠ Company '$CO' already exists at $DIR"
  exit 1
fi

mkdir -p "$DIR"

# === AGENTS.md ===
CO="$CO" VCS="$VCS" NS="$NS" envsubst < "$TPL/AGENTS.md.company.tmpl" > "$DIR/AGENTS.md"
echo "✓ $DIR/AGENTS.md"

# === SSH ключ + config (если не local) ===
SSH_HOST=""
if [ "$VCS" != "local" ]; then
  KEY_PATH=~/.ssh/${CO}_id_ed25519
  SSH_HOST="${VCS}-${CO}"
  
  if [ ! -f "$KEY_PATH" ]; then
    ssh-keygen -t ed25519 -f "$KEY_PATH" -N "" -C "${EMAIL:-$CO-machine}" >/dev/null
    echo "✓ SSH key generated: $KEY_PATH"
  else
    echo "→ SSH key already exists at $KEY_PATH"
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
    echo "✓ SSH config alias '$SSH_HOST' → $HOST"
  fi
fi

# === .company-config (теперь с host) ===
cat > "$DIR/.company-config" <<EOF
slug: $CO
vcs: $VCS
host: $HOST
namespace: $NS
ssh_host: ${SSH_HOST:-local}
git_email: ${EMAIL:-}
EOF
echo "✓ $DIR/.company-config"

# === .envrc ===
cat > "$DIR/.envrc" <<EOF
# direnv config for company '$CO'

# Git identity
${EMAIL:+export GIT_AUTHOR_EMAIL=$EMAIL}
${EMAIL:+export GIT_COMMITTER_EMAIL=$EMAIL}

# Секреты: добавляй через 'secret add VARNAME value'
EOF
echo "✓ $DIR/.envrc"

# === README ===
cat > "$DIR/README.md" <<EOF
# $CO

**VCS:** $VCS @ $HOST
**Namespace:** $NS
**SSH alias:** ${SSH_HOST:-(none, local)}
**Git email:** ${EMAIL:-(not set)}
**1Password vault:** Work-$CO

## Products
(добавляются автоматически через \`new-project $CO <product> ...\`)

## Setup secrets
\`\`\`bash
cd ~/work/$CO
secret add OPENAI_API_KEY <value>
secret add GITLAB_TOKEN <value>
direnv allow
\`\`\`
EOF
echo "✓ $DIR/README.md"

# === Auto-create 1Password vault ===
if command -v op >/dev/null 2>&1 && op account list >/dev/null 2>&1; then
  op vault create "Work-$CO" >/dev/null 2>&1 && echo "✓ 1Password vault 'Work-$CO' created" \
    || echo "→ 1Password vault 'Work-$CO' already exists"
fi

# === Manual steps ===
echo ""
echo "✅ Company '$CO' bootstrapped"
echo ""
echo "━━━ Manual steps ━━━"
echo ""

if [ "$VCS" != "local" ]; then
  echo "1. Добавь публичный ключ в $VCS аккаунт на $HOST:"
  echo ""
  cat "$KEY_PATH.pub"
  echo ""
  case "$VCS" in
    gitlab)    echo "   → https://$HOST/-/user_settings/ssh_keys" ;;
    github)    echo "   → https://$HOST/settings/ssh/new" ;;
    bitbucket) echo "   → https://$HOST/account/settings/ssh-keys/" ;;
  esac
  echo ""
  echo "2. Тест SSH:"
  echo "   ssh -T git@$SSH_HOST"
  echo ""
  
  case "$VCS" in
    gitlab) echo "3. Login glab CLI:"; echo "   glab auth login --hostname $HOST" ;;
    github) echo "3. Login gh CLI:"; echo "   gh auth login --hostname $HOST --git-protocol ssh" ;;
  esac
  echo ""
fi

echo "4. Добавь секреты + активируй direnv:"
echo "   cd $DIR"
echo "   secret add OPENAI_API_KEY <value>"
echo "   secret add ${VCS^^}_TOKEN <value>"
echo "   direnv allow"
echo ""
echo "5. Создавай продукты:"
echo "   new-project $CO <product> [repo1] [repo2] ..."
