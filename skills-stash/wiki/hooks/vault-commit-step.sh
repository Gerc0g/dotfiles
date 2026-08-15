#!/usr/bin/env bash
# PostToolUse hook: commits the vault after every step the agent takes.
#
# Aider-style granularity — one edit, one commit — so the history shows what
# the agent actually did and any step can be reverted on its own. Local only:
# pushing on every tool call would make the session feel the network.
# Silent on a clean vault; never breaks the session.
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" wikipedik autosync >/dev/null 2>&1
exit 0
