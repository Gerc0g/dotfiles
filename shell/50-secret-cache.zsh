# 1Password secret cache helper.
#
# Keeps .envrc files from calling `op read` on every cd. Cached values live in
# ~/.cache/dotfiles/secrets with chmod 600 and default TTL = 30 days.

secret-cache() {
  bash "$HOME/dotfiles/scripts/secret-cache.sh" "$@"
}
