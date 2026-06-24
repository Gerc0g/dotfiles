# Run sessions ON the home server and view them from the Mac or phone.
#
# The session LIVES on the server (tmux). You just ATTACH to it — drop the Mac
# connection and it keeps running. Three modes from the Mac:
#   slaunch <name> [dir]   server session + attach here (mirror on Mac)
#   mlaunch <name> [dir]   server session, NO mirror — prints how to join from phone
# Manage:
#   srv-ls · srv-attach <name> · srv-kill <name>
#
# Server = ${SERVER_SSH:-gerc0g} (Tailscale SSH). Server sessions get an ORANGE
# status bar (marker) so they are visually distinct from local Mac sessions.
#
# All ssh has a ConnectTimeout so it never hangs silently. A preflight check
# detects the one-time Tailscale SSH re-auth and tells you what to do instead of
# hanging.
#
# NOTE: until the server is provisioned with the agent toolchain + dotfiles, a
# server session is a plain shell — enough to test open/mirror/phone plumbing.
# Later `_srv_create` will call the server-side `launch` for the 4-pane layout.

: "${SERVER_SSH:=gerc0g}"
_SRV_SSH_OPTS=(-o ConnectTimeout=10)

# Fast reachability probe (BatchMode so a pending Tailscale browser-check fails
# fast instead of hanging on an interactive prompt).
_srv_reachable() {
  ssh "${_SRV_SSH_OPTS[@]}" -o BatchMode=yes "$SERVER_SSH" true 2>/dev/null
}

_srv_preflight() {
  _srv_reachable && return 0
  cat >&2 <<EOF
⚠ Сервер '$SERVER_SSH' недоступен по ssh.
Скорее всего — разовая Tailscale SSH-проверка. Пройди её ВРУЧНУЮ:
    ssh $SERVER_SSH
открой показанную ссылку login.tailscale.com → подтверди в браузере (или
переавторизуйся в приложении Tailscale). Затем повтори команду.
EOF
  return 1
}

# Build (idempotently) a detached 4-pane AGENT panel on the server — like `launch`:
#   🧠 plan·codex   💻 code·claude   🧪 test   🔮 oracle  (2x2, orange SERVER marker)
# codex/claude go through the VPN proxy via shell wrappers; oracle inherits the
# proxy from the session env. Workspace: ~/agents/<name> (git-init'd scratch repo).
_srv_panel() {
  local name="$1" dir="${2:-}"
  ssh "${_SRV_SSH_OPTS[@]}" "$SERVER_SSH" 'bash -s' "$name" "$dir" <<'REMOTE'
set -e
S="$1"; D="${2:-$HOME/agents/$1}"
tmux has-session -t "$S" 2>/dev/null && exit 0
mkdir -p "$D"; [ -d "$D/.git" ] || ( cd "$D" && git init -q )
P=$(tmux new-session -d -s "$S" -c "$D" -n work -P -F '#{pane_id}')
# Proxy via SESSION env → inherited by panes (NO visible export lines). Only agent
# traffic uses the VPN; git/tailnet/dev-stack bypass via NO_PROXY. Node CLIs need
# NODE_USE_ENV_PROXY=1. (Pane P starts before this and relies on its codex wrapper.)
tmux set-environment -t "$S" HTTPS_PROXY http://127.0.0.1:1080
tmux set-environment -t "$S" HTTP_PROXY http://127.0.0.1:1080
tmux set-environment -t "$S" ALL_PROXY http://127.0.0.1:1080
tmux set-environment -t "$S" NODE_USE_ENV_PROXY 1
tmux set-environment -t "$S" NO_PROXY localhost,127.0.0.1,::1,gerc0g,100.73.117.50,github.com,nrdsk.gitlab.yandexcloud.net,.local
tmux set-option -t "$S" status-style 'bg=colour166,fg=colour231,bold'
tmux set-option -t "$S" status-left " 🖧 SERVER · $S "
tmux set-option -t "$S" status-left-length 44
tmux set-window-option -t "$S:work" pane-border-status top
tmux set-window-option -t "$S:work" pane-border-format ' #{@role} '
C=$(tmux split-window -h -t "$P" -c "$D" -P -F '#{pane_id}')
T=$(tmux split-window -v -t "$P" -c "$D" -P -F '#{pane_id}')
O=$(tmux split-window -v -t "$C" -c "$D" -P -F '#{pane_id}')
# layout = codex TL, claude TR, test BL, oracle BR (same as real `launch`; NO tiled,
# which would re-sort panes by index and flip claude to the bottom).
tmux set-option -p -t "$P" @role '🧠 plan · codex'
tmux set-option -p -t "$C" @role '💻 code · claude'
tmux set-option -p -t "$T" @role '🧪 test'
tmux set-option -p -t "$O" @role '🔮 oracle'
tmux send-keys -t "$P" 'codex' Enter
tmux send-keys -t "$C" 'claude' Enter
tmux send-keys -t "$O" "bash \"\$HOME/dotfiles/scripts/oracle-loop.sh\" \"$D\"" Enter
# test pane: auto-detect the test command from the repo, like real `launch`.
set +e
TCMD=""
if [ -f "$D/Makefile" ]; then
  grep -qE '^test-watch:' "$D/Makefile" && TCMD="make test-watch"
  [ -z "$TCMD" ] && grep -qE '^test:' "$D/Makefile" && TCMD="make test"
fi
if [ -z "$TCMD" ] && [ -f "$D/package.json" ]; then
  grep -q '"test:watch"' "$D/package.json" && TCMD="npm run test:watch"
  [ -z "$TCMD" ] && grep -q '"test"' "$D/package.json" && TCMD="npm test"
fi
[ -z "$TCMD" ] && [ -f "$D/go.mod" ] && TCMD="go test ./..."
[ -z "$TCMD" ] && { [ -f "$D/pyproject.toml" ] || [ -f "$D/setup.py" ]; } && TCMD="uv run pytest -v --tb=short"
set -e
if [ -n "$TCMD" ]; then
  tmux send-keys -t "$T" "$TCMD" Enter
else
  tmux send-keys -t "$T" "clear; echo 'test · нет Makefile/tests в этом demo-каталоге — на реальной репе тут запустится make/npm/go/pytest'" Enter
fi
tmux select-pane -t "$P"
REMOTE
}

