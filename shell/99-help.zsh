help() {
  local cmd="$1"
  local doc=~/dotfiles/docs/COMMANDS.md
  [ ! -f "$doc" ] && { echo "COMMANDS.md not found"; return 1; }

  if [ -z "$cmd" ]; then
    echo ""
    echo "=== Personal Platform Commands ==="
    echo ""
    awk '/^## / { name = substr($0, 4); next } /^\*\*Что:\*\*/ { gsub(/\*\*Что:\*\* /, ""); printf "  %-20s %s\n", name, $0 }' "$doc"
    echo ""
    echo "Detail: ? <command>"
    echo ""
  else
    awk -v c="^## $cmd$" 'flag && /^## / && !($0 ~ c) { exit } $0 ~ c { flag=1 } flag { print }' "$doc"
  fi
}
alias '?'=help
