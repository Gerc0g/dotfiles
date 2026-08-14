# Company/product onboarding. The logic lives in the core (`hq onboard`);
# the reload stays here so the new company function appears in this session.
new-company() {
  hq onboard company "$@" && source ~/.zshrc
}

new-project() {
  hq onboard product "$@" && source ~/.zshrc
}
