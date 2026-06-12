wikipedik() {
  tmux kill-session -t wikipedik 2>/dev/null

  mkdir -p ~/Desktop/WikiPedik/Personal\ Brand

  tmux new -d -s wikipedik -c ~/Desktop/WikiPedik/dev
  tmux send-keys -t wikipedik:0 'export CODEX_HOME=$HOME/.codex-wiki; clear; codex' Enter

  tmux split-window -v -t wikipedik:0 -c ~/Desktop/WikiPedik/research
  tmux send-keys -t wikipedik:0.1 'export CODEX_HOME=$HOME/.codex-wiki; clear; codex' Enter

  tmux split-window -h -t wikipedik:0.1 -c ~/Desktop/WikiPedik/Personal\ Brand
  tmux send-keys -t wikipedik:0.2 'export CODEX_HOME=$HOME/.codex-wiki; clear; codex' Enter

  # Memory ops: prompt composer for the dev chat + quick mechanical actions
  tmux split-window -h -t wikipedik:0.0 -c ~/Desktop/WikiPedik/dev
  tmux send-keys -t wikipedik:0.1 'bash ~/dotfiles/scripts/wiki-menu.sh' Enter

  tmux select-layout -t wikipedik:0 tiled
  tmux select-pane -t wikipedik:0.0
  tmux attach -t wikipedik
}