# Server session + mirror it here (attach). Survives disconnect.
slaunch() {
  [ -z "$1" ] && { echo 'Usage: slaunch <name> [dir]'; return 1; }
  _srv_preflight || return 1
  _srv_panel "$1" "${2:-}" || { echo "⚠ не удалось создать серверную панель"; return 1; }
  echo "🖧 server agent panel '$1' @ $SERVER_SSH — подключаюсь (Ctrl-b d = detach, панель живёт на сервере)"
  ssh "${_SRV_SSH_OPTS[@]}" -t "$SERVER_SSH" "TERM=xterm-256color tmux attach -t '$1'"
}

# Server session WITHOUT mirroring — print how to join from the phone.
mlaunch() {
  [ -z "$1" ] && { echo 'Usage: mlaunch <name> [dir]'; return 1; }
  _srv_preflight || return 1
  _srv_panel "$1" "${2:-}" || { echo "⚠ не удалось создать серверную панель"; return 1; }
  local ip; ip=$(ssh "${_SRV_SSH_OPTS[@]}" "$SERVER_SSH" 'tailscale ip -4 2>/dev/null | head -1' 2>/dev/null)
  [ -z "$ip" ] && ip=100.73.117.50
  cat <<EOF
🖧 Серверная панель '$1' поднята на $SERVER_SSH (detached; на Mac НЕ зеркалится).

С ТЕЛЕФОНА (Tailscale включён на телефоне):
  ssh-клиент:   iOS → Blink Shell · Android → Termux
  подключиться: mosh root@$ip       (или ssh root@$ip — если без mosh)
  открыть панель: tmux attach -t $1
  на телефоне:  Ctrl-b z = развернуть одну панель на весь экран · тап = переключить
С этого Mac:    srv-attach $1
EOF
}

srv-ls()     { _srv_preflight || return 1; ssh "${_SRV_SSH_OPTS[@]}" "$SERVER_SSH" 'tmux ls'; }
srv-attach() { [ -z "$1" ] && { echo 'Usage: srv-attach <name>'; return 1; }; _srv_preflight || return 1; ssh "${_SRV_SSH_OPTS[@]}" -t "$SERVER_SSH" "TERM=xterm-256color tmux attach -t '$1'"; }
srv-kill()   { [ -z "$1" ] && { echo 'Usage: srv-kill <name>'; return 1; }; _srv_preflight || return 1; ssh "${_SRV_SSH_OPTS[@]}" "$SERVER_SSH" "tmux kill-session -t '$1'" && echo "killed $1"; }
