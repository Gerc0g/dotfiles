#!/usr/bin/env bash
# Install WikiPedik local runtime: vault clone/skeleton, skills, and hooks.

set -euo pipefail

DOTFILES="${DOTFILES:-$HOME/dotfiles}"
WIKI_ROOT="${WIKI_ROOT:-$HOME/Desktop/WikiPedik}"
WIKI_DEV="$WIKI_ROOT/dev"
WIKI_RESEARCH="$WIKI_ROOT/research"
WIKI_BRAND="$WIKI_ROOT/Personal Brand"
WIKIPEDIK_REMOTE="${WIKIPEDIK_REMOTE:-git@github.com:JustChimera/WikiPedik.git}"
WIKIPEDIK_GIT_NAME="${WIKIPEDIK_GIT_NAME:-}"
WIKIPEDIK_GIT_EMAIL="${WIKIPEDIK_GIT_EMAIL:-}"

echo "=== WikiPedik runtime install ==="

mkdir -p "$WIKI_ROOT"

if [ -d "$WIKI_ROOT/.git" ]; then
  echo "✓ root vault git repo exists: $WIKI_ROOT"
elif [ -d "$WIKI_DEV/.git" ]; then
  echo "⚠ legacy dev-level git repo exists: $WIKI_DEV/.git"
  echo "  Move it to $WIKI_ROOT/.git before running full-vault sync."
elif [ ! -e "$WIKI_ROOT/.git" ] && [ -z "$(find "$WIKI_ROOT" -mindepth 1 -maxdepth 1 ! -name dev 2>/dev/null)" ] && { [ ! -e "$WIKI_DEV" ] || [ -z "$(find "$WIKI_DEV" -mindepth 1 -maxdepth 1 2>/dev/null)" ]; }; then
  echo "→ Cloning root vault: $WIKIPEDIK_REMOTE"
  git clone "$WIKIPEDIK_REMOTE" "$WIKI_ROOT"
else
  echo "⚠ root vault exists but is not a git repo: $WIKI_ROOT"
  echo "  Leaving it untouched. Initialize or move it manually."
fi

if [ -d "$WIKI_ROOT/.git" ]; then
  git -C "$WIKI_ROOT" remote set-url origin "$WIKIPEDIK_REMOTE" 2>/dev/null || true
  [ -n "$WIKIPEDIK_GIT_NAME" ] && git -C "$WIKI_ROOT" config user.name "$WIKIPEDIK_GIT_NAME"
  [ -n "$WIKIPEDIK_GIT_EMAIL" ] && git -C "$WIKI_ROOT" config user.email "$WIKIPEDIK_GIT_EMAIL"
  echo "✓ root vault git remote: $(git -C "$WIKI_ROOT" remote get-url origin 2>/dev/null || echo none)"
  if [ -n "$WIKIPEDIK_GIT_EMAIL" ]; then
    echo "✓ root vault git email: $WIKIPEDIK_GIT_EMAIL"
  fi
fi

mkdir -p "$WIKI_RESEARCH/00-inbox" "$WIKI_RESEARCH/10-wiki"
touch "$WIKI_RESEARCH/10-wiki/index.md" "$WIKI_RESEARCH/10-wiki/log.md"
echo "✓ research vault skeleton: $WIKI_RESEARCH"

mkdir -p "$WIKI_BRAND"
echo "✓ personal brand vault path: $WIKI_BRAND"

mkdir -p "$HOME/.codex/skills" "$HOME/.codex/hooks"
mkdir -p "$HOME/.codex/skills" "$HOME/.codex/hooks"
mkdir -p "$HOME/.claude/skills" "$HOME/.claude/hooks"

ln -sfn "$DOTFILES/skills-stash/wiki/worker/wiki-context-pack" "$HOME/.codex/skills/wiki-context-pack"
ln -sfn "$DOTFILES/skills-stash/wiki/worker/lesson-append" "$HOME/.codex/skills/lesson-append"
ln -sfn "$DOTFILES/skills-stash/wiki/worker/wiki-context-pack" "$HOME/.claude/skills/wiki-context-pack"
ln -sfn "$DOTFILES/skills-stash/wiki/worker/lesson-append" "$HOME/.claude/skills/lesson-append"

