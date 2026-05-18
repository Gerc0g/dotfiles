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
mkdir -p ~/.codex-new ~/.codex-wiki ~/.claude-new
echo "✓ ~/.codex-new, ~/.codex-wiki, ~/.claude-new"

# === 7. WikiPedik vault skeleton ===
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
mkdir -p ~/Desktop/Prokectfiles
echo "✓ ~/Desktop/Prokectfiles/ ready"

# === 9. Подключить loader в .zshrc ===
if ! grep -q "dotfiles/shell/_loader.zsh" ~/.zshrc 2>/dev/null; then
  echo "" >> ~/.zshrc
  echo "# === Personal Platform ===" >> ~/.zshrc
  echo "[ -f ~/dotfiles/shell/_loader.zsh ] && source ~/dotfiles/shell/_loader.zsh" >> ~/.zshrc
  echo "✓ Loader added to ~/.zshrc"
else
  echo "✓ Loader already in ~/.zshrc"
fi

# === 10. Проверка OrbStack ===
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
     CODEX_HOME=~/.codex-wiki codex login
     CLAUDE_CONFIG_DIR=~/.claude-new claude login

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
