#!/usr/bin/env bash
# Compatibility shim: the logic lives in the core (`hq onboard product`).
exec "${HQ_BIN:-$HOME/dotfiles/bin/hq}" onboard product "$@"
