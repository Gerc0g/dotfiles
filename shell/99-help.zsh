# Command index. The rendering lives in the core (`hq commands`); the alias
# must stay in the shell.
help() {
  hq commands "$@"
}

alias '?'=help
