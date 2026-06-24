#!/usr/bin/env bash
# server-bootstrap.sh — provision the home Ubuntu server (gerc0g) for the same
# dev+agent flow as the Mac. Linux sibling of bootstrap.sh (which is macOS-only:
# brew/launchctl/OrbStack). Run AS ROOT, ON THE SERVER. Idempotent: re-runnable.
#
#   ssh gerc0g
#   bash ~/dotfiles/scripts/server-bootstrap.sh
#
# Chicken-and-egg: getting ~/dotfiles + ssh keys onto the server happens FROM THE
# MAC first (rsync), because the server has no git auth yet. From the Mac:
#   rsync -a ~/dotfiles gerc0g:~/                          # this repo
#   rsync -a ~/.ssh/{*_id_ed25519,*_id_ed25519.pub,gitlab_nrdsk} gerc0g:~/.ssh/
#   rsync -a ~/.codex-new/{auth.json,config.toml} gerc0g:~/.codex-new/
#   infocmp -x xterm-ghostty | ssh gerc0g 'tic -x -'      # terminfo (optional)
#   # cross-build TUIs on the Mac, then copy:
#   for t in oracle status test; do (cd ~/dotfiles/tools/$t-tui && GOOS=linux GOARCH=amd64 go build -o /tmp/$t-tui .); rsync /tmp/$t-tui gerc0g:~/bin/; done
#
# This script does NOT use `set -e`: optional steps degrade with a warning.

set -uo pipefail
log(){ printf '\033[1;36m▶ %s\033[0m\n' "$*"; }
ok(){  printf '  \033[32m✓\033[0m %s\n' "$*"; }
warn(){ printf '  \033[33m⚠ %s\033[0m\n' "$*" >&2; }
have(){ command -v "$1" >/dev/null 2>&1; }

# ─── §0 guard + server env ───
log "§0 environment"
[ "$(uname -s)" = "Linux" ] || { warn "not Linux — this is the SERVER bootstrap"; exit 1; }
export DEBIAN_FRONTEND=noninteractive
SUDO=$([ "$(id -u)" -eq 0 ] && echo "" || echo sudo)   # server runs as root → sudo is a no-op
: "${PROKECTFILES_ROOT:=$HOME/Desktop/Prokectfiles}"   # keep same layout as Mac
export PATH="$HOME/.local/bin:$HOME/bin:/usr/local/go/bin:$PATH"   # so have() finds installed tools on re-run
ENVFILE="$HOME/dotfiles/shell/00-server-env.local.zsh"   # gitignored, server-only
ok "HOME=$HOME  PROKECTFILES_ROOT=$PROKECTFILES_ROOT"

# ─── §1 apt toolchain ───
log "§1 apt toolchain"
APT_PKGS="zsh direnv ripgrep fd-find bat fzf build-essential unzip jq"
$SUDO apt-get update -qq && $SUDO apt-get install -y -qq $APT_PKGS && ok "apt: $APT_PKGS" || warn "apt install partial"
have fd || { [ -e /usr/bin/fdfind ] && ln -sf /usr/bin/fdfind "$HOME/bin/fd" 2>/dev/null; }
have bat || { [ -e /usr/bin/batcat ] && ln -sf /usr/bin/batcat "$HOME/bin/bat" 2>/dev/null; }

