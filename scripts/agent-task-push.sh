#!/usr/bin/env bash
# Compatibility shim: the logic lives in the core (`hq finish --no-review`).
exec "${HQ_BIN:-$HOME/dotfiles/bin/hq}" finish --no-review "$@"
