# Live system/agent pressure dashboard (Go TUI, built into bin/ by `make build`).
status() {
  local bin="$HOME/dotfiles/bin/status-tui"
  if [ -x "$bin" ]; then
    "$bin" "$@"
  else
    (cd "$HOME/dotfiles/tools/status-tui" && go run . "$@")
  fi
}
