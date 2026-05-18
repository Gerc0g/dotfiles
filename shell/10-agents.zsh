agent() {
  case "$1" in
    legacy) export CODEX_HOME=$HOME/.codex; export CLAUDE_CONFIG_DIR=$HOME/.claude; echo "→ LEGACY" ;;
    fresh)  export CODEX_HOME=$HOME/.codex-new; export CLAUDE_CONFIG_DIR=$HOME/.claude-new; echo "→ FRESH" ;;
    wiki)   export CODEX_HOME=$HOME/.codex-wiki; export CLAUDE_CONFIG_DIR=$HOME/.claude-new; echo "→ WIKI" ;;
    status|"") echo "CODEX_HOME=$CODEX_HOME"; echo "CLAUDE_CONFIG_DIR=$CLAUDE_CONFIG_DIR" ;;
    *) echo "Usage: agent {legacy|fresh|wiki|status}" ;;
  esac
}
