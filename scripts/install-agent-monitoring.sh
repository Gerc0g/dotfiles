#!/usr/bin/env bash
# Install local agent monitoring integrations for isolated dotfiles profiles.
# Integrates Masko Code with Claude hooks and dotfiles launch wrappers for Codex/Claude.

set -euo pipefail

MASKO_HOOK="${MASKO_HOOK:-$HOME/.masko-desktop/hooks/hook-sender}"
if [ ! -x "$MASKO_HOOK" ]; then
  if [ -x "$HOME/.masko-desktop/hooks/hook-sender.sh" ]; then
    MASKO_HOOK="$HOME/.masko-desktop/hooks/hook-sender.sh"
  else
    echo "⚠ Masko hook sender not found. Start Masko Code once, then rerun:"
    echo "  bash ~/dotfiles/scripts/install-agent-monitoring.sh"
    exit 0
  fi
fi

configure_claude_masko_hooks() {
  local profile_dir=$1
  local settings_path="$profile_dir/settings.json"

  mkdir -p "$profile_dir"

  PROFILE_DIR="$profile_dir" SETTINGS_PATH="$settings_path" MASKO_HOOK="$MASKO_HOOK" python3 - <<'PYCODE'
import json
import os
from pathlib import Path

settings = Path(os.environ["SETTINGS_PATH"])
masko_hook = os.environ["MASKO_HOOK"]

if settings.exists():
    try:
        data = json.loads(settings.read_text())
    except Exception as exc:
        print(f"⚠ invalid JSON in {settings}: {exc}; leaving untouched")
        raise SystemExit(0)
else:
    data = {}

hooks = data.setdefault("hooks", {})
# PermissionRequest must be blocking so Masko can return allow/deny.
events = {
    "SessionStart": True,
    "Notification": True,
    "PermissionRequest": False,
    "PreToolUse": True,
    "PostToolUse": True,
    "PostToolUseFailure": True,
    "Stop": True,
    "StopFailure": True,
    "SessionEnd": True,
    "SubagentStart": True,
    "SubagentStop": True,
    "TaskCompleted": True,
    "TeammateIdle": True,
    "UserPromptSubmit": True,
}

changed = False
for event, async_hook in events.items():
    items = hooks.setdefault(event, [])
    exists = any(
        h.get("command") == masko_hook
        for item in items
        for h in item.get("hooks", [])
    )
    if exists:
        continue
    hook = {"type": "command", "command": masko_hook}
    if async_hook:
        hook["async"] = True
    items.append({"matcher": "", "hooks": [hook]})
    changed = True

settings.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")
print(("✓ updated" if changed else "✓ already configured") + f" Masko hooks: {settings}")
PYCODE
}

configure_claude_masko_hooks "$HOME/.claude-new"
configure_claude_masko_hooks "$HOME/.claude-setup"

cat <<'EOF'

Masko integration installed.
Notes:
  - Existing running agent sessions must be restarted to emit SessionStart hooks.
  - Claude profiles use native hooks in ~/.claude-new and ~/.claude-setup.
  - Codex sessions launched through dotfiles `launch`/product shortcuts are reported by scripts/masko-agent-wrap.sh.
  - If Masko was restarted while agents were already running: bash ~/dotfiles/scripts/masko-resync-active.sh
EOF
