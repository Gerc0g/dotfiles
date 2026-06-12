#!/usr/bin/env bash
set -euo pipefail

tool_dir="$HOME/dotfiles/tools/wiki-tui"
binary="$tool_dir/wiki-tui"

if [ -x "$binary" ]; then
  "$binary"
  exec "${SHELL:-/bin/bash}" -l
fi

if ! command -v go >/dev/null 2>&1; then
  printf 'wiki-tui binary not found and go is unavailable: %s\n' "$binary" >&2
  exit 1
fi

cd "$tool_dir"
go run .
exec "${SHELL:-/bin/bash}" -l
