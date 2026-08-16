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
"${HQ_BIN:-$HOME/dotfiles/bin/hq}" wiki drain --background --commit >/dev/null 2>&1
exit 0
