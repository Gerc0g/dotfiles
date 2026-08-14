# VS Code Project Manager sync. The logic lives in the core (`hq editor sync`);
# the old command name is kept for muscle memory.
vscode-projects-sync() {
  hq editor sync "$@"
}
