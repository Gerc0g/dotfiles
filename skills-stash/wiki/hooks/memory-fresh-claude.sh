#!/usr/bin/env bash
# claude UserPromptSubmit hook. The logic lives in the core
# (`hq hook prompt-submit`); this shim only guarantees the contract:
# prompt hooks must never break a message, so it always exits 0.
#
# Keep this file frozen — codex trusts a hook by hashing it, and any edit
# silently revokes that trust.
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" hook prompt-submit --agent claude 2>/dev/null
exit 0
