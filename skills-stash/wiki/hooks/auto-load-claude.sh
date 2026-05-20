#!/usr/bin/env bash
# Claude Code SessionStart hook — auto-loads recent wiki context for current repo.
#
# Installation:
#   1. Copy to ~/.claude-new/hooks/SessionStart.sh
#   2. Reference in ~/.claude-new/settings.json hooks section:
#        {
#          "hooks": {
#            "SessionStart": [
#              {
#                "matcher": "*",
#                "hooks": [{ "type": "command", "command": "~/.claude-new/hooks/SessionStart.sh" }]
#              }
#            ]
#          }
#        }
#
# Behavior identical to codex version — checks for repo's docs/knowledge/hot.md
# and emits hook JSON with additionalContext.

# Session hooks must never break Claude startup. Keep this script best-effort.

CWD="${PWD}"
PROJECTS_ROOT="$HOME/Desktop/Prokectfiles"

case "$CWD" in
  "$PROJECTS_ROOT"/*)
    ;;
  *)
    exit 0
    ;;
esac

REPO_ROOT="$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$REPO_ROOT" ]; then
  exit 0
fi

HOT_FILE="$REPO_ROOT/docs/knowledge/hot.md"
if [ ! -f "$HOT_FILE" ] || [ ! -s "$HOT_FILE" ]; then
  exit 0
fi

if ! command -v python3 >/dev/null 2>&1; then
  exit 0
fi

HOT_FILE="$HOT_FILE" python3 - <<'PY' || exit 0
import json
import os
import sys

path = os.environ.get("HOT_FILE", "")
try:
    with open(path, "r", encoding="utf-8") as fh:
        hot = fh.read().strip()
except Exception:
    sys.exit(0)

if not hot:
    sys.exit(0)

checkpoint = """\n\n# WikiPedik memory checkpoint\n\nBefore the final answer, decide whether this task produced a durable root cause, production gotcha, failed approach, reusable rule, informal decision, or cross-repo invariant. If yes, use the `lesson-append` skill to append one concise candidate to `docs/knowledge/_inbox.md`. If not, write nothing."""
context = "# Recent wiki context (auto-loaded from docs/knowledge/hot.md)\n\n" + hot + checkpoint
print(json.dumps({
    "hookSpecificOutput": {
        "hookEventName": "SessionStart",
        "additionalContext": context,
    }
}, ensure_ascii=False))
PY

exit 0
