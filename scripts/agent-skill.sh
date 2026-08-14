#!/usr/bin/env bash
# Compatibility shim: the logic lives in the core (`hq skill`).
exec "${HQ_BIN:-$HOME/dotfiles/bin/hq}" skill "$@"
