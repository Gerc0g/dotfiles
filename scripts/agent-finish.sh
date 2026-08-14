#!/usr/bin/env bash
# Compatibility shim: the logic lives in the core (`hq finish`).
exec "${HQ_BIN:-$HOME/dotfiles/bin/hq}" finish "$@"