for skill in source-ingest agent-history-ingest inbox-drain wiki-synthesize wiki-lint wiki-status autoresearch; do
  ln -sfn "$DOTFILES/skills-stash/wiki/curator/$skill" "$HOME/.codex/skills/$skill"
done

ln -sfn "$DOTFILES/skills-stash/wiki/hooks/auto-load-codex.sh" "$HOME/.codex/hooks/SessionStart.sh"
ln -sfn "$DOTFILES/skills-stash/wiki/hooks/auto-load-codex.sh" "$HOME/.codex/hooks/SessionStart.sh"
ln -sfn "$DOTFILES/skills-stash/wiki/hooks/auto-load-claude.sh" "$HOME/.claude/hooks/SessionStart.sh"
echo "✓ skills and hooks symlinked"

ensure_codex_hook() {
  local cfg=$1
  local hook=$2

  if [ ! -f "$cfg" ]; then
    cat > "$cfg" <<EOF
[hooks]
SessionStart = [
  { matcher = "startup|resume", hooks = [{ type = "command", command = "$hook" }] },
]
EOF
    echo "✓ created $cfg"
    return 0
  fi

  if grep -qF "$hook" "$cfg"; then
    echo "✓ codex hook already configured: $cfg"
    return 0
  fi

  if grep -q '^\[hooks\]' "$cfg"; then
    echo "✗ $cfg already has [hooks] but not WikiPedik hook; add manually:"
    echo "  SessionStart = [{ matcher = \"startup|resume\", hooks = [{ type = \"command\", command = \"$hook\" }] }]"
    return 1
  fi

  cat >> "$cfg" <<EOF

[hooks]
SessionStart = [
  { matcher = "startup|resume", hooks = [{ type = "command", command = "$hook" }] },
]
EOF
  echo "✓ appended codex hook: $cfg"
}

ensure_codex_project_trust() {
  local cfg=$1
  local project_path=$2

  [ -f "$cfg" ] || return 0

  if grep -qF "[projects.\"$project_path\"]" "$cfg"; then
    echo "✓ codex project already trusted: $project_path"
    return 0
  fi

  cat >> "$cfg" <<EOF

[projects."$project_path"]
trust_level = "trusted"
EOF
  echo "✓ trusted codex project: $project_path"
}

ensure_claude_hook() {
  local cfg="$HOME/.claude/settings.json"
  local hook="$HOME/.claude/hooks/SessionStart.sh"

  CFG="$cfg" HOOK="$hook" python3 - <<'PY'
import json
import os
from pathlib import Path

cfg = Path(os.environ["CFG"])
hook = os.environ["HOOK"]

if cfg.exists():
    try:
        data = json.loads(cfg.read_text())
    except Exception:
        print(f"⚠ invalid JSON in {cfg}; leaving untouched")
        raise SystemExit(0)
else:
    data = {}

hooks = data.setdefault("hooks", {})
items = hooks.setdefault("SessionStart", [])

for item in items:
    for h in item.get("hooks", []):
        if h.get("command") == hook:
            print(f"✓ claude hook already configured: {cfg}")
            cfg.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")
            raise SystemExit(0)

items.append({
    "matcher": "*",
    "hooks": [{"type": "command", "command": hook}],
})
cfg.parent.mkdir(parents=True, exist_ok=True)
cfg.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")
print(f"✓ configured claude hook: {cfg}")
PY
}

ensure_codex_hook "$HOME/.codex/config.toml" "$HOME/.codex/hooks/SessionStart.sh"
ensure_codex_hook "$HOME/.codex/config.toml" "$HOME/.codex/hooks/SessionStart.sh"
ensure_codex_project_trust "$HOME/.codex/config.toml" "$WIKI_DEV"
ensure_codex_project_trust "$HOME/.codex/config.toml" "$WIKI_RESEARCH"
ensure_codex_project_trust "$HOME/.codex/config.toml" "$WIKI_BRAND"
ensure_codex_project_trust "$HOME/.codex/config.toml" "$WIKI_ROOT"
ensure_claude_hook

echo ""
echo "Verify:"
echo "  CODEX_HOME=~/.codex codex debug prompt-input smoke | rg 'wiki-context-pack|lesson-append'"
echo "  CODEX_HOME=~/.codex codex debug prompt-input smoke | rg 'inbox-drain|wiki-status'"
echo "  cd ~/Desktop/WikiPedik && git status --short --branch"
