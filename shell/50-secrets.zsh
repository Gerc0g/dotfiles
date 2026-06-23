secret() {
  local cmd="${1:-}"
  shift 2>/dev/null || true
  case "$cmd" in
    signin)  eval "$(op signin)" ;;
    list)    op item list --vault "$@" ;;
    get)     op read "$@" ;;
    add)     _secret_add "$@" ;;
    edit)    _secret_edit "$@" ;;
    name)    _secret_name_cmd "$@" ;;
    envline) _secret_envline_cmd "$@" ;;
    *)
      _secret_usage
      ;;
  esac
}

_secret_usage() {
  cat <<'USAGE'
Usage:
  secret signin
  secret list <vault>
  secret get <op://path>

  secret add [--company|--product|--repo] <VARNAME> <VALUE> [company]
  secret edit [--company|--product|--repo] <VARNAME> <VALUE> [company]
  secret name [--company|--product|--repo] <VARNAME> [company]
  secret envline [--company|--product|--repo] <VARNAME> [company]

Default scope is detected from cwd:
  company dir -> company secret: _company__VARNAME
  product dir -> product secret: <product>__VARNAME
  repo dir    -> repo secret: <product>__<repo>__VARNAME

All items live in the company vault: Work-<company>.
Repo/product .envrc files load scoped items via secret-cache.

Examples:
  secret add --repo OPENAI_API_KEY sk-...
  secret add --product SLACK_WEBHOOK https://...
  secret add --company GITLAB_TOKEN glpat-... neurodesk
  secret name --repo OPENAI_API_KEY
USAGE
}

_secret_parse_scope_args() {
  SECRET_SCOPE="auto"
  SECRET_ARGS=()

  while [ $# -gt 0 ]; do
    case "$1" in
      --company) SECRET_SCOPE="company"; shift ;;
      --product) SECRET_SCOPE="product"; shift ;;
      --repo) SECRET_SCOPE="repo"; shift ;;
      --scope)
        SECRET_SCOPE="${2:-}"
        shift 2
        ;;
      --)
        shift
        while [ $# -gt 0 ]; do
          SECRET_ARGS+=("$1")
          shift
        done
        ;;
      *)
        SECRET_ARGS+=("$1")
        shift
        ;;
    esac
  done

  case "$SECRET_SCOPE" in
    auto|company|product|repo) ;;
    *)
      echo "error: scope must be company, product, or repo" >&2
      return 2
      ;;
  esac
}

_secret_config_value() {
  local file=$1 key=$2
  awk -F':[[:space:]]*' -v key="$key" '$1 == key { print $2; exit }' "$file" 2>/dev/null
}

_secret_metadata_value() {
  local file=$1 key=$2
  awk -F= -v key="$key" '$1 == key { print substr($0, length(key) + 2); exit }' "$file" 2>/dev/null
}

_secret_slug_part() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9._-]+/_/g'
}

_secret_var_name() {
  printf '%s' "$1" | tr '[:lower:]' '[:upper:]'
}

_secret_find_company_dir() {
  local explicit_company="${1:-}"
  local base="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}"
  local dir="$PWD"

  if [ -n "$explicit_company" ]; then
    [ -f "$base/$explicit_company/.company-config" ] && printf '%s\n' "$base/$explicit_company"
    return
  fi

  while [ "$dir" != "/" ] && [ "$dir" != "$HOME" ]; do
    if [ -f "$dir/.company-config" ]; then
      printf '%s\n' "$dir"
      return
    fi
    dir=$(dirname "$dir")
  done
}

