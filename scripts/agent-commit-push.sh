#!/usr/bin/env bash
# Deprecated: per-commit push is disabled by default.
set -euo pipefail
cat >&2 <<'EOF'
agent-commit-push.sh is deprecated.
Use:
  agent-commit.sh "type(scope): русское описание" -- <explicit paths>
Then, when the logical task/branch is complete:
  agent-task-push.sh
EOF
if [ "${AGENT_ALLOW_PER_COMMIT_PUSH:-0}" != "1" ]; then
  exit 65
fi
msg=$1
shift
bash "$HOME/dotfiles/scripts/agent-commit.sh" "$msg" "$@"
bash "$HOME/dotfiles/scripts/agent-task-push.sh" --allow-dirty
