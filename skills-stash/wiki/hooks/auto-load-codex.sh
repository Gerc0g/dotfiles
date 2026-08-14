#!/usr/bin/env bash
# Codex CLI SessionStart hook. The logic lives in the core
# (`hq hook session-start`); this shim only guarantees the contract:
# session hooks must never break agent startup, so it always exits 0.
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" hook session-start --agent codex 2>/dev/null
exit 0
