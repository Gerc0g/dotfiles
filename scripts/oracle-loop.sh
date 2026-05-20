#!/usr/bin/env bash
set -euo pipefail

repo_dir="${1:-$PWD}"
tool_dir="$HOME/dotfiles/tools/oracle-tui"
binary="$tool_dir/oracle-tui"

if [ -x "$binary" ]; then
  "$binary" "$repo_dir"
  exec "${SHELL:-/bin/bash}" -l
fi

if ! command -v go >/dev/null 2>&1; then
  printf 'oracle-tui binary not found and go is unavailable: %s\n' "$binary" >&2
  exit 1
fi

cd "$tool_dir"
go run . "$repo_dir"
exec "${SHELL:-/bin/bash}" -l
