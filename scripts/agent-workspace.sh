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
VAULT_PROJECTS="$HOME/Desktop/WikiPedik/dev/20-projects"

sync_vscode_project_manager() {
  local sync="$HOME/dotfiles/scripts/vscode-projects-sync.py"
  [ -x "$sync" ] || return 0
  "$sync" --no-backup >/dev/null 2>&1 || true
}

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
  agent-workspace stale [days]   # active worktrees idle N+ days with no tmux session
  agent-workspace prune-branches [--dry-run]   # delete merged/pushed agent/* branches not used by any worktree
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
  if ! git -C "$repo_dir" rev-parse --verify HEAD >/dev/null 2>&1; then
    echo __orphan__
    return
  fi

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

# Working-memory salvage: before a worktree dies, copy artifacts that exist
# nowhere else into the WikiPedik vault for curator review.
#   - .agents/oracle/*.md   — second-model review answers (runtime dir, dies with worktree)
#   - docs/epics/*.md       — uncommitted task/epic state (untracked or modified)
# Salvage never deletes anything itself; callers decide what becomes removable.
salvage_worktree_artifacts() {
  local wt=$1 co=$2 prod=$3 repo=$4 id=$5
  local dest="$VAULT_PROJECTS/$co/$prod/repos/$repo/_salvage/$id"
  local salvaged=0 f rel

  [ -n "$co" ] && [ -n "$prod" ] && [ -n "$repo" ] && [ -n "$id" ] || return 0

  for f in "$wt"/.agents/oracle/*.md; do
    [ -f "$f" ] || continue
    mkdir -p "$dest/oracle"
    cp "$f" "$dest/oracle/"
    salvaged=$((salvaged + 1))
  done

  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    [ -f "$wt/$rel" ] || continue
    mkdir -p "$dest/$(dirname "$rel")"
    cp "$wt/$rel" "$dest/$rel"
    salvaged=$((salvaged + 1))
  done < <(git -C "$wt" status --porcelain -- 'docs/epics/*.md' 2>/dev/null | cut -c4-)

  if [ "$salvaged" -gt 0 ]; then
    {
      echo "# salvage: $co/$prod/$repo @ $id"
      echo ""
      echo "branch: $(metadata_value "$wt/.agent-workspace" branch 2>/dev/null)"
      echo "task: $(metadata_value "$wt/.agent-workspace" task 2>/dev/null)"
      echo "salvaged_at: $(date '+%Y-%m-%d %H:%M:%S')"
      echo "files: $salvaged"
      echo ""
      echo "Curator: review during wiki sync — promote durable parts to lessons/epics archive, delete the rest."
    } > "$dest/INFO.md"
    echo "salvaged $salvaged file(s) -> ${dest/#$HOME/\~}"
  fi
  return 0
}

# After salvage, runtime junk and untracked-but-salvaged epic docs may be
# dropped so a finished worktree becomes removable. Tracked files are never
# touched: some repos commit .agents/*/.gitignore, so .agents is cleaned via
# git clean (untracked + ignored only), not rm -rf.
drop_salvaged_junk() {
  local wt=$1 rel
  git -C "$wt" clean -fdxq -- .agents 2>/dev/null || true
  while IFS= read -r rel; do
    [ -n "$rel" ] || continue
    rm -f "$wt/$rel"
  done < <(git -C "$wt" status --porcelain -- 'docs/epics/*.md' 2>/dev/null | awk '$1 == "??"' | cut -c4-)
}

# Delete an agent/* branch only when no work can be lost:
#   - never touches branches attached to an existing worktree (live sessions);
#   - deletes when merged into the base branch OR fully pushed to upstream;
#   - keeps everything else and says why.
prune_branch_if_safe() {
  local repo_dir=$1 branch=$2 dry_run=${3:-0}
  local base_ref reason="" unpushed

  [ -n "$branch" ] || return 0
  case "$branch" in agent/*) ;; *) return 0 ;; esac
  git -C "$repo_dir" show-ref --verify --quiet "refs/heads/$branch" || return 0

  if git -C "$repo_dir" worktree list --porcelain 2>/dev/null | grep -qx "branch refs/heads/$branch"; then
    return 0
  fi

  base_ref=$(base_ref_for "$repo_dir")
  if [ -n "$base_ref" ] && [ "$base_ref" != "__orphan__" ] \
    && git -C "$repo_dir" merge-base --is-ancestor "$branch" "$base_ref" 2>/dev/null; then
    reason="merged into $base_ref"
  elif git -C "$repo_dir" rev-parse --verify --quiet "$branch@{u}" >/dev/null 2>&1; then
    unpushed=$(git -C "$repo_dir" rev-list --count "$branch@{u}..$branch" 2>/dev/null || echo 1)
    if [ "$unpushed" = "0" ]; then
      reason="fully pushed to $(git -C "$repo_dir" rev-parse --abbrev-ref "$branch@{u}" 2>/dev/null)"
    fi
  fi

  if [ -z "$reason" ]; then
    echo "keeping branch $branch (unmerged/unpushed local work)"
    return 0
  fi

  if [ "$dry_run" = "1" ]; then
    echo "would delete branch $branch ($reason)"
  else
    git -C "$repo_dir" branch -D "$branch" >/dev/null
    echo "deleted branch $branch ($reason)"
  fi
}

# Sweep agent/* branches across all main checkouts. Branches attached to a
# worktree are never considered; only merged or fully pushed ones are deleted.
prune_branches() {
  local dry_run=0
  [ "${1:-}" = "--dry-run" ] && dry_run=1
  local repo_dir branch found=0

  local out
  for repo_dir in "$BASE"/*/*/*; do
    [ -d "$repo_dir/.git" ] || continue
    while IFS= read -r branch; do
      [ -n "$branch" ] || continue
      out=$(prune_branch_if_safe "$repo_dir" "$branch" "$dry_run")
      [ -n "$out" ] || continue
      found=1
      printf '%s: %s\n' "${repo_dir#$BASE/}" "$out"
    done < <(git -C "$repo_dir" for-each-ref --format='%(refname:short)' 'refs/heads/agent/**' 'refs/heads/agent/*' 2>/dev/null | sort -u)
  done

  [ "$found" = "0" ] && echo "no agent/* branches found"
  return 0
}

# WikiPedik knowledge symlinks are untracked (kept in info/exclude), so a fresh
# worktree starts without them and SessionStart hooks / lesson-append go dark.
# Mirror them from the main checkout when present.
link_knowledge_symlinks() {
  local repo_dir=$1 wt_dir=$2 name target rule
  for name in knowledge product-knowledge company-knowledge; do
    target=$(readlink "$repo_dir/docs/$name" 2>/dev/null) || continue
    [ -d "$target" ] || continue
    mkdir -p "$wt_dir/docs"
    ln -sfn "$target" "$wt_dir/docs/$name"
  done
  # Path-scoped wiki rules (.claude/rules/wiki-*) are vault symlinks too.
  for rule in "$repo_dir"/.claude/rules/wiki-*; do
    [ -L "$rule" ] || continue
    target=$(readlink "$rule") || continue
    [ -f "$target" ] || continue
    mkdir -p "$wt_dir/.claude/rules"
    ln -sfn "$target" "$wt_dir/.claude/rules/$(basename "$rule")"
  done
}

seed_orphan_worktree() {
  local repo_dir=$1 wt_dir=$2
  find "$repo_dir" -mindepth 1 -maxdepth 1 ! -name .git -exec cp -R {} "$wt_dir" \;
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
    local state wt co prod repo repo_dir branch
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
    co=$(metadata_value "$meta" company)
    prod=$(metadata_value "$meta" product)
    repo=$(metadata_value "$meta" repo)
    repo_dir=$(repo_dir_for "$co" "$prod" "$repo")
    [ -d "$repo_dir/.git" ] || continue
    if [ "$dry_run" != "1" ]; then
      if [ "$quiet" = "1" ]; then
        salvage_worktree_artifacts "$wt" "$co" "$prod" "$repo" "$(metadata_value "$meta" id)" >/dev/null 2>&1 || true
      else
        salvage_worktree_artifacts "$wt" "$co" "$prod" "$repo" "$(metadata_value "$meta" id)" || true
      fi
      drop_salvaged_junk "$wt"
    fi
    is_clean_and_pushed "$wt" || continue
    found=1
    branch=$(metadata_value "$meta" branch)
    if [ "$dry_run" = "1" ]; then
      echo "would remove $wt"
    else
      [ "$quiet" = "1" ] || echo "removing $wt"
      git -C "$repo_dir" worktree remove "$wt"
      if [ "$quiet" = "1" ]; then
        prune_branch_if_safe "$repo_dir" "$branch" >/dev/null 2>&1 || true
      else
        prune_branch_if_safe "$repo_dir" "$branch" || true
      fi
    fi
  done < <(find "$WORKTREES" -name .agent-workspace -type f -print | sort)

  if [ "$found" = "0" ] && [ "$quiet" != "1" ]; then
    echo "no clean pushed ready worktrees to remove"
  fi
  if [ "$dry_run" != "1" ]; then
    sync_vscode_project_manager
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
  local n_stale
  n_stale=$(stale_count 3)
  if [ "${n_stale:-0}" -gt 0 ]; then
    echo "ℹ $n_stale stale active worktree(s) — triage with: agent-workspace stale" >&2
  fi

  base_ref=$(base_ref_for "$repo_dir")
  [ -n "$base_ref" ] || { echo "error: cannot detect base branch for $repo_dir" >&2; exit 65; }

  if [ "$base_ref" = "__orphan__" ]; then
    git -C "$repo_dir" worktree add --quiet --orphan -b "$branch" "$wt_dir" >&2
    seed_orphan_worktree "$repo_dir" "$wt_dir"
  else
    git -C "$repo_dir" worktree add --quiet "$wt_dir" -b "$branch" "$base_ref" >&2
  fi

  git -C "$wt_dir" config user.name "$(company_name "$co")"
  email=$(company_email "$co" || true)
  [ -n "$email" ] && git -C "$wt_dir" config user.email "$email"
  ensure_worktree_excludes "$wt_dir"
  link_knowledge_symlinks "$repo_dir" "$wt_dir"
  write_metadata "$wt_dir" "$co" "$prod" "$repo" "$task" "$id" "$branch" "$base_ref"
  sync_vscode_project_manager
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

# Worktrees in `active` state that nobody is working on: no tmux session and no
# commits for N days. Cleanup never touches them (they may hold unpushed work),
# so they accumulate silently — this view is for human triage.
stale_workspaces() {
  local days=${1:-3} now found=0
  case "$days" in ''|*[!0-9]*) echo "Usage: agent-workspace stale [days]" >&2; exit 64 ;; esac
  [ -d "$WORKTREES" ] || return 0
  now=$(date +%s)

  while IFS= read -r meta; do
    [ -n "$meta" ] || continue
    local wt state co prod repo id session last_ts idle dirty ahead
    wt=${meta%/.agent-workspace}
    state=$(metadata_value "$meta" cleanup_state)
    [ "$state" = "active" ] || continue

    co=$(metadata_value "$meta" company)
    prod=$(metadata_value "$meta" product)
    repo=$(metadata_value "$meta" repo)
    id=$(metadata_value "$meta" id)
    session="${co}-${prod}-${repo}-agent-${id}"
    tmux has-session -t "$session" 2>/dev/null && continue

    last_ts=$(git -C "$wt" log -1 --format=%ct 2>/dev/null || echo 0)
    [ "$last_ts" -gt 0 ] || last_ts=$(stat -f %m "$meta" 2>/dev/null || echo "$now")
    idle=$(( (now - last_ts) / 86400 ))
    [ "$idle" -ge "$days" ] || continue

    dirty=$(git -C "$wt" status --short 2>/dev/null | wc -l | tr -d ' ')
    ahead=$(git -C "$wt" rev-list --count '@{u}..HEAD' 2>/dev/null || echo '?')
    found=$((found + 1))
    printf '%s | idle=%sd | dirty=%s | unpushed=%s\n' "${wt#$WORKTREES/}" "$idle" "$dirty" "$ahead"
  done < <(find "$WORKTREES" -name .agent-workspace -type f -print | sort)

  if [ "$found" = "0" ]; then
    echo "no stale active worktrees (idle >= ${days}d, no tmux session)"
  else
    echo ""
    echo "triage: push unfinished work, 'agent-workspace ready ...' clean ones, or 'agent-workspace remove ...' abandoned ones"
  fi
}

stale_count() {
  stale_workspaces "${1:-3}" 2>/dev/null | grep -c '| idle=' || true
}

# Reap a single worktree when its tmux session closes (session-closed hook).
# Best-effort and SAFE: salvages artifacts, then removes the worktree ONLY if
# it is clean and fully pushed (or already ready). Dirty / unpushed work is
# left untouched — closing a window must never destroy unsynced work; such a
# worktree is picked up later by `stale`/`cleanup`. Quiet: meant for a hook.
reap_workspace() {
  local wt_dir=${1:-} meta co prod repo id branch repo_dir state
  [ -n "$wt_dir" ] && [ -d "$wt_dir" ] || return 0
  meta="$wt_dir/.agent-workspace"
  [ -f "$meta" ] || return 0

  co=$(metadata_value "$meta" company)
  prod=$(metadata_value "$meta" product)
  repo=$(metadata_value "$meta" repo)
  id=$(metadata_value "$meta" id)
  branch=$(metadata_value "$meta" branch)
  state=$(metadata_value "$meta" cleanup_state)
  repo_dir=$(repo_dir_for "$co" "$prod" "$repo")
  [ -d "$repo_dir/.git" ] || return 0

  salvage_worktree_artifacts "$wt_dir" "$co" "$prod" "$repo" "$id" >/dev/null 2>&1 || true
  drop_salvaged_junk "$wt_dir" >/dev/null 2>&1 || true

  # Keep unless safe to drop: clean+pushed, or already marked ready.
  if ! is_clean_and_pushed "$wt_dir" && [ "$state" != "ready" ]; then
    return 0
  fi

  git -C "$repo_dir" worktree remove "$wt_dir" >/dev/null 2>&1 || return 0
  prune_branch_if_safe "$repo_dir" "$branch" >/dev/null 2>&1 || true
  sync_vscode_project_manager >/dev/null 2>&1 || true
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
  sync_vscode_project_manager
  echo "marked ready: $wt_dir"
}

remove_workspace() {
  [ $# -eq 4 ] || { usage; exit 64; }
  local co=$1 prod=$2 repo=$3 id=$4 wt_dir repo_dir branch
  wt_dir=$(worktree_dir_for "$co" "$prod" "$repo" "$id")
  repo_dir=$(repo_dir_for "$co" "$prod" "$repo")
  [ -e "$wt_dir" ] || { echo "not found: $wt_dir" >&2; exit 66; }
  salvage_worktree_artifacts "$wt_dir" "$co" "$prod" "$repo" "$id"
  drop_salvaged_junk "$wt_dir"
  if [ -n "$(git -C "$wt_dir" status --short 2>/dev/null || true)" ]; then
    echo "error: worktree is dirty; commit/push or clean it before remove" >&2
    exit 65
  fi
  branch=$(metadata_value "$wt_dir/.agent-workspace" branch 2>/dev/null || true)
  git -C "$repo_dir" worktree remove "$wt_dir"
  prune_branch_if_safe "$repo_dir" "$branch"
  sync_vscode_project_manager
}

# Compact tmux-formatted indicator of what `reap` (session close) would do to a
# worktree. Meant to be embedded in status-right via #(...): tmux refreshes it
# every status-interval seconds, so the user always sees the live verdict.
#   green  ● synced      → closing the window removes the worktree (nothing lost)
#   yellow ● 3✎ 2↑ hold  → uncommitted/unpushed work; closing keeps it
reap_status() {
  local wt=${1:-} state dirty ahead
  [ -n "$wt" ] || return 0
  # Fast bail if the path is unreadable (e.g. tmux server without TCC access to
  # Desktop): never block or spam the status bar.
  ls "$wt" >/dev/null 2>&1 || { printf '#[fg=red]● ?#[default]'; return 0; }
  [ -d "$wt" ] || return 0
  [ -f "$wt/.agent-workspace" ] || return 0

  state=$(metadata_value "$wt/.agent-workspace" cleanup_state 2>/dev/null)
  dirty=$(git -C "$wt" status --porcelain 2>/dev/null | grep -c . || true)
  ahead=$(git -C "$wt" rev-list --count '@{u}..HEAD' 2>/dev/null || echo 0)

  if [ "${dirty:-0}" -eq 0 ] && { [ "${ahead:-0}" -eq 0 ] || [ "$state" = "ready" ]; }; then
    printf '#[fg=green]● synced#[default]'
  else
    local parts=""
    [ "${dirty:-0}" -gt 0 ] && parts="${dirty}✎"
    [ "${ahead:-0}" -gt 0 ] && parts="${parts:+$parts }${ahead}↑"
    printf '#[fg=yellow]● %s hold#[default]' "$parts"
  fi
}

cmd=${1:-}
shift || true
case "$cmd" in
  start) start_workspace "$@" ;;
  reap-status) reap_status "$@" ;;
  launch) start_workspace "$@" ;;
  list) list_workspaces ;;
  status) status_workspaces ;;
  cleanup) cleanup_workspaces "$@" ;;
  ready) ready_workspace "$@" ;;
  remove) remove_workspace "$@" ;;
  stale) stale_workspaces "$@" ;;
  prune-branches) prune_branches "$@" ;;
  reap) reap_workspace "$@" ;;
  *) usage; exit 64 ;;
esac
