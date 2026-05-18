#!/bin/bash
# Bootstrap company workspace
# Usage: new-company <slug> <vcs> <namespace>
set -e

if [ "$#" -lt 3 ]; then
  cat <<'USAGE'
Usage: new-company <slug> <vcs> <namespace>

Examples:
  new-company acme gitlab acme-engineering
  new-company beta github beta-org
  new-company personal local local

Args:
  slug:      short identifier (e.g. "acme")
  vcs:       gitlab | github | bitbucket | local
  namespace: org/group name on VCS
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

# .company-config (machine-readable metadata used by new-project)
cat > "$DIR/.company-config" <<EOF
slug: $CO
vcs: $VCS
namespace: $NS
EOF
echo "✓ $DIR/.company-config"

# .envrc template (закомментирован, ты раскомментишь когда нужно)
cat > "$DIR/.envrc" <<ENVEOF
# direnv config for company '$CO'
# Раскоментируй и заполни. Secrets через 1Password (op CLI):
# export OPENAI_API_KEY="\$(op read 'op://Work-$CO/OpenAI/credential')"
# export DATABASE_URL="\$(op read 'op://Work-$CO/Postgres/connection_string')"

# Простые env vars без secrets:
# export AWS_PROFILE=$CO-dev
ENVEOF
echo "✓ $DIR/.envrc (template — закомментирован)"

# README
cat > "$DIR/README.md" <<EOF
# $CO

**VCS:** $VCS
**Namespace:** $NS

## Products

(добавляются автоматически через \`new-project $CO <product> ...\`)

## Notes

- Подружить с 1Password: открой .envrc, раскомментируй нужные строки
- После заполнения .envrc: \`direnv allow\` один раз в этой папке
EOF
echo "✓ $DIR/README.md"

echo ""
echo "✅ Company '$CO' bootstrapped at $DIR"
echo ""
echo "Next: new-project $CO <product> [repo1] [repo2] ..."
