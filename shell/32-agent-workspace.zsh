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
    start|list|status|cleanup|ready|remove)
      bash "$HOME/dotfiles/scripts/agent-workspace.sh" "$@"
      ;;
    *)
      cat <<'EOF'
Usage:
  agent-workspace launch <company> <product> <repo> <task-slug>
  agent-workspace start  <company> <product> <repo> <task-slug>
  agent-workspace list
  agent-workspace status
  agent-workspace cleanup [--days N] [--dry-run]
  agent-workspace ready <company> <product> <repo> <worktree-id>
  agent-workspace remove <company> <product> <repo> <worktree-id>

Daily project shortcuts call this automatically:
  healler epic-04
  agents synapse ticket-quality
  comonline ingest-archive
EOF
      return 1
      ;;
  esac
}
