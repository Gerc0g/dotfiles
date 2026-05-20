# Global agent defaults — loaded first by ~/dotfiles/shell/_loader.zsh
# Target: ~/dotfiles/shell/05-agent-defaults.zsh
#
# Default profile: fresh (`~/.codex-new`, `~/.claude-new`).
# Override per-session via `agent <profile>` command.

export CODEX_HOME="${CODEX_HOME:-$HOME/.codex-new}"
export CLAUDE_CONFIG_DIR="${CLAUDE_CONFIG_DIR:-$HOME/.claude-new}"

# Diagnostic helper — `agent-debug` показывает что подхватилось
#
# ## agent-debug
# **Что:** Показывает текущие CODEX_HOME / CLAUDE_CONFIG_DIR и какие файлы там лежат
# **Запуск:** `agent-debug`
agent-debug() {
  echo "=== Agent config diagnostic ==="
  echo "CODEX_HOME = $CODEX_HOME"
  echo "CLAUDE_CONFIG_DIR = $CLAUDE_CONFIG_DIR"
  echo ""
  echo "--- $CODEX_HOME contents ---"
  ls -la "$CODEX_HOME" 2>/dev/null | head -20
  echo ""
  echo "--- $CLAUDE_CONFIG_DIR contents ---"
  ls -la "$CLAUDE_CONFIG_DIR" 2>/dev/null | head -20
  echo ""
  echo "--- Active codex processes ---"
  pgrep -fl codex 2>/dev/null | head -5
  echo ""
  echo "--- Codex binary version ---"
  codex --version 2>/dev/null
}
