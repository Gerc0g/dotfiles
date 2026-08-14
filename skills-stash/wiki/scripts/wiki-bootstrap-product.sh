#!/usr/bin/env bash
# Compatibility shim: the logic lives in the core (`hq wiki bootstrap`).
exec "${HQ_BIN:-$HOME/dotfiles/bin/hq}" wiki bootstrap "$@"
