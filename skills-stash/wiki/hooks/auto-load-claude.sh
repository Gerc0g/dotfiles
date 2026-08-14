#!/usr/bin/env bash
# Claude Code SessionStart hook. The logic lives in the core
# (`hq hook session-start`); this shim only guarantees the contract:
# session hooks must never break agent startup, so it always exits 0.
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" hook session-start --agent claude 2>/dev/null
exit 0
