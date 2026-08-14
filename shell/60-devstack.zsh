# Shared local dev infrastructure. The logic lives in the core (`hq devstack`).
#
# DEV_STACK_HOST stays exported here because project .envrc blocks resolve
# endpoints from it literally at `cd` time; the wrapper only dispatches.
export DEV_STACK_HOST="${DEV_STACK_HOST:-localhost}"

dev-stack() {
  hq devstack "${@:-}"
}
