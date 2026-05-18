# Project tmux launchers

agents() {
  tmux kill-session -t agents 2>/dev/null
  tmux new -d -s agents -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/agents/barrier
  tmux send-keys -t agents:0 'export CODEX_HOME=$HOME/.codex-new; clear; codex' Enter
  tmux split-window -v -t agents:0 -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/agents/barrier
  tmux send-keys -t agents:0.1 'git status; echo' Enter
  tmux attach -t agents
}

legacy() {
  tmux kill-session -t legacy 2>/dev/null
  tmux new -d -s legacy -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/legacy/agent_core
  tmux send-keys -t legacy:0 'export CODEX_HOME=$HOME/.codex-new; clear; codex' Enter
  tmux split-window -v -t legacy:0 -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/legacy/agent_core
  tmux send-keys -t legacy:0.1 'git status; echo' Enter
  tmux attach -t legacy
}

saas() {
  tmux kill-session -t saas 2>/dev/null
  tmux new -d -s saas -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/saas/backend
  tmux send-keys -t saas:0 'export CODEX_HOME=$HOME/.codex-new; clear; codex' Enter
  tmux split-window -v -t saas:0 -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/saas/backend
  tmux send-keys -t saas:0.1 'git status; echo' Enter
  tmux attach -t saas
}

infra() {
  tmux kill-session -t infra 2>/dev/null
  tmux new -d -s infra -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/infra/infra
  tmux send-keys -t infra:0 'export CODEX_HOME=$HOME/.codex-new; clear; codex' Enter
  tmux split-window -v -t infra:0 -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/infra/infra
  tmux send-keys -t infra:0.1 'git status; echo' Enter
  tmux attach -t infra
}

wiki() {
  tmux kill-session -t wiki 2>/dev/null
  tmux new -d -s wiki -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/wiki/nrdsk_wiki
  tmux send-keys -t wiki:0 'export CODEX_HOME=$HOME/.codex-new; clear; codex' Enter
  tmux split-window -v -t wiki:0 -c /Users/_gerc0g/Desktop/Prokectfiles/neurodesk/wiki/nrdsk_wiki
  tmux send-keys -t wiki:0.1 'git status; echo' Enter
  tmux attach -t wiki
}
