#!/usr/bin/env bash
# Catch-all autocommit for the WikiPedik vault.
#
# Scope-atomic curator commits (wiki-commit / wiki sync --commit) stay the
# meaningful history; this sweeps everything else (Obsidian config, human
# edits, stray files) so curation never sits uncommitted for weeks.
#
# Cross-machine sync: after committing, pulls remote (rebase) and pushes, so the
# vault stays consistent across Mac + server. Sync is best-effort and guarded —
# network/conflict issues only warn, never break the caller (launch runs it `|| true`).
# Aborts without committing when the diff looks like it contains a secret.
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
    last=$(stat -f %m "$STAMP" 2>/dev/null || stat -c %Y "$STAMP" 2>/dev/null || echo 0)
    now=$(date +%s)
    [ $((now - last)) -lt "$INTERVAL_S" ] && exit 0
  fi
fi

[ -d "$VAULT/.git" ] || exit 0

mkdir -p "$(dirname "$STAMP")"
touch "$STAMP"

# Commit local changes first (if any), so the tree is clean before rebasing.
if [ -n "$(git -C "$VAULT" status --porcelain 2>/dev/null)" ]; then
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
fi

# Cross-machine sync (best-effort, guarded). Pull remote (rebase our commits on
# top), then push — keeps the vault consistent across Mac + server. Any network
# or conflict issue just warns; it never breaks the caller. Skips if no remote.
if git -C "$VAULT" remote 2>/dev/null | grep -q .; then
  if git -C "$VAULT" pull --rebase --quiet 2>/dev/null; then
    if git -C "$VAULT" push --quiet 2>/dev/null; then
      echo "wikipedik-autocommit: synced (pull+push)"
    else
      echo "⚠ wikipedik-autocommit: push не прошёл (сеть/доступ?) — изменения сохранены локально" >&2
    fi
  else
    git -C "$VAULT" rebase --abort 2>/dev/null || true
    echo "⚠ wikipedik-autocommit: pull --rebase не прошёл (конфликт/сеть?) — синк отложен, разреши вручную" >&2
  fi
fi
exit 0
