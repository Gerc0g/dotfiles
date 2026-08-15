#!/usr/bin/env bash
# Stop / SessionEnd hook: commits whatever is left and pushes.
#
# Per-step commits stay local for speed; this is where the vault leaves the
# machine. Silent on a clean vault; never breaks the session.
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" wikipedik autosync --push >/dev/null 2>&1
exit 0
