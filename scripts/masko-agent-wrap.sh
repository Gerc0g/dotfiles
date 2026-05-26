#!/usr/bin/env bash
# Wrap a local agent process and report explicit lifecycle events to Masko Code.
# Usage: masko-agent-wrap.sh <source> <repo_dir> <launch_session> -- <command> [args...]

set -u

source_name=${1:-}
repo_dir=${2:-}
launch_session=${3:-}
shift 3 || true
if [ "${1:-}" = "--" ]; then
  shift
fi

if [ -z "$source_name" ] || [ -z "$repo_dir" ] || [ -z "$launch_session" ] || [ $# -eq 0 ]; then
  echo "usage: masko-agent-wrap.sh <source> <repo_dir> <launch_session> -- <command> [args...]" >&2
  exit 64
fi

session_id="dotfiles-${source_name}-${launch_session}-$$"
export MASKO_DOTFILES_SESSION_ID="$session_id"
MASKO_HOOK="${MASKO_HOOK:-$HOME/.masko-desktop/hooks/hook-sender.sh}"
[ -x "$MASKO_HOOK" ] || MASKO_HOOK="$HOME/.masko-desktop/hooks/hook-sender"

_masko_payload() {
  local event_name=$1
  local exit_code=${2:-0}
  MASKO_EVENT_NAME="$event_name" \
  MASKO_EXIT_CODE="$exit_code" \
  MASKO_SOURCE="$source_name" \
  MASKO_REPO_DIR="$repo_dir" \
  MASKO_LAUNCH_SESSION="$launch_session" \
  MASKO_SESSION_ID="$session_id" \
  python3 - <<'PYCODE'
import json
import os

payload = {
    "hook_event_name": os.environ["MASKO_EVENT_NAME"],
    "session_id": os.environ["MASKO_SESSION_ID"],
    "source": os.environ["MASKO_SOURCE"],
    "cwd": os.environ["MASKO_REPO_DIR"],
    "project_dir": os.environ["MASKO_REPO_DIR"],
    "launch_session": os.environ["MASKO_LAUNCH_SESSION"],
    "process_id": os.getppid(),
}
if payload["hook_event_name"] == "SessionEnd":
    payload["exit_code"] = int(os.environ.get("MASKO_EXIT_CODE", "0") or "0")

print(json.dumps(payload, ensure_ascii=False, separators=(",", ":")))
PYCODE
}

_masko_send() {
  local event_name=$1
  local exit_code=${2:-0}
  local payload
  payload=$(_masko_payload "$event_name" "$exit_code") || return 0

  if [ -x "$MASKO_HOOK" ]; then
    printf '%s' "$payload" | "$MASKO_HOOK" >/dev/null 2>&1 || true
    return 0
  fi

  MASKO_PAYLOAD="$payload" python3 - <<'PYCODE' >/dev/null 2>&1 || true
import os
import urllib.request

payload = os.environ["MASKO_PAYLOAD"].encode("utf-8")
try:
    urllib.request.urlopen("http://127.0.0.1:45832/health", timeout=0.3).read()
except Exception:
    raise SystemExit(0)

req = urllib.request.Request(
    "http://127.0.0.1:45832/hook",
    data=payload,
    headers={"Content-Type": "application/json"},
    method="POST",
)
urllib.request.urlopen(req, timeout=1.0).read()
PYCODE
}

_git_guard() {
  case "${AGENT_GIT_MODE:-}" in commit-local) ;; *) return 0 ;; esac
  [ -d "$repo_dir/.git" ] || [ -f "$repo_dir/.git" ] || return 0

  local status branch last_commit
  status=$(git -C "$repo_dir" status --short 2>/dev/null || true)
  [ -n "$status" ] || return 0
  branch=$(git -C "$repo_dir" branch --show-current 2>/dev/null || true)
  last_commit=$(git -C "$repo_dir" log -1 --pretty=format:'%h %s' 2>/dev/null || true)

  cat >&2 <<EOF

AGENT GIT GUARD: repo is dirty after agent process exit
repo: $repo_dir
branch: ${branch:-<detached-or-unknown>}
last commit: ${last_commit:-<none>}
mode: ${AGENT_GIT_MODE:-manual}

Dirty files:
$status

Required next step for agents:
  ~/dotfiles/scripts/agent-commit.sh "type(scope): русское описание" -- <explicit paths>

Do not start the next feature/epic until the completed logical change is committed locally.
Finish only when the task/branch is complete:
  ~/dotfiles/scripts/agent-finish.sh
EOF
}

_masko_send SessionStart 0
(
  while true; do
    sleep "${MASKO_HEARTBEAT_SECONDS:-60}"
    _masko_send SessionStart 0
  done
) &
heartbeat_pid=$!

_cleanup_heartbeat() {
  kill "$heartbeat_pid" >/dev/null 2>&1 || true
  wait "$heartbeat_pid" >/dev/null 2>&1 || true
}
trap _cleanup_heartbeat EXIT INT TERM

"$@"
status=$?
_cleanup_heartbeat
_git_guard
_masko_send SessionEnd "$status"
exit "$status"
