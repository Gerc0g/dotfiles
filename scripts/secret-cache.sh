#!/usr/bin/env bash
# Cache 1Password item fields locally for a short TTL to avoid Touch ID on every cd.

set -euo pipefail

CACHE_ROOT="${SECRET_CACHE_DIR:-${XDG_CACHE_HOME:-$HOME/.cache}/dotfiles/secrets}"
TTL_DAYS="${SECRET_CACHE_TTL_DAYS:-30}"

usage() {
  cat <<'USAGE'
Usage:
  secret-cache get <vault> <item> [field] [ttl_days]
  secret-cache refresh <vault> <item> [field]
  secret-cache clear [<vault> <item> [field]]
  secret-cache list

Defaults:
  field=credential
  ttl_days=$SECRET_CACHE_TTL_DAYS or 30

Examples:
  export TOKEN="$(secret-cache get Work-neurodesk agents__synapse__OPENROUTER_API_KEY credential)"
  SECRET_CACHE_TTL_DAYS=7 secret-cache get Work-neurodesk _company__GITLAB_TOKEN credential
USAGE
}

cache_key() {
  local vault=$1 item=$2 field=$3
  printf '%s' "${vault}|${item}|${field}" | shasum -a 256 | awk '{print $1}'
}

cache_file() {
  local key=$1
  printf '%s/%s.value' "$CACHE_ROOT" "$key"
}

meta_file() {
  local key=$1
  printf '%s/%s.meta' "$CACHE_ROOT" "$key"
}

mtime_epoch() {
  stat -f '%m' "$1" 2>/dev/null || stat -c '%Y' "$1"
}

is_fresh() {
  local file=$1 ttl_days=$2
  [ -f "$file" ] || return 1
  [ "${SECRET_CACHE_BYPASS:-}" = "1" ] && return 1

  local now mtime max_age
  now=$(date +%s)
  mtime=$(mtime_epoch "$file")
  max_age=$((ttl_days * 24 * 60 * 60))
  [ $((now - mtime)) -lt "$max_age" ]
}

ensure_cache_dir() {
  mkdir -p "$CACHE_ROOT"
  chmod 700 "$CACHE_ROOT"
}

read_from_1password() {
  local vault=$1 item=$2 field=$3
  command -v op >/dev/null 2>&1 || {
    echo "secret-cache: op CLI not found" >&2
    return 127
  }
  op item get "$item" --vault "$vault" --fields "label=$field" --reveal
}

write_cache() {
  local file=$1 meta=$2 vault=$3 item=$4 field=$5 value=$6
  local tmp
  tmp=$(mktemp "${file}.XXXXXX")
  chmod 600 "$tmp"
  printf '%s' "$value" > "$tmp"
  mv "$tmp" "$file"
  chmod 600 "$file"
  {
    printf 'vault=%s\n' "$vault"
    printf 'item=%s\n' "$item"
    printf 'field=%s\n' "$field"
    printf 'updated_at=%s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  } > "$meta"
  chmod 600 "$meta"
}

get_secret() {
  local vault=$1 item=$2 field=${3:-credential} ttl_days=${4:-$TTL_DAYS}
  local key file meta value

  ensure_cache_dir
  key=$(cache_key "$vault" "$item" "$field")
  file=$(cache_file "$key")
  meta=$(meta_file "$key")

  if is_fresh "$file" "$ttl_days"; then
    cat "$file"
    return 0
  fi

  value=$(read_from_1password "$vault" "$item" "$field")
  write_cache "$file" "$meta" "$vault" "$item" "$field" "$value"
  printf '%s' "$value"
}

clear_secret() {
  ensure_cache_dir
  if [ $# -eq 0 ]; then
    rm -f "$CACHE_ROOT"/*.value "$CACHE_ROOT"/*.meta 2>/dev/null || true
    return 0
  fi

  local vault=$1 item=$2 field=${3:-credential} key
  key=$(cache_key "$vault" "$item" "$field")
  rm -f "$(cache_file "$key")" "$(meta_file "$key")"
}

list_cache() {
  ensure_cache_dir
  for meta in "$CACHE_ROOT"/*.meta; do
    [ -f "$meta" ] || return 0
    sed -n 's/^\(vault\|item\|field\|updated_at\)=/\1: /p' "$meta" | paste -sd ' | ' -
  done
}

cmd=${1:-}
[ -n "$cmd" ] || { usage; exit 2; }
shift

case "$cmd" in
  get)
    [ $# -ge 2 ] || { usage; exit 2; }
    get_secret "$@"
    ;;
  refresh)
    [ $# -ge 2 ] || { usage; exit 2; }
    SECRET_CACHE_BYPASS=1 get_secret "$@"
    ;;
  clear)
    clear_secret "$@"
    ;;
  list)
    list_cache
    ;;
  help|-h|--help)
    usage
    ;;
  *)
    usage
    exit 2
    ;;
esac
