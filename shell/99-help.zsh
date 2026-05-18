help() {
  local cmd="$1"
  local doc=~/dotfiles/docs/COMMANDS.md
  
  [ ! -f "$doc" ] && { echo "COMMANDS.md not found at $doc"; return 1; }
  
  if [ -z "$cmd" ]; then
    _help_list
  else
    _help_detail "$cmd"
  fi
}

_help_list() {
  local doc=~/dotfiles/docs/COMMANDS.md
  
  local B=$'\e[1m'         # bold
  local D=$'\e[2m'         # dim
  local C=$'\e[36m'        # cyan (command names)
  local M=$'\e[35m'        # magenta (categories)
  local R=$'\e[0m'         # reset
  
  echo ""
  echo " ${B}Personal Platform${R}    ${D}? <cmd> for details${R}"
  echo ""
  
  awk -v B="$B" -v D="$D" -v C="$C" -v M="$M" -v R="$R" '
    /^### / {
      cat = substr($0, 5)
      printf " %s%s%s\n", M, cat, R
      next
    }
    /^## / {
      name = substr($0, 4)
      next
    }
    /^\*\*Что:\*\*/ {
      gsub(/\*\*Что:\*\* */, "")
      gsub(/\*\*/, "")
      printf "   %s%-15s%s %s\n", C, name, R, $0
    }
  ' "$doc"
  
  echo ""
}

_help_detail() {
  local cmd="$1"
  local doc=~/dotfiles/docs/COMMANDS.md
  
  local section=$(awk -v c="^## $cmd$" 'flag && /^## / && !($0 ~ c) { exit } $0 ~ c { flag=1 } flag { print }' "$doc")
  
  if [ -z "$section" ]; then
    echo "Command '$cmd' not found. Available:"
    _help_list
    return 1
  fi
  
  if command -v bat >/dev/null 2>&1; then
    echo "$section" | bat --language=md --style=plain --paging=never
  else
    echo "$section"
  fi
}

alias '?'=help
