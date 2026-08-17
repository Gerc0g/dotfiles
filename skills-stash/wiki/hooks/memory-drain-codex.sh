#!/usr/bin/env bash
# Codex CLI SessionEnd hook. The logic lives in the core
# (`hq hook session-end`); this shim only guarantees the contract:
# session hooks must never break agent shutdown, so it always exits 0.
#
# Keep this file frozen. Codex trusts a hook by hashing it, and any edit
# silently revokes that trust — which is exactly how the memory hooks died
# unnoticed for three days after the Go migration rewrote them.
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" hook session-end --agent codex 2>/dev/null
exit 0
