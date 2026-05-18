# Platform commands: new-company, new-project
new-company() {
  ~/dotfiles/scripts/new-company.sh "$@"
  source ~/.zshrc 2>/dev/null
}

new-project() {
  ~/dotfiles/scripts/new-project.sh "$@"
  source ~/.zshrc 2>/dev/null
}
