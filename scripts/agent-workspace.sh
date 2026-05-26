#!/usr/bin/env bash
# Managed git worktrees for isolated agent tasks.
# Usage:
#   agent-workspace.sh start  <company> <product> <repo> <task-slug>
#   agent-workspace.sh launch <company> <product> <repo> <task-slug>
#   agent-workspace.sh list
#   agent-workspace.sh status
#   agent-workspace.sh cleanup [--days N] [--dry-run]
#   agent-workspace.sh remove <company> <product> <repo> <worktree-id>

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
  agent-workspace cleanup [--days N] [--dry-run]
  agent-workspace ready <company> <product> <repo> <worktree-id>
  agent-workspace remove <company> <product> <repo> <worktree-id>
EOF
}

slugify() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9._-]+/-/g; s/^-+//; s/-+$//; s/-+/-/g'
}

short_id() {
  if command -v uuidgen >/dev/null 2>&1; then
    uuidgen | tr '[:upper:]' '[:lower:]' | tr -d '-' | cut -c1-8
  else
    printf '%s%04d' "$(date +%H%M%S)" "$((RANDOM % 10000))"
  fi
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
  local wt_dir=$1 co=$2 prod=$3 repo=$4 task=$5 id=$6 branch=$7 base_ref=$8
  cat > "$wt_dir/.agent-workspace" <<EOF
id=$id
company=$co
product=$prod
repo=$repo
task=$task
branch=$branch
base_ref=$base_ref
cleanup_state=active
created_at=$(date '+%Y-%m-%d %H:%M:%S')
EOF
}

metadata_value() {
  local file=$1 key=$2
  awk -F= -v key="$key" '$1 == key { print substr($0, length(key) + 2); exit }' "$file" 2>/dev/null || true
}

ensure_worktree_excludes() {
  local wt_dir=$1 exclude_file
  exclude_file=$(git -C "$wt_dir" rev-parse --git-path info/exclude)
  mkdir -p "$(dirname "$exclude_file")"
  touch "$exclude_file"
  grep -qxF '.agent-workspace' "$exclude_file" || echo '.agent-workspace' >> "$exclude_file"
}

mark_ready_metadata() {
  local file=$1 branch=$2 tmp
  [ -f "$file" ] || return 0
  tmp=$(mktemp)
  awk -F= '$1 != "cleanup_state" && $1 != "pushed_at" && $1 != "pushed_branch"' "$file" > "$tmp"
  {
    cat "$tmp"
    echo "cleanup_state=ready"
    echo "pushed_at=$(date '+%Y-%m-%d %H:%M:%S')"
    echo "pushed_branch=$branch"
  } > "$file"
  rm -f "$tmp"
}

is_clean_and_pushed() {
  local wt=$1
  [ -z "$(git -C "$wt" status --short 2>/dev/null)" ] || return 1
  git -C "$wt" rev-parse --abbrev-ref --symbolic-full-name '@{u}' >/dev/null 2>&1 || return 1
  [ -z "$(git -C "$wt" log --oneline '@{u}..HEAD' 2>/dev/null)" ] || return 1
}

review_is_merged() {
  local meta=$1 url state
  url=$(metadata_value "$meta" review_url)
  [ -n "$url" ] || return 1
  command -v gh >/dev/null 2>&1 || return 1
  state=$(gh pr view "$url" --json state -q .state 2>/dev/null || true)
  [ "$state" = "MERGED" ]
}

cleanup_workspaces() {
  local days=7 dry_run=0 quiet=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --days) days=${2:-}; shift 2 ;;
      --dry-run) dry_run=1; shift ;;
      --quiet) quiet=1; shift ;;
      *) echo "Usage: agent-workspace cleanup [--days N] [--dry-run]" >&2; exit 64 ;;
    esac
  done
  case "$days" in ''|*[!0-9]*) echo "error: --days must be a non-negative integer" >&2; exit 64 ;; esac

  [ -d "$WORKTREES" ] || return 0
  local found=0
  while IFS= read -r meta; do
    [ -n "$meta" ] || continue
    local state wt co prod repo repo_dir
    state=$(metadata_value "$meta" cleanup_state)
    if [ "$state" != "ready" ]; then
      if [ "$state" = "review" ] && review_is_merged "$meta"; then
        :
      else
        continue
      fi
    fi
    # Use metadata mtime after agent-task-push marked the workspace ready.
    if [ "$days" -gt 0 ] && ! find "$meta" -mtime +$((days - 1)) -print -quit | grep -q .; then
      continue
    fi
    wt=${meta%/.agent-workspace}
    is_clean_and_pushed "$wt" || continue
    co=$(metadata_value "$meta" company)
    prod=$(metadata_value "$meta" product)
    repo=$(metadata_value "$meta" repo)
    repo_dir=$(repo_dir_for "$co" "$prod" "$repo")
    [ -d "$repo_dir/.git" ] || continue
    found=1
    if [ "$dry_run" = "1" ]; then
      echo "would remove $wt"
    else
      [ "$quiet" = "1" ] || echo "removing $wt"
      git -C "$repo_dir" worktree remove "$wt"
    fi
  done < <(find "$WORKTREES" -name .agent-workspace -type f -print | sort)

  if [ "$found" = "0" ] && [ "$quiet" != "1" ]; then
    echo "no clean pushed ready worktrees to remove"
  fi
}

