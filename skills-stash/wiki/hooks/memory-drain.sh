#!/usr/bin/env bash
# SessionEnd hook: hands the just-finished repo's inbox to the curator.
#
# End of session is the honest moment: the lessons of this work are already
# captured, and nothing is in flight. The drain detaches immediately — a hook
# must not hold the session — and re-checks its guards in the child, so most
# firings cost nothing at all.
#
# Guards (threshold + interval) live in `hq wiki drain`; every decision, run
# or skip, lands in ~/.cache/wikipedik-autosync.log.
#
# The hook logs its own firing before doing anything. Without that, a silent
# log is ambiguous — a hook that never ran and a hook that ran and failed look
# identical, which is exactly how the first version hid a dead trigger for a
# whole day.
hq="${HQ_BIN:-$HOME/dotfiles/bin/hq}"
log="$HOME/.cache/wikipedik-autosync.log"

mkdir -p "$(dirname "$log")"
printf '%s hook SessionEnd: %s (%s)\n' "$(date '+%Y-%m-%d %H:%M:%S')" "$PWD" \
  "${CLAUDE_AGENT:-${CODEX_HOME:+codex}}" >>"$log"

if ! err=$("$hq" wiki drain --background --commit 2>&1 >/dev/null); then
  printf '%s hook SessionEnd: ошибка — %s\n' "$(date '+%Y-%m-%d %H:%M:%S')" \
    "${err:-неизвестно}" >>"$log"
fi

# A hook must never fail the session it is attached to.
exit 0
