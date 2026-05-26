# Project launch shortcuts.
#
# Product commands are thin wrappers around `launch`, so a repo opens with
# the standard 4-window layout: plan / code / test / oracle.

_project_launch() {
  local co=$1 prod=$2 default_repo=$3
  shift 3

  if [ "${1:-}" = "--agent" ]; then
    local task=${2:-}
    local repo=${3:-$default_repo}
    [ -n "$task" ] || { echo "Usage: $prod --agent <task-slug> [<repo>]"; return 1; }
    [ -n "$repo" ] || { echo "Usage: $prod --agent <task-slug> <repo>"; return 1; }
    agent-workspace launch "$co" "$prod" "$repo" "$task"
    return $?
  fi

  case $# in
    0)
      if [ -n "$default_repo" ]; then
        launch "$co" "$prod" "$default_repo"
      else
        launch "$co" "$prod"
      fi
      ;;
    1)
      launch "$co" "$prod" "$1"
      ;;
    *)
      echo "Usage: $prod [<repo>] | $prod --agent <task-slug> [<repo>]"
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
