wikipedik() {
  tmux kill-session -t wikipedik 2>/dev/null
  tmux new -d -s wikipedik -c ~/Desktop/WikiPedik/dev
  tmux send-keys -t wikipedik:0 'export CODEX_HOME=$HOME/.codex-wiki; clear; codex' Enter
  tmux split-window -v -t wikipedik:0 -c ~/Desktop/WikiPedik/research
  tmux send-keys -t wikipedik:0.1 'export CODEX_HOME=$HOME/.codex-wiki; clear; codex' Enter
  tmux attach -t wikipedik
}
