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

configure_codex_masko_hooks() {
  local profile_dir=$1
  local config_path="$profile_dir/config.toml"

  mkdir -p "$profile_dir"

  PROFILE_DIR="$profile_dir" CONFIG_PATH="$config_path" MASKO_HOOK="$MASKO_HOOK" python3 - <<'PYCODE'
import os
from pathlib import Path

config = Path(os.environ["CONFIG_PATH"])
masko_hook = os.environ["MASKO_HOOK"]

if config.exists():
    text = config.read_text()
else:
    text = ""

if "[hooks]" not in text:
    text = text.rstrip() + "\n\n[hooks]\n"

# Codex native hooks provide approval/tool/prompt events. Keep SessionStart
# dedicated to WikiPedik context injection; dotfiles launch wrappers report
# lifecycle SessionStart/SessionEnd to Masko.
events = [
    "UserPromptSubmit",
    "Notification",
    "PermissionRequest",
    "PreToolUse",
    "PostToolUse",
    "Stop",
]

changed = False
insert_at = text.find("\n[hooks.state]")
if insert_at == -1:
    insert_at = len(text)

blocks = []
for event in events:
    needle = f"{event} = ["
    if needle in text:
        block_end = text.find("\n]", text.find(needle))
        block = text[text.find(needle): block_end + 2 if block_end != -1 else len(text)]
        if masko_hook in block:
            continue
        print(f"⚠ {config}: {event} already exists without Masko hook; leaving it for manual merge")
        continue

    blocks.append(
        f'{event} = [\n'
        f'  {{ matcher = "*", hooks = [{{ type = "command", command = "{masko_hook}" }}] }},\n'
        f']\n'
    )
    changed = True

if blocks:
    prefix = text[:insert_at].rstrip() + "\n\n"
    suffix = text[insert_at:].lstrip("\n")
    text = prefix + "\n".join(blocks) + ("\n" if suffix else "") + suffix

config.write_text(text)
print(("✓ updated" if changed else "✓ already configured") + f" Codex Masko hooks: {config}")
PYCODE
}

configure_codex_masko_hooks "$HOME/.codex-new"
configure_codex_masko_hooks "$HOME/.codex-setup"
configure_codex_masko_hooks "$HOME/.codex-wiki"

cat <<'EOF'

Masko integration installed.
Notes:
  - Existing running agent sessions must be restarted to emit SessionStart hooks.
  - Claude profiles use native hooks in ~/.claude-new and ~/.claude-setup.
  - Codex profiles use native hooks for prompt/tool/permission events.
  - Codex sessions launched through dotfiles `launch`/product shortcuts are reported by scripts/masko-agent-wrap.sh for lifecycle.
  - If Masko was restarted while agents were already running: bash ~/dotfiles/scripts/masko-resync-active.sh
EOF
