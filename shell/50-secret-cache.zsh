# 1Password secret cache. The logic lives in the core (`hq secret cache`);
# generated .envrc files call scripts/secret-cache.sh, which execs the same.
secret-cache() {
  hq secret cache "$@"
}
