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
  "$PROJECTS_ROOT"/*) ;;
  "$HOME/dotfiles"|"$HOME/dotfiles"/*) ;;  # platform repo has its own memory (_platform scope)
  *)
    exit 0
    ;;
esac

REPO_ROOT="$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$REPO_ROOT" ]; then
  exit 0
fi

if ! command -v python3 >/dev/null 2>&1; then
  exit 0
fi

HOT_FILE="$REPO_ROOT/docs/knowledge/hot.md"

# Memory not attached (repo never bootstrapped, or a worktree without symlinks).
# Do not stay silent: tell the agent so the user learns about it in-chat instead
# of the memory loop dying invisibly for weeks.
if [ ! -f "$HOT_FILE" ] || [ ! -s "$HOT_FILE" ]; then
  REL="${REPO_ROOT#$PROJECTS_ROOT/}"
  case "$REL" in
    .worktrees/*) REL="${REL#.worktrees/}" ;;
  esac
  SCOPE="$(printf '%s' "$REL" | awk -F/ '{print $1, $2, $3}')"
  SCOPE="$SCOPE" python3 - <<'PY' || exit 0
import json
import os

scope = os.environ.get("SCOPE", "").strip()
context = (
    "# WikiPedik memory warning\n\n"
    "This repo has no WikiPedik project memory attached: docs/knowledge/hot.md is missing or empty.\n"
    "Lessons will not be captured and no prior context is available.\n"
    f"Early in this session, tell the user that project memory is disconnected and suggest running: `wiki bootstrap {scope}`."
)
print(json.dumps({
    "hookSpecificOutput": {
        "hookEventName": "SessionStart",
        "additionalContext": context,
    }
}, ensure_ascii=False))
PY
  exit 0
fi

INBOX_CANDIDATES="$(grep -c '^Status: candidate$' "$REPO_ROOT/docs/knowledge/_inbox.md" 2>/dev/null || echo 0)"

HOT_FILE="$HOT_FILE" INBOX_CANDIDATES="$INBOX_CANDIDATES" python3 - <<'PY' || exit 0
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

checkpoint = """\n\n# WikiPedik memory checkpoint\n\n1. The hot context above is an INDEX, not the whole memory. Read `docs/knowledge/<page>.md` (lessons, gotchas, debugging-stories, open-questions) on demand when the task touches those topics; for nontrivial debug/design/migration work start with the `wiki-context-pack` skill.\n2. Verify before apply: when acting on a remembered lesson that cites code locations, check the cited code first — if the code has changed and contradicts the lesson, capture a corrected version via `lesson-append` instead of applying the stale one.\n3. Before the final answer, decide whether this task produced a durable root cause, production gotcha, failed approach, reusable rule, informal decision, or cross-repo invariant. If yes, use the `lesson-append` skill to append one concise candidate (with citations) to `docs/knowledge/_inbox.md`. If not, write nothing."""

status = ""
cand = os.environ.get("INBOX_CANDIDATES", "0").strip()
if cand.isdigit() and int(cand) >= 5:
    status = f"\n\nInbox status: {cand} candidate lessons are pending in docs/knowledge/_inbox.md. Mention to the user that `wiki sync` is overdue."

context = "# Recent wiki context (auto-loaded from docs/knowledge/hot.md)\n\n" + hot + checkpoint + status
print(json.dumps({
    "hookSpecificOutput": {
        "hookEventName": "SessionStart",
        "additionalContext": context,
    }
}, ensure_ascii=False))
PY

exit 0
