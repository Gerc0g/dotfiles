#!/usr/bin/env bash
# Claude Code SessionEnd hook. The logic lives in the core
# (`hq hook session-end`); this shim only guarantees the contract:
# session hooks must never break agent shutdown, so it always exits 0.
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" hook session-end --agent claude 2>/dev/null
exit 0
