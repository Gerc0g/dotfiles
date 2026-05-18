#!/bin/bash
# Bootstrap company: folder + AGENTS.md + 1Password vault + SSH key + SSH config + git identity prompts
# Usage: new-company <slug> <vcs> <namespace> [git-email]
set -e

if [ "$#" -lt 3 ]; then
  cat <<'USAGE'
Usage: new-company <slug> <vcs> <namespace> [git-email]

Examples:
  new-company acme gitlab acme-engineering egor@acme.com
  new-company beta github beta-org egor@beta.io
  new-company personal local local

Args:
  slug:       short identifier (used in folder name, vault name, SSH alias)
  vcs:        gitlab | github | bitbucket | local
  namespace:  org/group name on VCS
  git-email:  (optional) email for git commits in this company
USAGE
  exit 1
fi

CO="$1"; VCS="$2"; NS="$3"; EMAIL="${4:-}"
DIR=~/work/$CO
TPL=~/dotfiles/templates

if [ -d "$DIR" ]; then
  echo "⚠ Company '$CO' already exists at $DIR"
  exit 1
fi

mkdir -p "$DIR"

# === 1. AGENTS.md ===
CO="$CO" VCS="$VCS" NS="$NS" envsubst < "$TPL/AGENTS.md.company.tmpl" > "$DIR/AGENTS.md"
echo "✓ $DIR/AGENTS.md"

# === 2. SSH ключ + SSH config (если не local) ===
SSH_HOST=""
if [ "$VCS" != "local" ]; then
  KEY_PATH=~/.ssh/${CO}_id_ed25519
  
  case "$VCS" in
    gitlab)    SSH_HOST="gitlab-$CO"; REAL_HOST="gitlab.com" ;;
    github)    SSH_HOST="github-$CO"; REAL_HOST="github.com" ;;
    bitbucket) SSH_HOST="bitbucket-$CO"; REAL_HOST="bitbucket.org" ;;
  esac
  
  if [ ! -f "$KEY_PATH" ]; then
    ssh-keygen -t ed25519 -f "$KEY_PATH" -N "" -C "${EMAIL:-$CO-machine}" >/dev/null
    echo "✓ SSH key generated: $KEY_PATH"
  else
    echo "→ SSH key already exists at $KEY_PATH"
  fi
  
  # Дописать SSH config
  SSH_CONFIG=~/.ssh/config
  touch "$SSH_CONFIG" && chmod 600 "$SSH_CONFIG"
  if ! grep -q "Host $SSH_HOST$" "$SSH_CONFIG"; then
    cat >> "$SSH_CONFIG" <<EOF

# === $CO ($VCS) ===
Host $SSH_HOST
  HostName $REAL_HOST
  User git
  IdentityFile $KEY_PATH
  IdentitiesOnly yes
EOF
    echo "✓ SSH config alias '$SSH_HOST' added"
  fi
fi

# === 3. .company-config (metadata для new-project) ===
cat > "$DIR/.company-config" <<EOF
slug: $CO
vcs: $VCS
namespace: $NS
ssh_host: ${SSH_HOST:-local}
git_email: ${EMAIL:-}
EOF
echo "✓ $DIR/.company-config"

# === 4. .envrc (1Password + git identity) ===
cat > "$DIR/.envrc" <<EOF
# direnv config for company '$CO'

# === Git identity (применяется ко всем git операциям в этой папке) ===
${EMAIL:+export GIT_AUTHOR_EMAIL=$EMAIL}
${EMAIL:+export GIT_COMMITTER_EMAIL=$EMAIL}

# === Секреты через 1Password ===
# Добавь через: secret add VARNAME value
# Они автоматически окажутся здесь как:
# export VARNAME="\$(op read 'op://Work-$CO/VARNAME/credential')"

# === Простые env vars (без secrets) ===
# export AWS_PROFILE=$CO-dev
# export COMPANY=$CO
EOF
echo "✓ $DIR/.envrc"

# === 5. README ===
cat > "$DIR/README.md" <<EOF
# $CO

**VCS:** $VCS
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

# === 6. Auto-create 1Password vault ===
if command -v op >/dev/null 2>&1 && op account list >/dev/null 2>&1; then
  op vault create "Work-$CO" >/dev/null 2>&1 && echo "✓ 1Password vault 'Work-$CO' created" \
    || echo "→ 1Password vault 'Work-$CO' already exists"
fi

# === 7. Финальные инструкции ===
echo ""
echo "✅ Company '$CO' bootstrapped at $DIR"
echo ""
echo "━━━ Manual steps (one-time per company) ━━━"
echo ""

if [ "$VCS" != "local" ]; then
  echo "1. Добавь публичный ключ в $VCS account:"
  echo ""
  cat "$KEY_PATH.pub"
  echo ""
  case "$VCS" in
    gitlab) echo "   → https://gitlab.com/-/user_settings/ssh_keys" ;;
    github) echo "   → https://github.com/settings/ssh/new" ;;
    bitbucket) echo "   → https://bitbucket.org/account/settings/ssh-keys/" ;;
  esac
  echo ""
  echo "2. Тест SSH:"
  echo "   ssh -T git@$SSH_HOST"
  echo ""
  
  case "$VCS" in
    gitlab) echo "3. Login glab CLI (для MR через CLI):"; echo "   glab auth login --hostname gitlab.com" ;;
    github) echo "3. Login gh CLI (для PR через CLI):"; echo "   gh auth login --hostname github.com --git-protocol ssh" ;;
  esac
  echo ""
fi

echo "4. Добавь секреты:"
echo "   cd $DIR"
echo "   secret add OPENAI_API_KEY <value>"
echo "   secret add ${VCS^^}_TOKEN <value>"
echo ""
echo "5. Активируй direnv:"
echo "   cd $DIR && direnv allow"
echo ""
echo "6. Создавай продукты:"
echo "   new-project $CO <product> [repo1] [repo2] ..."
