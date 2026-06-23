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

# Idempotently create a detached server tmux session with the SERVER marker.
_srv_create() {
  local name="$1" dir="${2:-\$HOME}"
  ssh "${_SRV_SSH_OPTS[@]}" "$SERVER_SSH" "tmux has-session -t '$name' 2>/dev/null || tmux new-session -d -s '$name' -c \"$dir\"; tmux set-option -t '$name' status-style 'bg=colour166,fg=colour231,bold'; tmux set-option -t '$name' status-left ' 🖧 SERVER · $name '; tmux set-option -t '$name' status-left-length 40"
}

# Server session + mirror it here (attach). Survives disconnect.
slaunch() {
  [ -z "$1" ] && { echo 'Usage: slaunch <name> [dir]'; return 1; }
  _srv_preflight || return 1
  _srv_create "$1" "${2:-}" || { echo "⚠ не удалось создать серверную сессию"; return 1; }
  echo "🖧 server session '$1' @ $SERVER_SSH — подключаюсь (Ctrl-b d = detach, сессия продолжит жить)"
  ssh "${_SRV_SSH_OPTS[@]}" -t "$SERVER_SSH" "TERM=xterm-256color tmux attach -t '$1'"
}

# Server session WITHOUT mirroring — print how to join from the phone.
mlaunch() {
  [ -z "$1" ] && { echo 'Usage: mlaunch <name> [dir]'; return 1; }
  _srv_preflight || return 1
  _srv_create "$1" "${2:-}" || { echo "⚠ не удалось создать серверную сессию"; return 1; }
  cat <<EOF
🖧 Серверная сессия '$1' поднята на $SERVER_SSH (detached; на экран Mac НЕ зеркалится).
Зайти с телефона:
  1) Blink Shell (или любой ssh-клиент) + включённый Tailscale
  2) ssh $SERVER_SSH
  3) tmux attach -t $1
С этого Mac позже:  srv-attach $1
EOF
}

srv-ls()     { _srv_preflight || return 1; ssh "${_SRV_SSH_OPTS[@]}" "$SERVER_SSH" 'tmux ls'; }
srv-attach() { [ -z "$1" ] && { echo 'Usage: srv-attach <name>'; return 1; }; _srv_preflight || return 1; ssh "${_SRV_SSH_OPTS[@]}" -t "$SERVER_SSH" "TERM=xterm-256color tmux attach -t '$1'"; }
srv-kill()   { [ -z "$1" ] && { echo 'Usage: srv-kill <name>'; return 1; }; _srv_preflight || return 1; ssh "${_SRV_SSH_OPTS[@]}" "$SERVER_SSH" "tmux kill-session -t '$1'" && echo "killed $1"; }
