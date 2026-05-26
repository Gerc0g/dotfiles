# Managed agent workspaces built on git worktree.
# Normal launch remains unchanged. Use `<project> --agent <task>` for isolated work.

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
    start|list|status|remove)
      bash "$HOME/dotfiles/scripts/agent-workspace.sh" "$@"
      ;;
    *)
      cat <<'EOF'
Usage:
  agent-workspace launch <company> <product> <repo> <task-slug>
  agent-workspace start  <company> <product> <repo> <task-slug>
  agent-workspace list
  agent-workspace status
  agent-workspace remove <company> <product> <repo> <task-slug>

Project shortcuts also support:
  healler --agent epic-04
  neuroslop4ik --agent script-runtime
  comonline --agent ingest-archive
EOF
      return 1
      ;;
  esac
}
