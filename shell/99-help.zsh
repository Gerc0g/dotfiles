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
      gsub(/`/, "")
      printf "   %s%-22s%s %s\n", C, name, R, $0
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
    echo " Command '$cmd' not found."
    return 1
  fi
  
  echo "$section" | awk \
    -v B=$'\033[1m' \
    -v C=$'\033[36m' \
    -v D=$'\033[2m' \
    -v R=$'\033[0m' '
    function highlight(s,    out, n, p, k) {
      gsub(/`/, "\x01", s)
      n = split(s, p, "\x01")
      out = p[1]
      for (k = 2; k <= n; k++) {
        if (k % 2 == 0) out = out C p[k] R
        else out = out p[k]
      }
      return out
    }
    
    /^---$/ { next }
    
    /^## / {
      sub(/^## /, "")
      title = $0
      printf "\n %s%s%s\n", B, title, R
      bar = ""
      for (i = 0; i < length(title); i++) bar = bar "─"
      printf " %s%s%s\n", D, bar, R
      next
    }
    
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
    
    /^- / {
      sub(/^- /, "")
      printf "       %s•%s %s\n", D, R, highlight($0)
      next
    }
    
    /^$/ { print ""; next }
    
    { printf "   %s\n", highlight($0) }
  '
  
  echo ""
}

alias '?'=help
