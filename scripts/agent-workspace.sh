#!/usr/bin/env bash
# Managed git worktrees for isolated agent tasks.
# Usage:
#   agent-workspace.sh start  <company> <product> <repo> <task-slug>
#   agent-workspace.sh launch <company> <product> <repo> <task-slug>
#   agent-workspace.sh list
#   agent-workspace.sh status
#   agent-workspace.sh remove <company> <product> <repo> <task-slug>

set -euo pipefail

BASE="$HOME/Desktop/Prokectfiles"
WORKTREES="$BASE/.worktrees"

usage() {
  cat >&2 <<'EOF'
Usage:
  agent-workspace start  <company> <product> <repo> <task-slug>
  agent-workspace launch <company> <product> <repo> <task-slug>
  agent-workspace list
  agent-workspace status
  agent-workspace remove <company> <product> <repo> <task-slug>
EOF
}

slugify() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9._-]+/-/g; s/^-+//; s/-+$//; s/-+/-/g'
}

repo_dir_for() {
  printf '%s/%s/%s/%s' "$BASE" "$1" "$2" "$3"
}

worktree_dir_for() {
  printf '%s/%s/%s/%s/%s' "$WORKTREES" "$1" "$2" "$3" "$4"
}

company_email() {
  local cfg="$BASE/$1/.company-config"
  [ -f "$cfg" ] && awk '/^git_email:/ {print $2; exit}' "$cfg"
}

company_name() {
  echo "egor"
}

base_ref_for() {
  local repo_dir=$1
  git -C "$repo_dir" fetch origin --prune >/dev/null 2>&1 || true
  if git -C "$repo_dir" show-ref --verify --quiet refs/remotes/origin/dev; then
    echo origin/dev
  elif git -C "$repo_dir" show-ref --verify --quiet refs/remotes/origin/main; then
    echo origin/main
  elif git -C "$repo_dir" show-ref --verify --quiet refs/heads/dev; then
    echo dev
  elif git -C "$repo_dir" show-ref --verify --quiet refs/heads/main; then
    echo main
  else
    git -C "$repo_dir" branch --show-current
  fi
}

write_metadata() {
  local wt_dir=$1 co=$2 prod=$3 repo=$4 task=$5 branch=$6 base_ref=$7
  cat > "$wt_dir/.agent-workspace" <<EOF
company=$co
product=$prod
repo=$repo
task=$task
branch=$branch
base_ref=$base_ref
created_at=$(date '+%Y-%m-%d %H:%M:%S')
EOF
}

start_workspace() {
  [ $# -eq 4 ] || { usage; exit 64; }
  local co=$1 prod=$2 repo=$3 task_raw=$4
  local task branch repo_dir wt_dir base_ref email
  task=$(slugify "$task_raw")
  [ -n "$task" ] || { echo "error: empty task slug" >&2; exit 64; }
  branch="agent/$task"
  repo_dir=$(repo_dir_for "$co" "$prod" "$repo")
  wt_dir=$(worktree_dir_for "$co" "$prod" "$repo" "$task")

  [ -d "$repo_dir/.git" ] || { echo "error: not a git repo: $repo_dir" >&2; exit 66; }
  mkdir -p "$(dirname "$wt_dir")"

  if [ -d "$wt_dir/.git" ] || [ -f "$wt_dir/.git" ]; then
    echo "$wt_dir"
    return 0
  fi

  base_ref=$(base_ref_for "$repo_dir")
  [ -n "$base_ref" ] || { echo "error: cannot detect base branch for $repo_dir" >&2; exit 65; }

  if git -C "$repo_dir" show-ref --verify --quiet "refs/heads/$branch"; then
    git -C "$repo_dir" worktree add "$wt_dir" "$branch"
  else
    git -C "$repo_dir" worktree add "$wt_dir" -b "$branch" "$base_ref"
  fi

  git -C "$wt_dir" config user.name "$(company_name "$co")"
  email=$(company_email "$co" || true)
  [ -n "$email" ] && git -C "$wt_dir" config user.email "$email"
  write_metadata "$wt_dir" "$co" "$prod" "$repo" "$task" "$branch" "$base_ref"
  echo "$wt_dir"
}

list_workspaces() {
  [ -d "$WORKTREES" ] || return 0
  find "$WORKTREES" -name .agent-workspace -print | sort | while IFS= read -r meta; do
    wt=${meta%/.agent-workspace}
    branch=$(git -C "$wt" branch --show-current 2>/dev/null || true)
    dirty=$(git -C "$wt" status --short 2>/dev/null | wc -l | tr -d ' ')
    printf '%s | branch=%s | dirty=%s\n' "${wt#$WORKTREES/}" "${branch:-?}" "$dirty"
  done
}

status_workspaces() {
  [ -d "$WORKTREES" ] || { echo "no agent workspaces"; return 0; }
  find "$WORKTREES" -name .agent-workspace -print | sort | while IFS= read -r meta; do
    wt=${meta%/.agent-workspace}
    echo "== $wt"
    git -C "$wt" status --short --branch || true
  done
}

remove_workspace() {
  [ $# -eq 4 ] || { usage; exit 64; }
  local co=$1 prod=$2 repo=$3 task_raw=$4 task wt_dir
  task=$(slugify "$task_raw")
  wt_dir=$(worktree_dir_for "$co" "$prod" "$repo" "$task")
  [ -e "$wt_dir" ] || { echo "not found: $wt_dir" >&2; exit 66; }
  if [ -n "$(git -C "$wt_dir" status --short 2>/dev/null || true)" ]; then
    echo "error: worktree is dirty; commit/push or clean it before remove" >&2
    exit 65
  fi
  git -C "$(repo_dir_for "$co" "$prod" "$repo")" worktree remove "$wt_dir"
}

cmd=${1:-}
shift || true
case "$cmd" in
  start) start_workspace "$@" ;;
  launch) start_workspace "$@" ;;
  list) list_workspaces ;;
  status) status_workspaces ;;
  remove) remove_workspace "$@" ;;
  *) usage; exit 64 ;;
esac