_secret_load_context() {
  local explicit_company="${1:-}"
  local base="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}"
  local git_root meta dir

  SECRET_COMPANY=""
  SECRET_PRODUCT=""
  SECRET_REPO=""
  SECRET_COMPANY_DIR=""
  SECRET_PRODUCT_DIR=""
  SECRET_REPO_DIR=""

  git_root=$(git rev-parse --show-toplevel 2>/dev/null || true)
  meta=""
  if [ -n "$git_root" ] && [ -f "$git_root/.agent-workspace" ]; then
    meta="$git_root/.agent-workspace"
  fi

  if [ -n "$meta" ] && [ -z "$explicit_company" ]; then
    SECRET_COMPANY=$(_secret_metadata_value "$meta" company)
    SECRET_PRODUCT=$(_secret_metadata_value "$meta" product)
    SECRET_REPO=$(_secret_metadata_value "$meta" repo)
    SECRET_COMPANY_DIR="$base/$SECRET_COMPANY"
    SECRET_PRODUCT_DIR="$base/$SECRET_COMPANY/$SECRET_PRODUCT"
    SECRET_REPO_DIR="$base/$SECRET_COMPANY/$SECRET_PRODUCT/$SECRET_REPO"
    return
  fi

  SECRET_COMPANY_DIR=$(_secret_find_company_dir "$explicit_company")
  if [ -z "$SECRET_COMPANY_DIR" ]; then
    return
  fi

  SECRET_COMPANY=$(_secret_config_value "$SECRET_COMPANY_DIR/.company-config" slug)
  [ -z "$SECRET_COMPANY" ] && SECRET_COMPANY=$(basename "$SECRET_COMPANY_DIR")

  dir="$PWD"
  while [ "$dir" != "/" ] && [ "$dir" != "$SECRET_COMPANY_DIR" ]; do
    if [ -f "$dir/.product-config" ]; then
      SECRET_PRODUCT_DIR="$dir"
      SECRET_PRODUCT=$(_secret_config_value "$dir/.product-config" slug)
      [ -z "$SECRET_PRODUCT" ] && SECRET_PRODUCT=$(basename "$dir")
      break
    fi
    dir=$(dirname "$dir")
  done

  if [ -n "$git_root" ] && [ -n "$SECRET_PRODUCT_DIR" ]; then
    case "$git_root" in
      "$SECRET_PRODUCT_DIR"/*)
        SECRET_REPO_DIR="$git_root"
        SECRET_REPO=$(basename "$git_root")
        ;;
    esac
  fi
}

_secret_auto_scope() {
  if [ -n "$SECRET_REPO" ]; then
    printf 'repo\n'
  elif [ -n "$SECRET_PRODUCT" ]; then
    printf 'product\n'
  else
    printf 'company\n'
  fi
}

_secret_require_context() {
  local scope=$1

  if [ -z "$SECRET_COMPANY" ]; then
    echo "error: company не определена. Зайди в ~/Desktop/Prokectfiles/<co>/... или передай company последним аргументом" >&2
    return 1
  fi

  case "$scope" in
    product)
      [ -n "$SECRET_PRODUCT" ] || {
        echo "error: product scope требует cwd внутри product или repo" >&2
        return 1
      }
      ;;
    repo)
      [ -n "$SECRET_PRODUCT" ] && [ -n "$SECRET_REPO" ] || {
        echo "error: repo scope требует cwd внутри repo или managed worktree" >&2
        return 1
      }
      ;;
  esac
}

_secret_item_name() {
  local scope=$1 varname=$2
  case "$scope" in
    company)
      printf '_company__%s\n' "$varname"
      ;;
    product)
      printf '%s__%s\n' "$(_secret_slug_part "$SECRET_PRODUCT")" "$varname"
      ;;
    repo)
      printf '%s__%s__%s\n' "$(_secret_slug_part "$SECRET_PRODUCT")" "$(_secret_slug_part "$SECRET_REPO")" "$varname"
      ;;
  esac
}

_secret_envrc_for_scope() {
  local scope=$1
  case "$scope" in
    company) printf '%s/.envrc\n' "$SECRET_COMPANY_DIR" ;;
    product) printf '%s/.envrc\n' "$SECRET_PRODUCT_DIR" ;;
    repo)    printf '%s/.envrc\n' "$SECRET_REPO_DIR" ;;
  esac
}

_secret_envline() {
  local varname=$1 vault=$2 item=$3
  printf 'export %s="$(bash "$HOME/dotfiles/scripts/secret-cache.sh" get %s %s credential)"\n' "$varname" "$vault" "$item"
}

_secret_ensure_envrc() {
  local scope=$1 varname=$2 vault=$3 item=$4
  local envrc dir line

  envrc=$(_secret_envrc_for_scope "$scope")
  dir=$(dirname "$envrc")
  [ -d "$dir" ] || {
    echo "error: envrc directory does not exist: $dir" >&2
    return 1
  }

  touch "$envrc"
  if [ "$scope" != "company" ] && ! grep -q '^source_up$' "$envrc" 2>/dev/null; then
    {
      echo "source_up"
      echo ""
      cat "$envrc"
    } > "${envrc}.tmp" && mv "${envrc}.tmp" "$envrc"
  fi

  line=$(_secret_envline "$varname" "$vault" "$item")
  if ! grep -qE "^export ${varname}=" "$envrc"; then
    {
      echo ""
      echo "# Loaded from 1Password item: op://${vault}/${item}/credential"
      echo "$line"
    } >> "$envrc"
    echo "✓ Добавлено в $envrc"
    echo "  Активировать: cd $dir && direnv allow"
  else
    echo "→ $envrc уже содержит export ${varname}="
  fi
}

_secret_prepare() {
  local min_args=$1
  shift

  _secret_parse_scope_args "$@" || return $?
  if [ ${#SECRET_ARGS[@]} -lt "$min_args" ]; then
    _secret_usage
    return 2
  fi

  SECRET_VARNAME=$(_secret_var_name "${SECRET_ARGS[1]}")
  if [ "$min_args" -eq 1 ]; then
    SECRET_VALUE=""
    SECRET_EXPLICIT_COMPANY="${SECRET_ARGS[2]:-}"
  else
    SECRET_VALUE="${SECRET_ARGS[2]:-}"
    SECRET_EXPLICIT_COMPANY="${SECRET_ARGS[3]:-}"
  fi

  _secret_load_context "$SECRET_EXPLICIT_COMPANY"
  [ "$SECRET_SCOPE" = "auto" ] && SECRET_SCOPE=$(_secret_auto_scope)
  _secret_require_context "$SECRET_SCOPE" || return $?

  SECRET_VAULT="Work-$SECRET_COMPANY"
  SECRET_ITEM=$(_secret_item_name "$SECRET_SCOPE" "$SECRET_VARNAME")
}

_secret_add() {
  _secret_prepare 2 "$@" || return $?

  op vault create "$SECRET_VAULT" >/dev/null 2>&1 || true

  if op item get "$SECRET_ITEM" --vault "$SECRET_VAULT" >/dev/null 2>&1; then
    echo "⚠ Item '$SECRET_ITEM' уже в '$SECRET_VAULT'. Используй: secret edit --$SECRET_SCOPE $SECRET_VARNAME <value>"
    return 1
  fi

  if op item create --category=apicredential --vault="$SECRET_VAULT" \
       --title="$SECRET_ITEM" credential="$SECRET_VALUE" >/dev/null 2>&1; then
    echo "✓ Создан op://$SECRET_VAULT/$SECRET_ITEM"
  else
    echo "⚠ Не удалось создать item"
    return 1
  fi

  _secret_ensure_envrc "$SECRET_SCOPE" "$SECRET_VARNAME" "$SECRET_VAULT" "$SECRET_ITEM"
}

_secret_edit() {
  _secret_prepare 2 "$@" || return $?

  if op item edit "$SECRET_ITEM" --vault="$SECRET_VAULT" credential="$SECRET_VALUE" >/dev/null 2>&1; then
    echo "✓ Обновлён op://$SECRET_VAULT/$SECRET_ITEM"
  else
    echo "⚠ Item '$SECRET_ITEM' не найден"
    return 1
  fi

  _secret_ensure_envrc "$SECRET_SCOPE" "$SECRET_VARNAME" "$SECRET_VAULT" "$SECRET_ITEM"
}

_secret_name_cmd() {
  _secret_prepare 1 "$@" || return $?
  echo "$SECRET_ITEM"
}

_secret_envline_cmd() {
  _secret_prepare 1 "$@" || return $?
  _secret_envline "$SECRET_VARNAME" "$SECRET_VAULT" "$SECRET_ITEM"
}
