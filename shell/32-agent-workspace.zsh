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
        wt="$HOME/Desktop/Prokectfiles/.worktrees/$1/$2/$3/$4"
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
      # Re-create a detached session for every active worktree that has none.
      shift
      local meta wt co prod repo id session opened=0
      for meta in "$HOME/Desktop/Prokectfiles/.worktrees"/**/.agent-workspace(N); do
        wt="${meta:h}"
        co=$(awk -F= '$1=="company"{print $2}' "$meta")
        prod=$(awk -F= '$1=="product"{print $2}' "$meta")
        repo=$(awk -F= '$1=="repo"{print $2}' "$meta")
        id=$(awk -F= '$1=="id"{print $2}' "$meta")
        session="${co}-${prod}-${repo}-agent-${id}"
        tmux has-session -t "$session" 2>/dev/null && continue
        echo "→ reopening $repo/$id"
        _launch_session_path "$session" "$wt" "🖥 ${repo}/${id}" detached
        opened=$((opened + 1))
      done
      echo "reopened $opened worktree session(s). Attach: tmux attach -t <name>, or 'tmux ls'."
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
  agent-workspace reopen-all          # detached session per active worktree without one
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
