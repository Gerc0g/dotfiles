# Scoped 1Password secrets. The logic lives in the core (`hq secret`).
#
# `secret signin` is the one subcommand that must stay in the shell: it
# mutates the session environment through `eval "$(op signin)"`, which a
# child process cannot do for its caller.
secret() {
  case "${1:-}" in
    signin) eval "$(op signin)" ;;
    *)      hq secret "$@" ;;
  esac
}
