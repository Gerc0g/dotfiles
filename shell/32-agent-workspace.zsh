# Managed agent workspaces built on git worktree.
# Daily project launch commands use this by default. Keep this command as the
# lower-level escape hatch for listing/removing worktrees.

agent-workspace() {
  local cmd=${1:-}
  case "$cmd" in
    launch)
      shift
      [ $# -eq 4 ] || { echo "Usage: agent-workspace launch <company> <product> <repo> <task-slug>"; return 1; }
      local co=$1 prod=$2 repo=$3 task=$4
      local wt task_slug session label
      wt=$(bash "$HOME/dotfiles/scripts/agent-workspace.sh" start "$co" "$prod" "$repo" "$task") || return $?
      task_slug=$(basename "$wt")
      session="${co}-${prod}-${repo}-agent-${task_slug}"
      label="🖥 ${repo}/${task_slug}"
      _launch_session_path "$session" "$wt" "$label"
      ;;
    open|reopen)
      # Reopen the standard 4-pane layout on an EXISTING worktree (no new id).
      shift
      [ $# -eq 1 ] || [ $# -eq 4 ] || { echo "Usage: agent-workspace open <worktree-path> | <company> <product> <repo> <id>"; return 1; }
      local wt
      if [ $# -eq 1 ]; then
        wt=$1
      else
        wt="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/.worktrees/$1/$2/$3/$4"
      fi
      [ -f "$wt/.agent-workspace" ] || { echo "⚠ not a managed worktree: $wt"; return 1; }
      local co prod repo id session label
      co=$(awk -F= '$1=="company"{print $2}' "$wt/.agent-workspace")
      prod=$(awk -F= '$1=="product"{print $2}' "$wt/.agent-workspace")
      repo=$(awk -F= '$1=="repo"{print $2}' "$wt/.agent-workspace")
      id=$(awk -F= '$1=="id"{print $2}' "$wt/.agent-workspace")
      session="${co}-${prod}-${repo}-agent-${id}"
      label="🖥 ${repo}/${id}"
      _launch_session_path "$session" "$wt" "$label"
      ;;
    reopen-all)
      # Universal: detached session for EVERY worktree under .worktrees that
      # has no live tmux session — managed (with .agent-workspace) AND manual
      # (plain `git worktree add`). Identity is derived from the path
      # .worktrees/<co>/<prod>/<repo>/<id>, so it works for all companies,
      # products and repos without any per-project config.
      shift
      local only_co=${1:-}
      local wtroot="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/.worktrees"
      local gitf wt rel co prod repo id session opened=0 skipped=0
      # (N.) = nullglob + regular files only: a git worktree has .git as a FILE
      # ("gitdir: …"); a nested clone/submodule has .git as a directory — skip it.
      for gitf in "$wtroot"/*/*/*/*/.git(N.); do
        wt="${gitf:h}"
        rel="${wt#$wtroot/}"            # co/prod/repo/id
        co="${rel%%/*}";  rel="${rel#*/}"
        prod="${rel%%/*}"; rel="${rel#*/}"
        repo="${rel%%/*}"; id="${rel#*/}"
        [ -n "$only_co" ] && [ "$co" != "$only_co" ] && continue
        session="${co}-${prod}-${repo}-agent-${id}"
        if tmux has-session -t "$session" 2>/dev/null; then
          skipped=$((skipped + 1)); continue
        fi
        echo "→ reopening $co/$prod/$repo/$id"
        if _launch_session_path "$session" "$wt" "🖥 ${repo}/${id}" detached; then
          opened=$((opened + 1))
        fi
      done
      echo "reopened $opened session(s), $skipped already live. Attach: 'tmux ls' → tmux attach -t <name>."
      ;;
    start|list|status|cleanup|ready|remove|stale|prune-branches)
      bash "$HOME/dotfiles/scripts/agent-workspace.sh" "$@"
      ;;
    *)
      cat <<'EOF'
Usage:
  agent-workspace launch <company> <product> <repo> <task-slug>
  agent-workspace start  <company> <product> <repo> <task-slug>
  agent-workspace open   <worktree-path> | <company> <product> <repo> <id>
  agent-workspace reopen-all [company]   # detached session per worktree without one (managed + manual), any co/prod/repo
  agent-workspace list
  agent-workspace status
  agent-workspace cleanup [--days N] [--dry-run]
  agent-workspace ready <company> <product> <repo> <worktree-id>
  agent-workspace remove <company> <product> <repo> <worktree-id>
  agent-workspace stale [days]
  agent-workspace prune-branches [--dry-run]

Daily project shortcuts call this automatically:
  healler epic-04
  agents synapse ticket-quality
  comonline ingest-archive
EOF
      return 1
      ;;
  esac
}
