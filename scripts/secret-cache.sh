#!/usr/bin/env bash
# Compatibility shim: the logic lives in the core (`hq secret cache`).
#
# This path is frozen into the .envrc files of every repo and product, so the
# file must stay callable forever. It only redirects.
HQ="${HQ_BIN:-$HOME/dotfiles/bin/hq}"
if [ ! -x "$HQ" ]; then
  HQ=$(command -v hq) || { echo "secret-cache: hq binary not found (build with: make -C ~/dotfiles build)" >&2; exit 127; }
fi
exec "$HQ" secret cache "$@"
