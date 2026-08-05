# Central env defaults, loaded before everything else (00- sorts first).
#
# PROKECTFILES_ROOT: where company/product/repo workspaces + .worktrees live.
# Default = the Mac path (so existing behaviour is unchanged). On the server,
# export a different root in the server env (scripts/server-bootstrap.sh sets it),
# and every tool follows because they all read this var.
export PROKECTFILES_ROOT="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}"

# Core binary (bin/hq, built by `make build`). Prepended so a freshly built hq
# wins over anything installed globally.
export PATH="$HOME/dotfiles/bin:$PATH"
