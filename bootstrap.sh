#!/bin/bash
# Bootstrap dotfiles on a fresh Mac.
# Usage: ~/dotfiles/bootstrap.sh
#
# This script only does what needs the network, a package manager or a GUI:
# install toolchains and agent CLIs, then hand over to `hq setup`.
#
# Everything that is *machine state* — directories, symlinks, the loader line in
# ~/.zshrc, the scheduled scrubber — is declared in core/setup and applied by
# `hq setup`. That declaration is also what `hq doctor` checks, so the installer
# and the checker cannot drift apart. Add new state there, not here.

set -e

cd ~/dotfiles

echo "=== Personal Platform Bootstrap ==="
echo ""

# === 1. Homebrew ===
if ! command -v brew >/dev/null 2>&1; then
  echo "→ Installing Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  [[ -d /opt/homebrew/bin ]] && eval "$(/opt/homebrew/bin/brew shellenv)"
else
  echo "✓ Homebrew installed"
fi

# === 2. Packages (Brewfile pins the toolchains: go, node, tmux, ghostty, ...) ===
echo ""
echo "→ Installing packages from Brewfile..."
brew bundle --file=Brewfile

# === 3. Codex CLI (npm fallback) ===
if ! command -v codex >/dev/null 2>&1; then
  echo ""
  echo "→ Installing Codex CLI via npm..."
  npm install -g @openai/codex 2>/dev/null || echo "⚠ Codex install failed — install manually"
fi

# === 4. Claude Code CLI (curl fallback) ===
if ! command -v claude >/dev/null 2>&1; then
  echo ""
  echo "→ Installing Claude Code CLI..."
  curl -fsSL https://claude.ai/install.sh | bash 2>/dev/null || echo "⚠ Claude install failed — install manually"
fi

# === 5. Core binary ===
# Built before `hq setup`, because that is the thing which runs it.
echo ""
echo "→ Building core binary (bin/hq)..."
make build

# === 6. Machine state ===
# Directories, symlinks, ~/.zshrc loader, launchd scrubber — all declared in Go.
echo ""
echo "→ Applying machine state (hq setup)..."
./bin/hq setup

# === 7. Agent skills ===
echo ""
echo "→ Installing agent skills..."
./bin/hq skill install

# === 8. Claude plugins and profile settings ===
echo ""
echo "→ Configuring Claude profiles..."
configure_claude_profile() {
  local profile_dir=$1
  local hook_path="$profile_dir/hooks/SessionStart.sh"
  local settings_path="$profile_dir/settings.json"

  mkdir -p "$profile_dir/hooks"
  ln -sfn "$HOME/dotfiles/skills-stash/wiki/hooks/auto-load-claude.sh" "$hook_path"

  PROFILE_DIR="$profile_dir" HOOK_PATH="$hook_path" python3 - <<'PY'
import json
import os
from pathlib import Path

profile = Path(os.environ["PROFILE_DIR"])
hook_path = os.environ["HOOK_PATH"]
settings = profile / "settings.json"

if settings.exists():
    try:
        data = json.loads(settings.read_text())
    except Exception:
        data = {}
else:
    data = {}

# No "Co-Authored-By: Claude" in commits and no "Generated with Claude Code"
# footer in PR/MR bodies — platform convention is clean attribution.
data["includeCoAuthoredBy"] = False

enabled = data.setdefault("enabledPlugins", {})
enabled["pyright-lsp@claude-plugins-official"] = True
enabled["vtsls@claude-code-lsps"] = True
enabled["yaml-language-server@claude-code-lsps"] = True

marketplaces = data.setdefault("extraKnownMarketplaces", {})
marketplaces["claude-plugins-official"] = {
    "source": {"source": "github", "repo": "anthropics/claude-plugins-official"}
}
marketplaces["claude-code-lsps"] = {
    "source": {"source": "github", "repo": "boostvolt/claude-code-lsps"}
}

hooks = data.setdefault("hooks", {})
items = hooks.setdefault("SessionStart", [])
if not any(h.get("command") == hook_path for item in items for h in item.get("hooks", [])):
    items.append({
        "matcher": "*",
        "hooks": [{"type": "command", "command": hook_path}],
    })

settings.write_text(json.dumps(data, indent=2, ensure_ascii=False) + "\n")
PY
}

install_claude_lsp_plugins() {
  local profile_dir=$1

  if ! command -v claude >/dev/null 2>&1; then
    return 0
  fi

  CLAUDE_CONFIG_DIR="$profile_dir" claude plugin marketplace add anthropics/claude-plugins-official >/dev/null 2>&1 || true
  CLAUDE_CONFIG_DIR="$profile_dir" claude plugin marketplace add boostvolt/claude-code-lsps >/dev/null 2>&1 || true
  CLAUDE_CONFIG_DIR="$profile_dir" claude plugin install pyright-lsp@claude-plugins-official >/dev/null 2>&1 || true
  CLAUDE_CONFIG_DIR="$profile_dir" claude plugin install vtsls@claude-code-lsps >/dev/null 2>&1 || true
  CLAUDE_CONFIG_DIR="$profile_dir" claude plugin install yaml-language-server@claude-code-lsps >/dev/null 2>&1 || true
}

configure_claude_profile "$HOME/.claude"
install_claude_lsp_plugins "$HOME/.claude"
echo "✓ Claude profile configured (pyright, vtsls, yaml-language-server)"

# === 9. Final check ===
echo ""
echo "→ Verifying machine state..."
./bin/hq doctor || echo "⚠ Some checks failed — see above"

# === Manual steps ===
cat <<'MANUAL'

==============================================
✅ Automated setup complete!

Manual steps (cannot be automated):

  1. Reload shell:
     source ~/.zshrc

  2. Запусти OrbStack (Spotlight → OrbStack)
     Первый запуск даст permissions. После — Docker работает прозрачно.

  3. Login to agents:
     codex login
     claude login

  4. 1Password CLI:
     • Открой 1Password app → Settings → Developer → Integrate with CLI ON
     • В терминале: eval "$(op signin)"
     • Тест: op vault list

  5. SSH keys: будут генерироваться автоматически при new-company
     (один ключ на компанию, не один глобальный)

  6. Тест platform:
     hq ls              # компании / продукты / репозитории
     hq doctor          # состояние машины
     ?                  # список shell-команд

  7. Запустить shared dev stack (опционально):
     dev-stack up

  8. Открыть Obsidian:
     open -a Obsidian
     File → Open Vault → ~/Desktop/WikiPedik/dev

==============================================
MANUAL
