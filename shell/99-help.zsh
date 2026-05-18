help() {
  local cmd="$1"
  local doc=~/dotfiles/docs/COMMANDS.md
  [ ! -f "$doc" ] && { echo "COMMANDS.md not found"; return 1; }
  
  if [ -z "$cmd" ]; then
    _help_list
  else
    _help_detail "$cmd"
  fi
}

_help_list() {
  local doc=~/dotfiles/docs/COMMANDS.md
  local B=$'\033[1m' D=$'\033[2m' C=$'\033[36m' M=$'\033[35m' R=$'\033[0m'
  
  echo ""
  echo " ${B}Personal Platform${R}    ${D}? <cmd> for details${R}"
  echo ""
  
  awk -v B="$B" -v D="$D" -v C="$C" -v M="$M" -v R="$R" '
    /^### / {
      cat = substr($0, 5)
      printf " %s%s%s\n", M, cat, R
      next
    }
    /^## / { name = substr($0, 4); next }
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
  
  local section
  section=$(awk -v c="^## $cmd$" 'flag && /^## / && !($0 ~ c) { exit } $0 ~ c { flag=1 } flag { print }' "$doc")
  
  if [ -z "$section" ]; then
    echo ""
    echo " Command '$cmd' not found. Available:"
    _help_list
    return 1
  fi
  
  # Render without markdown noise — strip ##, **, `code`, ---, bullets
  echo "$section" | awk \
    -v B=$'\033[1m' \
    -v C=$'\033[36m' \
    -v Y=$'\033[33m' \
    -v D=$'\033[2m' \
    -v R=$'\033[0m' '
    function highlight(s) {
      gsub(/`/, "\x01", s)
      n = split(s, p, "\x01")
      out = p[1]
      for (k=2; k<=n; k++) {
        if (k % 2 == 0) out = out C p[k] R
        else out = out p[k]
      }
      return out
    }
    
    /^---$/ { next }
    
    # Title
    /^## / {
      sub(/^## /, "")
      printf "\n %s%s%s\n", B, $0, R
      printf " %s%s%s\n", D, gensub(/./, "─", "g", $0), R
      next
    }
    
    # **Label:** value
    /^\*\*[^*]+:\*\*/ {
      line = $0
      sub(/^\*\*/, "", line)
      i = index(line, ":**")
      label = substr(line, 1, i-1)
      value = substr(line, i+3)
      sub(/^ /, "", value)
      
      if (value == "") {
        printf "\n   %s%s:%s\n", B, label, R
      } else {
        printf "   %s%s:%s %s\n", B, label, R, highlight(value)
      }
      next
    }
    
    # Bullets
    /^- / {
      sub(/^- /, "")
      printf "       %s•%s %s\n", D, R, highlight($0)
      next
    }
    
    # Empty line
    /^$/ { print ""; next }
    
    # Plain text
    { printf "   %s\n", highlight($0) }
  '
  
  echo ""
}

alias '?'=help
