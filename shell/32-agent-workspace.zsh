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

# _workspace_enter moves the shell into a worktree and opens it in the editor.
#
# Both halves matter: the editor is where the work happens, and the shell has
# to follow so the next command — an agent, a test run, a git call — starts in
# the same place.
_workspace_enter() {
  local wt=$1
  [ -d "$wt" ] || { echo "⚠ нет каталога: $wt" >&2; return 1; }

  cd "$wt" || return 1
  print -P "%F{cyan}$wt%f"

  local branch
  branch=$(git -C "$wt" rev-parse --abbrev-ref HEAD 2>/dev/null)
  [ -n "$branch" ] && print -P "%F{244}ветка $branch%f"

  _workspace_open_editor "$wt"
}

# _workspace_open_editor opens a directory in the configured editor.
#
# HQ_EDITOR picks the binary (default: VS Code), HQ_OPEN_EDITOR=0 turns the
# whole thing off — useful over SSH and on the server, where launching a GUI
# is either impossible or wrong. A missing editor is not an error: the shell
# already moved into the directory, which is the part that must not fail.
_workspace_open_editor() {
  local dir=$1
  local editor=${HQ_EDITOR:-code}

  [ "${HQ_OPEN_EDITOR:-1}" = "0" ] && return 0
  command -v "$editor" >/dev/null 2>&1 || return 0

  "$editor" "$dir" >/dev/null 2>&1
  print -P "%F{244}открыто в $editor%f"
}
