#!/bin/bash
# Bootstrap company workspace + auto-create 1Password vault
# Usage: new-company <slug> <vcs> <namespace>
set -e

if [ "$#" -lt 3 ]; then
  cat <<'USAGE'
Usage: new-company <slug> <vcs> <namespace>

Examples:
  new-company acme gitlab acme-engineering
  new-company beta github beta-org
  new-company personal local local
USAGE
  exit 1
fi

CO="$1"; VCS="$2"; NS="$3"
DIR=~/work/$CO
TPL=~/dotfiles/templates

if [ -d "$DIR" ]; then
  echo "⚠ Company '$CO' already exists at $DIR"
  exit 1
fi

mkdir -p "$DIR"

# AGENTS.md
CO="$CO" VCS="$VCS" NS="$NS" envsubst < "$TPL/AGENTS.md.company.tmpl" > "$DIR/AGENTS.md"
echo "✓ $DIR/AGENTS.md"

# .company-config (metadata)
cat > "$DIR/.company-config" <<EOF
slug: $CO
vcs: $VCS
namespace: $NS
EOF
echo "✓ $DIR/.company-config"

# .envrc from template
CO="$CO" envsubst < "$TPL/envrc.tmpl" > "$DIR/.envrc"
echo "✓ $DIR/.envrc (раскомментируй нужные строки или используй: secret add VAR VALUE)"

# README
cat > "$DIR/README.md" <<EOF
# $CO

**VCS:** $VCS
**Namespace:** $NS
**1Password vault:** Work-$CO

## Products
(добавляются автоматически через \`new-project $CO <product> ...\`)

## Setup secrets
\`\`\`bash
cd ~/work/$CO
secret add OPENAI_API_KEY sk-...          # auto: создаёт item в vault + добавляет в .envrc
secret add GITLAB_TOKEN glpat-...
direnv allow                              # один раз
\`\`\`
EOF
echo "✓ $DIR/README.md"

# === АВТОСОЗДАНИЕ 1PASSWORD VAULT ===
if command -v op >/dev/null 2>&1; then
  if op account list >/dev/null 2>&1; then
    if op vault create "Work-$CO" >/dev/null 2>&1; then
      echo "✓ 1Password vault 'Work-$CO' создан"
    else
      echo "→ 1Password vault 'Work-$CO' уже существует"
    fi
  else
    echo "⚠ op CLI не signed in. Чтобы создать vault: eval \$(op signin) && op vault create 'Work-$CO'"
  fi
else
  echo "⚠ op CLI не установлен. Vault не создан. Установи: brew install --cask 1password-cli"
fi

echo ""
echo "✅ Company '$CO' bootstrapped at $DIR"
echo ""
echo "Next steps:"
echo "  1. Добавь секреты:    secret add OPENAI_API_KEY <value>"
echo "  2. Активируй direnv:  cd $DIR && direnv allow"
echo "  3. Создай продукты:   new-project $CO <product> [repos...]"
