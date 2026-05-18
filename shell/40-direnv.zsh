# Direnv hook — автоматически грузит .envrc при cd
eval "$(direnv hook zsh)"

# Подавить шумные logs от direnv (только важное)
export DIRENV_LOG_FORMAT=""
