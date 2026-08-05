# Project shortcuts.
#
# Every shortcut does the same thing: pick a repo, create an isolated worktree
# for the task, and move the shell there. The tmux layout that used to follow
# is gone — work continues in the editor.
#
# Shortcuts are generated from a table rather than hand-written one by one, so
# adding a product is a line, not a function. The listing itself comes from
# `hq ls`, which is the single implementation of the company/product/repo scan.

# alias : company : product : default repo (empty = ask)
_PROJECT_SHORTCUTS=(
  "infra:neurodesk:infra:infra"
  "legacy:neurodesk:legacy::"
  "agents:neurodesk:agents::"
  "saas:neurodesk:saas::"
  "cerebro:neurodesk:cerebro:cerebro"
  "aetheria:chimera:aetheria::"
  "homeless:chimera:homeless::"
  "healler:chimera:homeless:Healler"
  "neuroslop4ik:chimera:neuroslop4ik:NeuroSlop4ik"
  "comonline:chimera:comonline:ComaOnline-Sources"
  "pizduk:chimera:pizduk:Pizduk"
  "clients:SupportOps-Core:clients:adminka"
)

_ws_products() {
  hq ls "$1" --flat 2>/dev/null | cut -d/ -f2 | sort -u
}

_ws_repos() {
  hq ls "$1" "$2" --flat 2>/dev/null | cut -d/ -f3
}

_ws_pick() {
  local prompt="$1"; shift
  local opts=("$@")
  local n=${#opts[@]}

  [ $n -eq 0 ] && return 1
  [ $n -eq 1 ] && { echo "${opts[1]}"; return 0; }

  print -P "%F{cyan}$prompt%f" >&2
  local i=1
  for o in "${opts[@]}"; do
    print -P "  %F{244}$i)%f  $o" >&2
    i=$((i + 1))
  done
  print -P "%F{244}Выбор 1-$n (Enter — отмена):%f " >&2

  local choice
  read -r choice
  [ -z "$choice" ] && return 1
  [ "$choice" -ge 1 ] 2>/dev/null && [ "$choice" -le $n ] || return 1
  echo "${opts[$choice]}"
}

# _project_open <company> <product> <default-repo> [<repo>|<task>] [<task>]
_project_open() {
  local co=$1 prod=$2 default_repo=$3
  shift 3

  local repo task
  if [ -n "$default_repo" ]; then
    repo=$default_repo
    task=${1:-work}
  else
    repo=${1:-}
    task=${2:-work}
    if [ -z "$repo" ]; then
      local repos=("${(@f)$(_ws_repos "$co" "$prod")}")
      [ ${#repos[@]} -eq 0 ] && { echo "⚠ нет репозиториев в $co/$prod" >&2; return 1; }
      repo=$(_ws_pick "Репозитории $co/$prod:" "${repos[@]}") || return 1
    fi
  fi

  agent-workspace start "$co" "$prod" "$repo" "$task"
}

# Generate one function per shortcut from the table above.
for _shortcut in "${_PROJECT_SHORTCUTS[@]}"; do
  _alias=${_shortcut%%:*}
  _rest=${_shortcut#*:}
  _co=${_rest%%:*}
  _rest=${_rest#*:}
  _prod=${_rest%%:*}
  _repo=${_rest#*:}
  _repo=${_repo%:}
  functions[$_alias]="_project_open ${(q)_co} ${(q)_prod} ${(q)_repo} \"\$@\""
done
unset _shortcut _alias _rest _co _prod _repo

# Company-level entry: <company> → pick product → pick repo. Registered for
# every company that has a .company-config, so no per-company code.
_company_open() {
  local co=$1; shift
  local prod=${1:-} repo=${2:-}

  if [ -z "$prod" ]; then
    local products=("${(@f)$(_ws_products "$co")}")
    [ ${#products[@]} -eq 0 ] && { echo "⚠ нет продуктов в $co" >&2; return 1; }
    prod=$(_ws_pick "Продукты $co:" "${products[@]}") || return 1
  fi

  _project_open "$co" "$prod" "" "$repo"
}

for _co_dir in "${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}"/*/.company-config(N); do
  _co_name=${_co_dir:h:t}
  functions[$_co_name]="_company_open ${(q)_co_name} \"\$@\""
done
unset _co_dir _co_name

# `wiki` is both a product shortcut and the memory dispatcher.
wiki() {
  case "${1:-}" in
    sync)       shift; wiki-sync "$@" ;;
    status)     shift; wiki-status "$@" ;;
    synthesize) shift; wiki-synthesize "$@" ;;
    bootstrap)  shift; wiki-bootstrap-product "$@" ;;
    autocommit) shift; wiki-autocommit "$@" ;;
    rules-sync) shift; wiki-rules-sync "$@" ;;
    *)          _project_open neurodesk wiki nrdsk_wiki "$@" ;;
  esac
}

# The platform repo is edited live: shell and configs load from ~/dotfiles, so
# an isolated worktree elsewhere would simply not take effect.
dotfiles() {
  cd "$HOME/dotfiles" || return 1
  print -P "%F{cyan}$HOME/dotfiles%f"
  _workspace_open_editor "$HOME/dotfiles"
}
alias dots='dotfiles'