# ─── §2 non-apt installs (each guarded) ───
log "§2 toolchain (go / uv / node / op / gh / claude / codex)"
have go    || { curl -fsSL https://go.dev/dl/go1.24.2.linux-amd64.tar.gz | $SUDO tar -C /usr/local -xz && ok "go"; }
have uv    || { curl -fsSL https://astral.sh/uv/install.sh | sh && ok "uv"; }
have node  || { curl -fsSL https://deb.nodesource.com/setup_lts.x | $SUDO bash - && $SUDO apt-get install -y -qq nodejs && ok "node"; }
have corepack && corepack enable 2>/dev/null
have op    || warn "1Password CLI (op) missing — install from https://1password.com/downloads/command-line/ (needed for secrets)"
have gh    || { (type apt-get >/dev/null && $SUDO apt-get install -y -qq gh) || warn "gh: add cli.github.com apt repo"; }
have glab  || warn "glab missing — install .deb from gitlab-org/cli if you use GitLab MRs"
have claude || { ct=$(mktemp); if curl -fsSL https://claude.ai/install.sh -o "$ct" 2>/dev/null && head -1 "$ct" | grep -q '^#!'; then bash "$ct" && ok "claude"; else warn "claude install blocked (region/geo — got non-script HTML). Server egress must go via an allowed region (VPN). Deferred."; fi; rm -f "$ct"; }
have codex || { npm i -g @openai/codex 2>/dev/null && ok "codex"; }
have oracle || { npm i -g @steipete/oracle 2>/dev/null && ok "oracle"; }

# ─── §3 dotfiles (assumed rsynced; clone as fallback once keys exist) ───
log "§3 dotfiles"
[ -d "$HOME/dotfiles" ] && ok "~/dotfiles present" || warn "~/dotfiles missing — rsync it from the Mac (see header)"

# ─── §4 agent profiles + symlinks (link_profile pattern from bootstrap.sh) ───
log "§4 agent profiles"
mkdir -p "$HOME/.codex-new" "$HOME/.codex-setup" "$HOME/.claude-new" "$HOME/.claude-setup" "$HOME/bin"
link_profile(){ local src="$1" dst="$2"; [ -e "$src" ] || return 0; [ -L "$dst" ] && [ "$(readlink "$dst")" = "$src" ] && return 0; [ -e "$dst" ] && mv "$dst" "$dst.bak.$$"; ln -s "$src" "$dst" && ok "link $(basename "$dst")"; }
link_profile "$HOME/dotfiles/agent-profiles/BASELINE.md" "$HOME/.codex-new/AGENTS.md"
link_profile "$HOME/dotfiles/agent-profiles/BASELINE.md" "$HOME/.codex-setup/AGENTS.md"
link_profile "$HOME/dotfiles/agent-profiles/BASELINE.md" "$HOME/.claude-new/CLAUDE.md"
link_profile "$HOME/dotfiles/agent-profiles/BASELINE.md" "$HOME/.claude-setup/CLAUDE.md"

# ─── §5 server env file + zshrc loader + default shell ───
log "§5 shell env"
cat > "$ENVFILE" <<EOF
# server-only overrides (gitignored). Loaded by the dotfiles loader.
export DEV_STACK_HOST=localhost          # dev-stack runs HERE
export PROKECTFILES_ROOT="$PROKECTFILES_ROOT"
export CODEX_HOME="\$HOME/.codex-new"
export CLAUDE_CONFIG_DIR="\$HOME/.claude-new"
export PATH="\$HOME/.local/bin:\$HOME/bin:/usr/local/go/bin:\$PATH"
[ -f "\$HOME/.config/op/service-account-token" ] && export OP_SERVICE_ACCOUNT_TOKEN="\$(cat \$HOME/.config/op/service-account-token)"
EOF
# Route ONLY the agents through the local VPN proxy (sing-box :1080) so geo-blocked
# Anthropic/OpenAI become reachable; everything else (git/gh/apt/tailnet/dev-stack)
# stays direct. Falls back to direct (with a warning) if the proxy is down.
cat >> "$ENVFILE" <<'WRAP'

export AGENT_PROXY="http://127.0.0.1:1080"
export AGENT_NO_PROXY="localhost,127.0.0.1,::1,gerc0g,100.73.117.50,.local"
_agent_via_proxy() {
  if nc -z -w1 127.0.0.1 1080 2>/dev/null; then
    # NODE_USE_ENV_PROXY=1: Node 24 honors HTTPS_PROXY for fetch/undici only with this
    # (codex/claude are node CLIs and otherwise bypass the proxy → geo-block).
    HTTPS_PROXY="$AGENT_PROXY" HTTP_PROXY="$AGENT_PROXY" ALL_PROXY="$AGENT_PROXY" NO_PROXY="$AGENT_NO_PROXY" NODE_USE_ENV_PROXY=1 "$@"
  else
    echo "⚠ VPN-прокси (sing-box :1080) недоступен — '$2' идёт напрямую (geo-блок?). Проверь: systemctl status sing-box" >&2
    "$@"
  fi
}
claude() { _agent_via_proxy command claude "$@"; }
codex()  { _agent_via_proxy command codex  "$@"; }
WRAP
ok "wrote $ENVFILE (incl. agent VPN-proxy wrappers)"
grep -q 'dotfiles/shell/_loader.zsh' "$HOME/.zshrc" 2>/dev/null || { printf '\n[ -f ~/dotfiles/shell/_loader.zsh ] && source ~/dotfiles/shell/_loader.zsh\n' >> "$HOME/.zshrc"; ok "hooked loader into ~/.zshrc"; }
[ "$(basename "${SHELL:-}")" = zsh ] || { chsh -s "$(command -v zsh)" 2>/dev/null && ok "default shell → zsh"; }

# ─── §6 terminfo (best done from Mac; fallback note) ───
log "§6 terminfo"
infocmp xterm-ghostty >/dev/null 2>&1 && ok "xterm-ghostty terminfo present" || warn "xterm-ghostty terminfo missing — run from Mac: infocmp -x xterm-ghostty | ssh gerc0g 'tic -x -'  (or use TERM=xterm-256color)"

# ─── §7 TUI bins (cross-built on Mac, copied to ~/bin) ───
log "§7 TUI binaries"
for t in oracle status test; do have "$t-tui" || [ -x "$HOME/bin/$t-tui" ] && : || warn "$t-tui missing — cross-build on Mac (GOOS=linux GOARCH=amd64) and rsync to ~/bin (go is installed → go-run fallback also works)"; done

# ─── §8 STOP: interactive minimum ───
log "§8 manual steps left (irreducible — do these yourself)"
cat <<'EOF'
  1) 1Password Service Account: create in 1Password UI, grant READ on Work-* vaults,
     then place the token:  install -m600 /dev/stdin ~/.config/op/service-account-token <<<'ops_...'
     (ensure needed secrets live in Work-<company> vaults, NOT Personal).
  2) claude login:  run `claude` once → OAuth (Keychain creds can't be copied from Mac).
  3) oracle / ChatGPT Pro session for oracle-loop.sh (one-time).
  4) smoke-test:  exec zsh ; dev-stack doctor ; op vault list ; codex --version ; claude --version
EOF
log "done. Re-run safely anytime (idempotent)."
