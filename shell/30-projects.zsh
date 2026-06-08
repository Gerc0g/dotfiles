# Project launch shortcuts.
#
# Product commands are the daily entrypoint for writing work. They always open
# the standard 4-pane layout inside a managed git worktree so agents do not
# write into shared main/dev checkouts by accident.

_project_safe_launch() {
  local co=$1 prod=$2 repo=$3 task=${4:-work}

  [ -n "$repo" ] || { echo "Repo is required." >&2; return 1; }
  [ -n "$task" ] || task=work

  agent-workspace launch "$co" "$prod" "$repo" "$task"
}

_project_launch() {
  local co=$1 prod=$2 default_repo=$3
  shift 3

  if [ "${1:-}" = "--agent" ]; then
    local task=${2:-}
    local repo=${3:-$default_repo}
    [ -n "$task" ] || { echo "Usage: $prod --agent <task-slug> [<repo>]"; return 1; }
    [ -n "$repo" ] || { echo "Usage: $prod --agent <task-slug> <repo>"; return 1; }
    _project_safe_launch "$co" "$prod" "$repo" "$task"
    return $?
  fi

  case $# in
    0)
      if [ -n "$default_repo" ]; then
        _project_safe_launch "$co" "$prod" "$default_repo"
      else
        local repos=($(_launch_list_repos "$co" "$prod"))
        [ ${#repos[@]} -eq 0 ] && { echo "⚠ No repos in $co/$prod"; return 1; }
        local repo
        repo=$(_launch_pick "Repos in ${co}/${prod}:" "${repos[@]}") || return 1
        _project_safe_launch "$co" "$prod" "$repo"
      fi
      ;;
    1)
      if [ -n "$default_repo" ]; then
        _project_safe_launch "$co" "$prod" "$default_repo" "$1"
      else
        _project_safe_launch "$co" "$prod" "$1"
      fi
      ;;
    2)
      if [ -n "$default_repo" ]; then
        echo "Usage: $prod [<task-slug>] | $prod --agent <task-slug> [<repo>]" >&2
        return 1
      fi
      _project_safe_launch "$co" "$prod" "$1" "$2"
      ;;
    *)
      echo "Usage: $prod [<repo> [<task-slug>]] | $prod --agent <task-slug> [<repo>]"
      return 1
      ;;
  esac
}

infra() {
  _project_launch neurodesk infra infra "$@"
}

legacy() {
  _project_launch neurodesk legacy "" "$@"
}

agents() {
  _project_launch neurodesk agents "" "$@"
}

wiki() {
  case "${1:-}" in
    sync)
      shift
      wiki-sync "$@"
      ;;
    status)
      shift
      wiki-status "$@"
      ;;
    synthesize)
      shift
      wiki-synthesize "$@"
      ;;
    bootstrap)
      shift
      wiki-bootstrap-product "$@"
      ;;
    *)
      _project_launch neurodesk wiki nrdsk_wiki "$@"
      ;;
  esac
}

saas() {
  _project_launch neurodesk saas "" "$@"
}

aetheria() {
  _project_launch chimera aetheria "" "$@"
}

homeless() {
  _project_launch chimera homeless "" "$@"
}

healler() {
  _project_launch chimera homeless Healler "$@"
}

neuroslop4ik() {
  _project_launch chimera neuroslop4ik NeuroSlop4ik "$@"
}

comonline() {
  _project_launch chimera comonline ComaOnline-Sources "$@"
}

clients() {
  case $# in
    0) launch "SupportOps-Core" "clients" "adminka" ;;
    1) launch "SupportOps-Core" "clients" "$1" ;;
    *) echo "Usage: clients [<repo>]"; return 1 ;;
  esac
}
