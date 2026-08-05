# Managed agent workspaces built on git worktree.
#
# The tmux layer that used to sit on top of this is gone: work happens in the
# editor now, so a command that painted four panes was ceremony around the one
# thing that mattered — an isolated worktree with the right git identity.
#
# This wrapper exists because `cd` has to happen in the calling shell; every
# other subcommand is passed straight through to the script.

agent-workspace() {
  local cmd=${1:-}

  case "$cmd" in
    start)
      shift
      [ $# -eq 4 ] || { echo "Usage: agent-workspace start <company> <product> <repo> <task-slug>" >&2; return 1; }
      local wt
      wt=$(bash "$HOME/dotfiles/scripts/agent-workspace.sh" start "$@") || return $?
      _workspace_enter "$wt"
      ;;

    open)
      shift
      [ $# -eq 1 ] || [ $# -eq 4 ] || { echo "Usage: agent-workspace open <worktree-path> | <company> <product> <repo> <id>" >&2; return 1; }
      local wt
      if [ $# -eq 1 ]; then
        wt=$1
      else
        wt="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/.worktrees/$1/$2/$3/$4"
      fi
      [ -f "$wt/.agent-workspace" ] || { echo "⚠ это не управляемый worktree: $wt" >&2; return 1; }
      _workspace_enter "$wt"
      ;;

    list|status|cleanup|ready|remove|stale|prune-branches)
      bash "$HOME/dotfiles/scripts/agent-workspace.sh" "$@"
      ;;

    *)
      cat <<'EOF'
Usage:
  agent-workspace start  <company> <product> <repo> <task-slug>
  agent-workspace open   <worktree-path> | <company> <product> <repo> <id>
  agent-workspace list
  agent-workspace status
  agent-workspace stale [days]
  agent-workspace ready  <company> <product> <repo> <worktree-id>
  agent-workspace remove <company> <product> <repo> <worktree-id>
  agent-workspace cleanup [--days N] [--dry-run]
  agent-workspace prune-branches [--dry-run]

Ярлыки продуктов делают то же самое короче:
  agents synapse ticket-quality
  healler epic-04
EOF
      return 1
      ;;
  esac
}

# _workspace_enter moves the shell into a worktree and reports where it is, so
# the next command — opening the editor, running an agent — starts in the right
# place.
_workspace_enter() {
  local wt=$1
  [ -d "$wt" ] || { echo "⚠ нет каталога: $wt" >&2; return 1; }

  cd "$wt" || return 1
  print -P "%F{cyan}$wt%f"

  local branch
  branch=$(git -C "$wt" rev-parse --abbrev-ref HEAD 2>/dev/null)
  [ -n "$branch" ] && print -P "%F{244}ветка $branch%f"
}
