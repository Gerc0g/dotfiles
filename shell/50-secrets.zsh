# Secrets layer is a stub while it is being redesigned (`hq secret`).
#
# `secret signin` stays functional: `eval "$(op signin)"` mutates the session
# environment (a child process cannot), and op auth is still needed by
# onboarding for vault creation.
secret() {
  case "${1:-}" in
    signin) eval "$(op signin)" ;;
    *)      hq secret "$@" ;;
  esac
}
