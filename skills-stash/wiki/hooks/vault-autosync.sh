#!/usr/bin/env bash
# Stop hook: commits the vault after the agent finishes its answer.
#
# The rule to commit lives in the zone contract, but a model skips service
# phases sooner or later — this is the mechanical duplicate that makes the
# rule hold anyway. Silent when the vault is clean; never breaks the session.
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" wikipedik autosync >/dev/null 2>&1
exit 0
