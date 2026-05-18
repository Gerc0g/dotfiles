#!/bin/bash
# Bootstrap dotfiles on a fresh Mac — install everything, link configs, prepare environment
# Usage: ~/dotfiles/bootstrap.sh

set -e

cd ~/dotfiles

echo "=== Personal Platform Bootstrap ==="
echo ""

# === 1. Homebrew ===
if ! command -v brew >/dev/null 2>&1; then
  echo "→ Installing Homebrew..."
  /bin/bash -c "$(curl -fsSL https://raw.githubusercontent.com/Homebrew/install/HEAD/install.sh)"
  # Add to PATH for current shell
  if [[ -d /opt/homebrew/bin ]]; then
    eval "$(/opt/homebrew/bin/brew shellenv)"
  fi
else
  echo "✓ Homebrew installed"
fi

# === 2. brew bundle ===
echo ""
echo "→ Installing packages from Brewfile..."
brew bundle --file=Brewfile

# === 3. Codex CLI (если не через brew) ===
if ! command -v codex >/dev/null 2>&1; then
  echo ""
  echo "→ Installing Codex CLI via npm..."
  npm install -g @openai/codex 2>/dev/null || echo "⚠ Codex install failed — install manually later"
fi

# === 4. Claude Code CLI (если не через brew) ===
if ! command -v claude >/dev/null 2>&1; then
  echo ""
  echo "→ Installing Claude Code CLI..."
  curl -fsSL https://claude.ai/install.sh | bash 2>/dev/null || echo "⚠ Claude install failed — install manually later"
fi

# === 5. Symlink configs ===
echo ""
echo "→ Linking config files..."
mkdir -p ~/.config/ghostty
ln -sfn ~/dotfiles/ghostty/config ~/.config/ghostty/config
ln -sfn ~/dotfiles/tmux/tmux.conf ~/.tmux.conf
echo "✓ ghostty config symlinked"
echo "✓ tmux config symlinked"

# === 6. Create agent profile directories ===
echo ""
echo "→ Creating agent profile dirs..."
mkdir -p ~/.codex-new ~/.codex-wiki ~/.claude-new
echo "✓ ~/.codex-new, ~/.codex-wiki, ~/.claude-new"

# === 7. WikiPedik vault skeleton (если нет) ===
if [ ! -d ~/Desktop/WikiPedik ]; then
  echo ""
  echo "→ Creating WikiPedik skeleton..."
  mkdir -p ~/Desktop/WikiPedik/dev/{00-inbox,10-wiki,20-projects}
  mkdir -p ~/Desktop/WikiPedik/research/{00-inbox,10-wiki}
  touch ~/Desktop/WikiPedik/dev/10-wiki/{index.md,log.md}
  touch ~/Desktop/WikiPedik/research/10-wiki/{index.md,log.md}
  echo "✓ WikiPedik vaults created (empty — fill via codex)"
fi

# === 8. ~/work directory ===
mkdir -p ~/work
echo "✓ ~/work/ ready"

# === 9. Add loader to .zshrc ===
if ! grep -q "dotfiles/shell/_loader.zsh" ~/.zshrc 2>/dev/null; then
  echo "" >> ~/.zshrc
  echo "# === Personal Platform ===" >> ~/.zshrc
  echo "[ -f ~/dotfiles/shell/_loader.zsh ] && source ~/dotfiles/shell/_loader.zsh" >> ~/.zshrc
  echo "✓ Loader added to ~/.zshrc"
else
  echo "✓ Loader already in ~/.zshrc"
fi

# === Manual steps reminder ===
cat <<'MANUAL'

==============================================
✅ Automated setup complete!

Manual steps remaining (cannot be automated):

  1. Reload shell:
     source ~/.zshrc

  2. Login to agents:
     CODEX_HOME=~/.codex-new codex login
     CODEX_HOME=~/.codex-wiki codex login
     CLAUDE_CONFIG_DIR=~/.claude-new claude login

  3. (Optional) 1Password CLI:
     op signin

  4. SSH keys for git:
     ssh-keygen -t ed25519 -C "your-email@example.com"
     cat ~/.ssh/id_ed25519.pub  # add this to GitHub/GitLab

  5. Test platform:
     ?                  # should list all commands
     wikipedik          # should open tmux

  6. Open Obsidian: open -a Obsidian
     File → Open Vault → ~/Desktop/WikiPedik/dev
     (then open research vault separately)

==============================================
MANUAL
