#!/usr/bin/env bash
# Compatibility shim: the logic lives in the core (`hq onboard company`).
exec "${HQ_BIN:-$HOME/dotfiles/bin/hq}" onboard company "$@"
