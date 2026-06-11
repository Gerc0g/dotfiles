#!/usr/bin/env bash
# Catch-all autocommit for the WikiPedik vault.
#
# Scope-atomic curator commits (wiki-commit / wiki sync --commit) stay the
# meaningful history; this sweeps everything else (Obsidian config, human
# edits, stray files) so curation never sits uncommitted for weeks.
#
# Never pushes. Aborts without committing when the diff looks like it
# contains a secret.
#
# Usage:
#   wikipedik-autocommit.sh            # commit now if dirty
#   wikipedik-autocommit.sh --if-due   # only if last run was >24h ago (for launch tail)

set -euo pipefail

VAULT="${WIKIPEDIK_ROOT:-$HOME/Desktop/WikiPedik}"
STAMP="${WIKIPEDIK_AUTOCOMMIT_STAMP:-$HOME/.cache/wikipedik-autocommit.stamp}"
INTERVAL_S=$((24 * 60 * 60))

if [ "${1:-}" = "--if-due" ]; then
  if [ -f "$STAMP" ]; then
    last=$(stat -f %m "$STAMP" 2>/dev/null || echo 0)
    now=$(date +%s)
    [ $((now - last)) -lt "$INTERVAL_S" ] && exit 0
  fi
fi

[ -d "$VAULT/.git" ] || exit 0

mkdir -p "$(dirname "$STAMP")"
touch "$STAMP"

if [ -z "$(git -C "$VAULT" status --porcelain 2>/dev/null)" ]; then
  exit 0
fi

# Stage first so the scan also covers brand-new files (plain `diff HEAD`
# misses untracked content).
git -C "$VAULT" add -A

# High-confidence secret patterns only; vault must never enshrine credentials.
SECRET_RE='(BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_-]{20,}|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|xox[bap]-[A-Za-z0-9-]{10,})'
if git -C "$VAULT" diff --cached 2>/dev/null | grep -EIq "$SECRET_RE"; then
  git -C "$VAULT" reset --quiet HEAD
  echo "⚠ wikipedik-autocommit: похоже на секрет в diff — автокоммит отменён, проверь вручную (git -C ~/Desktop/WikiPedik diff)" >&2
  exit 1
fi

git -C "$VAULT" commit --quiet -m "chore(vault): автокоммит несинхронизированных изменений"
echo "wikipedik-autocommit: committed $(git -C "$VAULT" show --stat --format= HEAD | tail -1 | sed 's/^ *//')"
