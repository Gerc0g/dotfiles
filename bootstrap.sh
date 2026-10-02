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

if [[ ${1:-} == --mode ]]; then
  if [[ $# -lt 2 ]]; then echo 'Missing --mode value' >&2; exit 2; fi
  bootstrap_mode=${2:-}
  shift 2
  case "$bootstrap_mode" in
    server) exec bash "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)/services/hq-server/bootstrap.sh" "$@" ;;
    workstation) ;;
    *) echo 'Mode must be workstation or server' >&2; exit 2 ;;
  esac
fi
if [[ $(uname -s) != Darwin ]]; then
  echo 'Use --mode server for Linux; workstation bootstrap requires macOS.' >&2
  exit 2
fi

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

# === 8. Claude LSP plugins (network installs; profile state lives in hq setup) ===
echo ""
echo "→ Installing Claude LSP plugins..."
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

install_claude_lsp_plugins "$HOME/.claude"
echo "✓ Claude LSP plugins installed (pyright, vtsls, yaml-language-server)"

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
