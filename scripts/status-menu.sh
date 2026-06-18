#!/usr/bin/env bash
set -euo pipefail

tool_dir="$HOME/dotfiles/tools/status-tui"
binary="$tool_dir/status-tui"

if [ -x "$binary" ]; then
  "$binary" "$@"
  exit $?
fi

if ! command -v go >/dev/null 2>&1; then
  printf 'status-tui binary not found and go is unavailable: %s\n' "$binary" >&2
  exit 1
fi

cd "$tool_dir"
go run . "$@"
