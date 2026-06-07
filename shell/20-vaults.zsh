wikipedik() {
  tmux kill-session -t wikipedik 2>/dev/null

  mkdir -p ~/Desktop/WikiPedik/Personal\ Brand

  tmux new -d -s wikipedik -c ~/Desktop/WikiPedik/dev
  tmux send-keys -t wikipedik:0 'export CODEX_HOME=$HOME/.codex-wiki; clear; codex' Enter

  tmux split-window -v -t wikipedik:0 -c ~/Desktop/WikiPedik/research
  tmux send-keys -t wikipedik:0.1 'export CODEX_HOME=$HOME/.codex-wiki; clear; codex' Enter

  tmux split-window -h -t wikipedik:0.1 -c ~/Desktop/WikiPedik/Personal\ Brand
  tmux send-keys -t wikipedik:0.2 'export CODEX_HOME=$HOME/.codex-wiki; clear; codex' Enter

  tmux select-layout -t wikipedik:0 tiled
  tmux attach -t wikipedik
}
