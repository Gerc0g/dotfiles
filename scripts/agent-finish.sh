#!/usr/bin/env bash
# Finish an isolated agent task: verify, push branch, open PR/MR, record review metadata.
# Usage: agent-finish.sh [--base <branch>] [--title <title>] [--ready-without-review]

set -euo pipefail

base_override=""
title_override=""
ready_without_review=0

usage() {
  cat >&2 <<'USAGE'
Usage: agent-finish.sh [--base <branch>] [--title <title>] [--ready-without-review]

Runs verify/test, pushes the current agent branch, opens a PR/MR into dev/main,
and updates .agent-workspace metadata. Does not merge.
USAGE
}

while [ $# -gt 0 ]; do
  case "$1" in
    --base) base_override=${2:-}; shift 2 ;;
    --title) title_override=${2:-}; shift 2 ;;
    --ready-without-review) ready_without_review=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) usage; exit 64 ;;
  esac
done

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

meta="$repo_root/.agent-workspace"
if [ ! -f "$meta" ]; then
  echo "error: agent-finish.sh must run inside a managed agent worktree" >&2
  echo "start tasks with project shortcuts, for example: agents synapse ticket-quality" >&2
  exit 65
fi

metadata_value() {
  awk -F= -v key="$2" '$1 == key { print substr($0, length(key) + 2); exit }' "$1" 2>/dev/null || true
}

metadata_set() {
  local key=$1 value=$2 tmp
  tmp=$(mktemp)
  awk -F= -v key="$key" '$1 != key' "$meta" > "$tmp"
  {
    cat "$tmp"
    printf '%s=%s\n' "$key" "$value"
  } > "$meta"
  rm -f "$tmp"
}

branch=$(git branch --show-current)
if [ -z "$branch" ]; then
  echo "error: detached HEAD; refusing to finish" >&2
  exit 65
fi

case "$branch" in
  agent/*|feat/*|fix/*|docs/*|chore/*) ;;
  main|dev)
    echo "error: refusing to finish from integration branch '$branch'" >&2
    exit 65
    ;;
  *) echo "warning: finishing non-standard branch '$branch'" >&2 ;;
esac

if [ -n "$(git status --short)" ]; then
  echo "error: working tree is dirty; commit completed changes before finish" >&2
  git status --short >&2
  echo >&2
  echo "Use:" >&2
  echo "  ~/dotfiles/scripts/agent-commit.sh \"type(scope): русское описание\" -- <explicit paths>" >&2
  exit 65
fi

if [ -f Makefile ] && grep -qE '^verify:' Makefile; then
  make verify
elif [ -f Makefile ] && grep -qE '^test:' Makefile; then
  make test
fi

if ! git remote get-url origin >/dev/null 2>&1; then
  echo "error: remote origin is missing" >&2
  exit 65
fi

base=${base_override:-}
if [ -z "$base" ]; then
  base_ref=$(metadata_value "$meta" base_ref)
  case "$base_ref" in
    origin/*) base=${base_ref#origin/} ;;
    dev|main) base=$base_ref ;;
  esac
fi
if [ -z "$base" ]; then
  if git show-ref --verify --quiet refs/remotes/origin/dev; then
    base=dev
  elif git show-ref --verify --quiet refs/remotes/origin/main; then
    base=main
  else
    base=main
  fi
fi

task=$(metadata_value "$meta" task)
id=$(metadata_value "$meta" id)
repo=$(metadata_value "$meta" repo)
title=${title_override:-"${task:-agent task}"}
body=$(cat <<BODY
Automated agent task finish.

- Task: ${task:-unknown}
- Worktree id: ${id:-unknown}
- Branch: $branch
- Base: $base

Review before merge. Do not squash unrelated changes.
BODY
)

printf '\n--- verifying branch state ---\n'
git status --short --branch
printf '\n--- pushing branch ---\n'
git push -u origin HEAD
metadata_set cleanup_state pushed
metadata_set pushed_at "$(date '+%Y-%m-%d %H:%M:%S')"
metadata_set pushed_branch "$branch"
metadata_set review_base "$base"

review_url=""
origin_url=$(git remote get-url origin)
if command -v gh >/dev/null 2>&1 && printf '%s' "$origin_url" | grep -qi 'github.com'; then
  if review_url=$(gh pr view "$branch" --json url -q .url 2>/dev/null); then
    :
  else
    review_url=$(gh pr create --draft --base "$base" --head "$branch" --title "$title" --body "$body" 2>/dev/null || true)
  fi
elif command -v glab >/dev/null 2>&1 && printf '%s' "$origin_url" | grep -Eqi 'gitlab|git\.'; then
  if review_url=$(glab mr view "$branch" --output json 2>/dev/null | sed -n 's/.*"web_url":"\([^"]*\)".*/\1/p' | head -1); then
    :
  fi
  if [ -z "$review_url" ]; then
    review_url=$(glab mr create --draft --target-branch "$base" --source-branch "$branch" --title "$title" --description "$body" 2>/dev/null || true)
  fi
else
  echo "warning: no supported PR/MR CLI found for origin: $origin_url" >&2
fi

if [ -n "$review_url" ]; then
  metadata_set cleanup_state review
  metadata_set review_url "$review_url"
  metadata_set review_opened_at "$(date '+%Y-%m-%d %H:%M:%S')"
  printf '\n✓ review opened: %s\n' "$review_url"
  printf 'Worktree state: review. Cleanup after merge or manual approval.\n'
elif [ "$ready_without_review" = "1" ]; then
  metadata_set cleanup_state ready
  metadata_set ready_without_review_at "$(date '+%Y-%m-%d %H:%M:%S')"
  printf '\n✓ branch pushed without review URL; marked ready by explicit flag.\n'
else
  printf '\n✓ branch pushed: %s\n' "$branch"
  printf 'Review was not opened automatically. Open it manually, then update metadata or rerun agent-finish.sh.\n'
fi