start_workspace() {
  [ $# -eq 4 ] || { usage; exit 64; }
  local co=$1 prod=$2 repo=$3 task_raw=$4
  local task id branch repo_dir wt_dir base_ref email
  task=$(slugify "$task_raw")
  [ -n "$task" ] || { echo "error: empty task slug" >&2; exit 64; }
  id=$(short_id)
  branch="agent/$task-$id"
  repo_dir=$(repo_dir_for "$co" "$prod" "$repo")
  wt_dir=$(worktree_dir_for "$co" "$prod" "$repo" "$id")

  [ -d "$repo_dir/.git" ] || { echo "error: not a git repo: $repo_dir" >&2; exit 66; }
  mkdir -p "$(dirname "$wt_dir")"

  while [ -e "$wt_dir" ] || git -C "$repo_dir" show-ref --verify --quiet "refs/heads/$branch"; do
    id=$(short_id)
    branch="agent/$task-$id"
    wt_dir=$(worktree_dir_for "$co" "$prod" "$repo" "$id")
  done

  cleanup_workspaces --days 7 --quiet || true

  base_ref=$(base_ref_for "$repo_dir")
  [ -n "$base_ref" ] || { echo "error: cannot detect base branch for $repo_dir" >&2; exit 65; }

  git -C "$repo_dir" worktree add --quiet "$wt_dir" -b "$branch" "$base_ref" >&2

  git -C "$wt_dir" config user.name "$(company_name "$co")"
  email=$(company_email "$co" || true)
  [ -n "$email" ] && git -C "$wt_dir" config user.email "$email"
  ensure_worktree_excludes "$wt_dir"
  write_metadata "$wt_dir" "$co" "$prod" "$repo" "$task" "$id" "$branch" "$base_ref"
  echo "$wt_dir"
}

list_workspaces() {
  [ -d "$WORKTREES" ] || return 0
  find "$WORKTREES" -name .agent-workspace -type f -print | sort | while IFS= read -r meta; do
    wt=${meta%/.agent-workspace}
    id=$(metadata_value "$meta" id)
    task=$(metadata_value "$meta" task)
    state=$(metadata_value "$meta" cleanup_state)
    branch=$(git -C "$wt" branch --show-current 2>/dev/null || true)
    dirty=$(git -C "$wt" status --short 2>/dev/null | wc -l | tr -d ' ')
    printf '%s | id=%s | task=%s | branch=%s | state=%s | dirty=%s\n' "${wt#$WORKTREES/}" "${id:-$(basename "$wt")}" "${task:-?}" "${branch:-?}" "${state:-?}" "$dirty"
  done
}

status_workspaces() {
  [ -d "$WORKTREES" ] || { echo "no agent workspaces"; return 0; }
  find "$WORKTREES" -name .agent-workspace -type f -print | sort | while IFS= read -r meta; do
    wt=${meta%/.agent-workspace}
    echo "== $wt"
    echo "task=$(metadata_value "$meta" task) state=$(metadata_value "$meta" cleanup_state)"
    git -C "$wt" status --short --branch || true
  done
}


ready_workspace() {
  [ $# -eq 4 ] || { usage; exit 64; }
  local co=$1 prod=$2 repo=$3 id=$4 wt_dir meta tmp
  wt_dir=$(worktree_dir_for "$co" "$prod" "$repo" "$id")
  meta="$wt_dir/.agent-workspace"
  [ -f "$meta" ] || { echo "not found: $meta" >&2; exit 66; }
  is_clean_and_pushed "$wt_dir" || {
    echo "error: worktree must be clean and fully pushed before marking ready" >&2
    exit 65
  }
  tmp=$(mktemp)
  awk -F= '$1 != "cleanup_state" && $1 != "ready_at"' "$meta" > "$tmp"
  {
    cat "$tmp"
    echo "cleanup_state=ready"
    echo "ready_at=$(date '+%Y-%m-%d %H:%M:%S')"
  } > "$meta"
  rm -f "$tmp"
  echo "marked ready: $wt_dir"
}

remove_workspace() {
  [ $# -eq 4 ] || { usage; exit 64; }
  local co=$1 prod=$2 repo=$3 id=$4 wt_dir repo_dir
  wt_dir=$(worktree_dir_for "$co" "$prod" "$repo" "$id")
  repo_dir=$(repo_dir_for "$co" "$prod" "$repo")
  [ -e "$wt_dir" ] || { echo "not found: $wt_dir" >&2; exit 66; }
  if [ -n "$(git -C "$wt_dir" status --short 2>/dev/null || true)" ]; then
    echo "error: worktree is dirty; commit/push or clean it before remove" >&2
    exit 65
  fi
  git -C "$repo_dir" worktree remove "$wt_dir"
}

cmd=${1:-}
shift || true
case "$cmd" in
  start) start_workspace "$@" ;;
  launch) start_workspace "$@" ;;
  list) list_workspaces ;;
  status) status_workspaces ;;
  cleanup) cleanup_workspaces "$@" ;;
  ready) ready_workspace "$@" ;;
  remove) remove_workspace "$@" ;;
  *) usage; exit 64 ;;
esac
