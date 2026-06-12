#!/bin/bash
# Bootstrap dotfiles on a fresh Mac
# Usage: ~/dotfiles/bootstrap.sh

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

# === 2. brew bundle (ставит всё из Brewfile) ===
echo ""
echo "→ Installing packages from Brewfile..."
brew bundle --file=Brewfile

# === 3. Codex CLI (fallback на npm) ===
if ! command -v codex >/dev/null 2>&1; then
  echo ""
  echo "→ Installing Codex CLI via npm..."
  npm install -g @openai/codex 2>/dev/null || echo "⚠ Codex install failed — install manually"
fi

# === 4. Claude Code CLI (fallback на curl) ===
if ! command -v claude >/dev/null 2>&1; then
  echo ""
  echo "→ Installing Claude Code CLI..."
  curl -fsSL https://claude.ai/install.sh | bash 2>/dev/null || echo "⚠ Claude install failed — install manually"
fi

# === 5. Symlink configs ===
echo ""
echo "→ Linking config files..."
mkdir -p ~/.config/ghostty
ln -sfn ~/dotfiles/ghostty/config ~/.config/ghostty/config
ln -sfn ~/dotfiles/tmux/tmux.conf ~/.tmux.conf
echo "✓ ghostty + tmux configs symlinked"

# === 6. Create agent profile directories ===
echo ""
echo "→ Creating agent profile dirs..."
mkdir -p ~/.codex-new ~/.codex-setup ~/.codex-wiki ~/.claude-new ~/.claude-setup
echo "✓ ~/.codex-new, ~/.codex-setup, ~/.codex-wiki, ~/.claude-new, ~/.claude-setup"

# === 7. Agent baseline profiles ===
echo ""
echo "→ Linking agent baseline profiles..."
link_profile() {
  local src=$1
  local dst=$2

  if [ -e "$dst" ] && [ ! -L "$dst" ] && ! cmp -s "$src" "$dst"; then
    local backup="${dst}.bak.$(date +%Y%m%d%H%M%S)"
    cp "$dst" "$backup"
    echo "  backed up $dst -> $backup"
  fi

  ln -sfn "$src" "$dst"
}

link_profile "$HOME/dotfiles/agent-profiles/BASELINE.md" "$HOME/.codex-new/AGENTS.md"
link_profile "$HOME/dotfiles/agent-profiles/BASELINE.md" "$HOME/.codex-setup/AGENTS.md"
link_profile "$HOME/dotfiles/agent-profiles/BASELINE.md" "$HOME/.claude-new/CLAUDE.md"
link_profile "$HOME/dotfiles/agent-profiles/BASELINE.md" "$HOME/.claude-setup/CLAUDE.md"
echo "✓ Codex/Claude baseline profiles linked"

# === 8. WikiPedik runtime ===
echo ""
echo "→ Installing WikiPedik runtime..."
bash ~/dotfiles/skills-stash/wiki/scripts/install-wiki-runtime.sh || {
  echo "⚠ WikiPedik runtime install failed — run manually:"
  echo "  bash ~/dotfiles/skills-stash/wiki/scripts/install-wiki-runtime.sh"
}

# === 9. Agent monitoring ===
echo ""
echo "→ Installing agent monitoring integrations..."
bash ~/dotfiles/scripts/install-agent-monitoring.sh || {
  echo "⚠ Agent monitoring install failed — run manually:"
  echo "  bash ~/dotfiles/scripts/install-agent-monitoring.sh"
}

# === 10. Agent skills ===
echo ""
echo "→ Installing agent skills..."
bash ~/dotfiles/scripts/agent-skill.sh install
bash ~/dotfiles/scripts/agent-skill.sh doctor

# === 11. Claude LSP plugins ===
echo ""
echo "→ Configuring Claude LSP plugins..."
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

configure_claude_profile "$HOME/.claude-new"
configure_claude_profile "$HOME/.claude-setup"
install_claude_lsp_plugins "$HOME/.claude-new"
install_claude_lsp_plugins "$HOME/.claude-setup"
echo "✓ Claude LSP plugins configured (pyright, vtsls, yaml-language-server)"

# === 12. ~/work directory ===
mkdir -p ~/Desktop/Prokectfiles
echo "✓ ~/Desktop/Prokectfiles/ ready"

# === 12b. Daily transcript secret-scrubber (launchd) ===
echo ""
echo "→ Installing transcript-scrub launchd job..."
mkdir -p ~/Library/LaunchAgents ~/Library/Logs
cp ~/dotfiles/scripts/launchd/com.gerc0g.transcript-scrub.plist ~/Library/LaunchAgents/
launchctl bootout "gui/$(id -u)/com.gerc0g.transcript-scrub" 2>/dev/null || true
if launchctl bootstrap "gui/$(id -u)" ~/Library/LaunchAgents/com.gerc0g.transcript-scrub.plist 2>/dev/null; then
  echo "✓ transcript-scrub scheduled daily 03:30"
else
  echo "⚠ transcript-scrub job not loaded — run manually:"
  echo "  launchctl bootstrap gui/\$(id -u) ~/Library/LaunchAgents/com.gerc0g.transcript-scrub.plist"
fi

# === 13. Подключить loader в .zshrc ===
if ! grep -q "dotfiles/shell/_loader.zsh" ~/.zshrc 2>/dev/null; then
  echo "" >> ~/.zshrc
  echo "# === Personal Platform ===" >> ~/.zshrc
  echo "[ -f ~/dotfiles/shell/_loader.zsh ] && source ~/dotfiles/shell/_loader.zsh" >> ~/.zshrc
  echo "✓ Loader added to ~/.zshrc"
else
  echo "✓ Loader already in ~/.zshrc"
fi

# === 14. Проверка OrbStack ===
if command -v docker >/dev/null 2>&1; then
  if docker ps >/dev/null 2>&1; then
    echo "✓ Docker runtime (OrbStack) работает"
  else
    echo "⚠ Docker установлен но не запущен — открой OrbStack через Spotlight"
  fi
fi

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
     CODEX_HOME=~/.codex-new codex login
     CODEX_HOME=~/.codex-setup codex login
     CODEX_HOME=~/.codex-wiki codex login
     CLAUDE_CONFIG_DIR=~/.claude-new claude login
     CLAUDE_CONFIG_DIR=~/.claude-setup claude login

  4. 1Password CLI:
     • Открой 1Password app → Settings → Developer → Integrate with CLI ON
     • В терминале: eval "$(op signin)"
     • Тест: op vault list

  5. SSH keys: будут генерироваться автоматически при new-company
     (один ключ на компанию, не один глобальный)

  6. Тест platform:
     ?                  # список всех команд
     wikipedik          # должен открыть tmux с двумя codex

  7. Запустить shared dev stack (опционально):
     dev-stack up       # Postgres, Redis, Grafana, Prometheus
     # ~500 MB RAM. dev-stack down когда не нужен.

  8. Открыть Obsidian:
     open -a Obsidian
     File → Open Vault → ~/Desktop/WikiPedik/dev
     (потом research vault через Cmd+,)

==============================================
MANUAL
