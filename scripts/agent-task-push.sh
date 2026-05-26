#!/usr/bin/env bash
# Push the current agent branch after the logical task is complete.
# Usage: agent-task-push.sh [--allow-dirty]

set -euo pipefail

allow_dirty=0
if [ "${1:-}" = "--allow-dirty" ]; then
  allow_dirty=1
  shift
fi
if [ $# -ne 0 ]; then
  echo "Usage: agent-task-push.sh [--allow-dirty]" >&2
  exit 64
fi

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "error: not inside a git repository" >&2
  exit 64
}
cd "$repo_root"

ensure_worktree_excludes() {
  local exclude_file
  exclude_file=$(git rev-parse --git-path info/exclude)
  mkdir -p "$(dirname "$exclude_file")"
  touch "$exclude_file"
  grep -qxF '.agent-workspace' "$exclude_file" || echo '.agent-workspace' >> "$exclude_file"
}

[ -f .agent-workspace ] && ensure_worktree_excludes

branch=$(git branch --show-current)
if [ -z "$branch" ]; then
  echo "error: detached HEAD; refusing to push" >&2
  exit 65
fi

if ! git remote get-url origin >/dev/null 2>&1; then
  echo "error: remote origin is missing" >&2
  exit 65
fi

if [ "$allow_dirty" != "1" ] && [ -n "$(git status --short)" ]; then
  echo "error: working tree is dirty; commit completed changes before task push" >&2
  git status --short >&2
  exit 65
fi

case "$branch" in
  agent/*|feat/*|fix/*|docs/*|chore/*) ;;
  main|dev)
    if [ "${AGENT_ALLOW_INTEGRATION_PUSH:-0}" != "1" ]; then
      echo "error: refusing to push integration branch '$branch' from agent task" >&2
      echo "set AGENT_ALLOW_INTEGRATION_PUSH=1 only for WikiPedik/dotfiles/manual sync flows" >&2
      exit 65
    fi
    ;;
  *)
    echo "warning: pushing non-standard task branch '$branch'" >&2
    ;;
esac

printf '\n--- branch status ---\n'
git status --short --branch
printf '\n--- pushing task branch ---\n'
git push -u origin HEAD

if [ -f .agent-workspace ]; then
  tmp=$(mktemp)
  awk -F= '$1 != "cleanup_state" && $1 != "pushed_at" && $1 != "pushed_branch"' .agent-workspace > "$tmp"
  {
    cat "$tmp"
    echo "cleanup_state=pushed"
    echo "pushed_at=$(date '+%Y-%m-%d %H:%M:%S')"
    echo "pushed_branch=$branch"
  } > .agent-workspace
  rm -f "$tmp"
fi

printf '\n✓ pushed task branch: %s\n' "$branch"
printf 'Worktree marked pushed. Run agent-finish.sh to open review, or mark ready manually after review.\n'
