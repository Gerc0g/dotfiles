#!/bin/sh
# Authenticate and download an initial remote snapshot; never upload local data.
set -eu
umask 077
ob="$HOME/.local/bin/ob"
stage="$HOME/.local/state/obsidian-headless/initial-vault"
if systemctl --user is-active --quiet obsidian-sync.service; then
    echo 'Sync is already running; stop here instead of replacing its configuration.' >&2
    exit 1
fi
"$ob" login
"$ob" sync-list-remote
printf '\nRemote vault ID or exact name (the vault already used on desktop/phone): '
IFS= read -r vault
test -n "$vault" || exit 1
mkdir -p "$stage"
"$ob" sync-setup --vault "$vault" --path "$stage" --device-name hq-vps
"$ob" sync-config --path "$stage" --mode pull-only --conflict-strategy conflict \
    --file-types image,audio,video,pdf,unsupported --configs ''
"$ob" sync --path "$stage"
printf '\nRemote snapshot downloaded. The live HQ vault has not changed.\n'
printf 'Return to the task to reconcile server edits and enable continuous sync.\n'
