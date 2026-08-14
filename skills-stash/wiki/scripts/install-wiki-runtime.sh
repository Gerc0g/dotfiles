#!/usr/bin/env bash
# Compatibility shim: the WikiPedik runtime is machine state now. Everything
# this script used to do — vault clone/skeleton, curator skill links,
# SessionStart hooks, codex config.toml, claude settings.json — is declared
# in core/setup and applied (and checked) by `hq setup` / `hq doctor`.
set -euo pipefail

"${HQ_BIN:-$HOME/dotfiles/bin/hq}" setup

# Optional vault git identity, kept from the original script.
VAULT="${WIKIPEDIK_ROOT:-$HOME/Desktop/WikiPedik}"
if [ -d "$VAULT/.git" ]; then
  [ -n "${WIKIPEDIK_GIT_NAME:-}" ] && git -C "$VAULT" config user.name "$WIKIPEDIK_GIT_NAME"
  [ -n "${WIKIPEDIK_GIT_EMAIL:-}" ] && git -C "$VAULT" config user.email "$WIKIPEDIK_GIT_EMAIL"
fi
exit 0
