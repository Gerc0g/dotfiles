# new-project — bootstrap a new project from templates
new-project() {
  ~/dotfiles/scripts/new-project.sh "$@"
  source ~/.zshrc 2>/dev/null
}
