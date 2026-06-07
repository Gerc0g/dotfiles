#!/usr/bin/env bash
# Codex CLI SessionStart hook — auto-loads recent wiki context for current repo.
#
# Installation:
#   1. Copy to ~/.codex-new/hooks/SessionStart.sh
#   2. Reference in ~/.codex-new/config.toml:
#        [hooks]
#        SessionStart = [
#          { matcher = "startup|resume", hooks = [{ type = "command", command = "/Users/_gerc0g/.codex-new/hooks/SessionStart.sh" }] },
#        ]
#
# Behavior:
#   - If cwd is inside ~/Desktop/Prokectfiles/<co>/<prod>/<repo>/
#     AND docs/knowledge/hot.md exists,
#     emits Codex hook JSON with additionalContext.
#   - Otherwise: silently exit (no noise).

# Session hooks must never break Codex startup. Keep this script best-effort.

CWD="${PWD}"
PROJECTS_ROOT="$HOME/Desktop/Prokectfiles"
WIKIPEDIK_ROOT="$HOME/Desktop/WikiPedik"

# WikiPedik vault sessions have their own lightweight checkpoint. This is not a
# project-memory hot cache; it is a session discipline reminder for Obsidian work.
case "$CWD" in
  "$WIKIPEDIK_ROOT"|"$WIKIPEDIK_ROOT"/*)
    if ! command -v python3 >/dev/null 2>&1; then
      exit 0
    fi

    WIKIPEDIK_ROOT="$WIKIPEDIK_ROOT" CWD="$CWD" python3 - <<'PY' || exit 0
import json
import os

root = os.environ.get("WIKIPEDIK_ROOT", "")
cwd = os.environ.get("CWD", "")

scope = "root"
if cwd.startswith(os.path.join(root, "dev")):
    scope = "dev"
elif cwd.startswith(os.path.join(root, "research")):
    scope = "research"
elif cwd.startswith(os.path.join(root, "Personal Brand")):
    scope = "Personal Brand"

context = f"""# WikiPedik session checkpoint

Current WikiPedik scope: {scope}

- Conversation and vault-writing default: Russian.
- Code identifiers, commands, URLs, and stable product names stay as written.
- Personal Brand content is Russian by default. Translate or write English only on explicit user request.
- English Personal Brand output should preserve the Russian source and usually be a separate copy with filename suffix ` - EN.md`.
- Before the final answer, if this session changed the vault, remind the user to commit and push the vault state to GitHub.
- Current Git boundary: `{root}` is the vault Git repository. Project memory lives in `{root}/dev`, research in `{root}/research`, and Personal Brand in `{root}/Personal Brand`.
- Do not run `git commit` or `git push` unless the user explicitly asks for commit+push in the current session or a configured finish command does it.
- Commit messages: English Conventional Commit type/scope, Russian description/body, for example `docs(brand): обновить трекер личного бренда`.
"""

print(json.dumps({
    "hookSpecificOutput": {
        "hookEventName": "SessionStart",
        "additionalContext": context,
    }
}, ensure_ascii=False))
PY
    exit 0
    ;;
esac

# Are we inside a project repo?
case "$CWD" in
  "$PROJECTS_ROOT"/*)
    # Inside a project tree, continue
    ;;
  *)
    # Not in a project repo — nothing to do
    exit 0
    ;;
esac

# Find repo root (nearest .git ancestor)
REPO_ROOT="$(git -C "$CWD" rev-parse --show-toplevel 2>/dev/null || true)"
if [ -z "$REPO_ROOT" ]; then
  exit 0
fi

HOT_FILE="$REPO_ROOT/docs/knowledge/hot.md"

# Hot cache might not exist yet — that's fine
if [ ! -f "$HOT_FILE" ]; then
  exit 0
fi

# Skip if hot.md is empty
if [ ! -s "$HOT_FILE" ]; then
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
