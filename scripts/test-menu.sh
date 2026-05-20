#!/usr/bin/env bash
set -euo pipefail

repo_dir="${1:-$PWD}"
suggested="${2:-}"
tool_dir="$HOME/dotfiles/tools/test-tui"
binary="$tool_dir/test-tui"

if [ -x "$binary" ]; then
  "$binary" "$repo_dir" "$suggested"
  exec "${SHELL:-/bin/bash}" -l
fi

if ! command -v go >/dev/null 2>&1; then
  printf 'test-tui binary not found and go is unavailable: %s\n' "$binary" >&2
  exit 1
fi

cd "$tool_dir"
go run . "$repo_dir" "$suggested"
exec "${SHELL:-/bin/bash}" -l
