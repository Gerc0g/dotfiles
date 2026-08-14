# WikiPedik vault zones. Zone resolution lives in the core (`hq wikipedik`);
# the wrapper does what a child process cannot — cd. Entering a zone creates
# its directory lazily, so a freshly wiped zone is enterable right away.
wikipedik() {
  local dir
  dir=$(hq wikipedik path "${1:-root}") || return 1
  mkdir -p "$dir"
  cd "$dir" || return 1
  print -P "%F{cyan}$dir%f"

  local dirty
  dirty=$(git -C "${WIKIPEDIK_ROOT:-$HOME/Desktop/WikiPedik}" status --porcelain 2>/dev/null | wc -l | tr -d ' ')
  [ "${dirty:-0}" -gt 0 ] && print -P "%F{244}незакоммиченных файлов: $dirty (hq wikipedik sync)%f"
}
