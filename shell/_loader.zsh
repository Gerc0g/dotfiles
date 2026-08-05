# Keep the shell surface explicit. Adding a file under shell/ must not make an
# unfinished experiment globally available on the next terminal start.
_dotfiles_shell_modules=(
  00-env.zsh
  05-agent-defaults.zsh
  20-vaults.zsh
  30-platform.zsh
  30-projects.zsh
  32-agent-workspace.zsh
  33-vscode-projects.zsh
  40-direnv.zsh
  50-secret-cache.zsh
  50-secrets.zsh
  60-devstack.zsh
  69-status.zsh
  71-wiki.zsh
  72-agent-skill.zsh
  99-help.zsh
)

for _dotfiles_shell_module in "${_dotfiles_shell_modules[@]}"; do
  source "$HOME/dotfiles/shell/$_dotfiles_shell_module"
done

unset _dotfiles_shell_module _dotfiles_shell_modules
