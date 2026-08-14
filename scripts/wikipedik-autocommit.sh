#!/usr/bin/env bash
# Compatibility shim: the logic lives in the core (`hq wiki autocommit`).
exec "${HQ_BIN:-$HOME/dotfiles/bin/hq}" wiki autocommit "$@"
