# WikiPedik vault entry point.
#
# This used to open a tmux session with a codex pane per vault zone. The panes
# are gone with the rest of the tmux layer: three agents waiting in three
# directories is not a workflow, it is three agents waiting.

wikipedik() {
  local root="${WIKIPEDIK_ROOT:-$HOME/Desktop/WikiPedik}"
  local zone=${1:-dev}

  case "$zone" in
    dev|research|brand) ;;
    *) echo "Usage: wikipedik [dev|research|brand]" >&2; return 1 ;;
  esac

  [ "$zone" = "brand" ] && zone="Personal Brand"

  local target="$root/$zone"
  [ -d "$target" ] || { echo "⚠ нет зоны вольта: $target" >&2; return 1; }

  cd "$target" || return 1
  print -P "%F{cyan}$target%f"

  local dirty
  dirty=$(git -C "$root" status --porcelain 2>/dev/null | wc -l | tr -d ' ')
  [ "${dirty:-0}" -gt 0 ] && print -P "%F{244}незакоммиченных файлов: $dirty%f"
}
