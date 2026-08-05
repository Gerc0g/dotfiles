# shell functions: refresh-templates, setup-context, complete-onboard
#
# Target: ~/dotfiles/shell/68-setup-flow.zsh
#
# ### Setup flow
#
# ## refresh-templates
# **Что:** Перегенерирует AGENTS.md из текущих templates (overwrites existing).
#          Только AGENTS.md, без architectural analysis. Быстро.
# **Запуск:**
# - `refresh-templates <co>` — company + все products + все repos
# - `refresh-templates <co> <prod>` — product + все repos
# - `refresh-templates <co> <prod> <repo>` — один репо
# **Зависит от:** ~/dotfiles/templates/AGENTS.md.{company,product,repo}.tmpl
# **Время:** ~5 секунд
#
# ## setup-context
# **Что:** refresh-templates + analyze (batch-setup для продукта, analyze-repo для репа).
#          Autonomous: ты пишешь `y` в начале, дальше codex сам.
# **Запуск:** идентично refresh-templates
# **Время:** ~30-60 минут на продукт unattended
#
# ## complete-onboard
# **Что:** Интерактивный onboard chain. Проходит company → каждый product → каждый repo.
# **Запуск:** идентично refresh-templates
# **Время:** ~80-120 минут interactive (на твоей стороне)

# ---------------------------------------------------------------------
# Helpers — read .company-config / .product-config and regenerate AGENTS.md
# ---------------------------------------------------------------------

_setup_load_company_vars() {
  local co=$1
  local cfg="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$co/.company-config"
  [ -f "$cfg" ] || { echo "⚠ Missing $cfg"; return 1; }

  export CO="$co"
  export CO_TITLE="$(echo "$co" | awk '{print toupper(substr($0,1,1)) substr($0,2)}')"
  export VCS=$(awk '/^vcs:/ {print $2}' "$cfg")
  export HOST=$(awk '/^host:/ {print $2}' "$cfg")
  export NS=$(awk '/^namespace:/ {print $2}' "$cfg")
  export SSH_HOST=$(awk '/^ssh_host:/ {print $2}' "$cfg")
  export VAULT="Work-$co"
  export EMAIL=$(awk '/^git_email:/ {print $2}' "$cfg")
  [ -z "$EMAIL" ] && EMAIL="egor@$co"
}

_setup_regen_company() {
  local co=$1
  _setup_load_company_vars "$co" || return 1
  envsubst '$CO $CO_TITLE $VCS $HOST $NS $SSH_HOST $VAULT $EMAIL' \
    < ~/dotfiles/templates/AGENTS.md.company.tmpl \
    > "${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$co/AGENTS.md"
  echo "  ✓ $co/AGENTS.md"
}

_setup_regen_product() {
  local co=$1 prod=$2
  CO="$co" PROD="$prod" envsubst '$CO $PROD' \
    < ~/dotfiles/templates/AGENTS.md.product.tmpl \
    > "${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$co/$prod/AGENTS.md"
  echo "  ✓ $co/$prod/AGENTS.md"
}

_setup_regen_repo() {
  local co=$1 prod=$2 repo=$3
  CO="$co" PROD="$prod" REPO="$repo" envsubst '$CO $PROD $REPO' \
    < ~/dotfiles/templates/AGENTS.md.repo.tmpl \
    > "${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$co/$prod/$repo/AGENTS.md"
  echo "  ✓ $co/$prod/$repo/AGENTS.md"
}

_setup_iter_products() {
  # echo all products (subdirs with .product-config) for given company
  local co=$1
  for d in "${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$co"/*/; do
    [ -f "$d/.product-config" ] && basename "$d"
  done
}

_setup_iter_repos() {
  # echo all repos (subdirs with .git) for given company/product
  local co=$1 prod=$2
  for d in "${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$co/$prod"/*/; do
    [ -d "$d/.git" ] && basename "$d"
  done
}

# ---------------------------------------------------------------------
# refresh-templates — only AGENTS.md regen, fast, no codex
# ---------------------------------------------------------------------

refresh-templates() {
  local co=$1 prod=$2 repo=$3
  local base=${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}

  if [ -z "$co" ]; then
    echo "Usage: refresh-templates <co> [<prod>] [<repo>]"
    return 1
  fi
  if [ ! -d "$base/$co" ]; then
    echo "⚠ Company '$co' not found at $base/$co"
    return 1
  fi

  # Determine scope by argc
  local scope="company"
  [ -n "$prod" ] && scope="product"
  [ -n "$repo" ] && scope="repo"

  echo "→ refresh-templates: scope=$scope"
  case $scope in
    company)
      echo "  Will overwrite: company AGENTS.md + all products + all repos"
      echo "  Products: $(_setup_iter_products $co | tr '\n' ' ')"
      ;;
    product)
      echo "  Will overwrite: $co/$prod/AGENTS.md + all repos in product"
      echo "  Repos: $(_setup_iter_repos $co $prod | tr '\n' ' ')"
      ;;
    repo)
      echo "  Will overwrite: $co/$prod/$repo/AGENTS.md"
      ;;
  esac
  echo -n "Continue? (y/N): "
  read -k 1 c; echo
  [[ ! "$c" =~ [yY] ]] && echo "Aborted." && return 0

  case $scope in
    company)
      _setup_regen_company "$co"
      for p in $(_setup_iter_products $co); do
        _setup_regen_product "$co" "$p"
        for r in $(_setup_iter_repos $co $p); do
          _setup_regen_repo "$co" "$p" "$r"
        done
      done
      ;;
    product)
      _setup_regen_product "$co" "$prod"
      for r in $(_setup_iter_repos $co $prod); do
        _setup_regen_repo "$co" "$prod" "$r"
      done
      ;;
    repo)
      _setup_regen_repo "$co" "$prod" "$repo"
      ;;
  esac

  echo "✓ Templates refreshed"
}

# ---------------------------------------------------------------------
# setup-context — refresh + autonomous architectural analysis
# ---------------------------------------------------------------------

setup-context() {
  local co=$1 prod=$2 repo=$3
  local base=${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}

  if [ -z "$co" ]; then
    echo "Usage: setup-context <co> [<prod>] [<repo>]"
    return 1
  fi
  if [ ! -d "$base/$co" ]; then
    echo "⚠ Company '$co' not found"
    return 1
  fi

  local scope="company"
  [ -n "$prod" ] && scope="product"
  [ -n "$repo" ] && scope="repo"

  echo "→ setup-context: scope=$scope"
  echo "  This will:"
  echo "    1. Refresh ALL AGENTS.md in scope from templates (overwrite)"
  case $scope in
    company)
      local prod_count=$(_setup_iter_products $co | wc -l | tr -d ' ')
      echo "    2. Run batch-setup for each of $prod_count products (autonomous, codex analyzes code)"
      echo "  ETA: 30-60 min × $prod_count products = $((30 * prod_count))-$((60 * prod_count)) min"
      ;;
    product)
      local repo_count=$(_setup_iter_repos $co $prod | wc -l | tr -d ' ')
      echo "    2. Run batch-setup for $repo_count repos (autonomous, codex analyzes code)"
      echo "  ETA: 30-60 min for $repo_count repos"
      ;;
    repo)
      echo "    2. Run analyze-repo (autonomous, codex analyzes code)"
      echo "  ETA: 5-15 min"
      ;;
  esac
  echo -n "Continue? (y/N): "
  read -k 1 c; echo
  [[ ! "$c" =~ [yY] ]] && echo "Aborted." && return 0

  export CODEX_HOME="$HOME/.codex"
  export CLAUDE_CONFIG_DIR="$HOME/.claude"

  local start=$(date +%s)

  case $scope in
    company)
      # Refresh all
      _setup_regen_company "$co"
      for p in $(_setup_iter_products $co); do
        _setup_regen_product "$co" "$p"
        for r in $(_setup_iter_repos $co $p); do
          _setup_regen_repo "$co" "$p" "$r"
        done
      done
      echo "  ✓ All AGENTS.md regenerated"
      echo ""
      # Batch-setup each product IN PARALLEL — каждый продукт в фоне, со своим логом
      echo "=== Launching batch-setup in parallel for all products ==="
      local pids=()
      local prods=()
      for p in $(_setup_iter_products $co); do
        echo "  → $co/$p (background)"
        (cd "$base/$co/$p" && batch-setup --auto > /dev/null 2>&1) &
        pids+=($!)
        prods+=("$p")
      done
      echo ""
      echo "  ${#pids[@]} batch-setup processes started."
      echo "  Live dashboard: agents-sessions $co"
      echo "  Detailed logs: ~/dotfiles/logs/batch-setup-${co}-*.log"
      echo ""
      echo "  Waiting for all products to finish..."
      local i=1
      for pid in "${pids[@]}"; do
        wait "$pid"
        local rc=$?
        if [ $rc -eq 0 ]; then
          echo "  ✓ [$i/${#pids[@]}] ${prods[$i]} done"
        else
          echo "  ✗ [$i/${#pids[@]}] ${prods[$i]} failed (rc=$rc)"
        fi
        i=$((i+1))
      done

      # Phase 3: condense design.md / ARCHITECTURE.md → AGENTS.md sections
      echo ""
      echo "=== Phase 3: fill AGENTS.md from generated docs ==="
      batch-fill-agents "$co"
      ;;
    product)
      _setup_regen_product "$co" "$prod"
      for r in $(_setup_iter_repos $co $prod); do
        _setup_regen_repo "$co" "$prod" "$r"
      done
      echo "  ✓ Templates refreshed"
      echo ""
      (cd "$base/$co/$prod" && batch-setup --auto 2>&1 | sed 's/^/  /')

      echo ""
      echo "=== Final: fill AGENTS.md from generated docs ==="
      batch-fill-agents "$co" "$prod"
      ;;
    repo)
      _setup_regen_repo "$co" "$prod" "$repo"
      echo "  ✓ Template refreshed"
      echo ""
      (cd "$base/$co/$prod/$repo" && analyze-repo 2>&1 | sed 's/^/  /')

      echo ""
      echo "=== Final: fill AGENTS.md from generated docs ==="
      (cd "$base/$co/$prod/$repo" && fill-agents-md 2>&1 | sed 's/^/  /')
      ;;
  esac

  local total=$(($(date +%s) - start))
  echo ""
  echo "==============================================="
  echo "✅ setup-context complete in $((total/60))m $((total%60))s"
  echo "==============================================="
  echo ""
  echo "Next: run interactive onboarding"
  case $scope in
    company)
      echo "  complete-onboard $co"
      ;;
    product)
      echo "  complete-onboard $co $prod"
      ;;
    repo)
      echo "  onboard $co $prod $repo"
      ;;
  esac
}

# ---------------------------------------------------------------------
# complete-onboard — interactive onboard chain through all levels in scope
# ---------------------------------------------------------------------

complete-onboard() {
  local co=$1 prod=$2 repo=$3
  local base=${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}

  if [ -z "$co" ]; then
    echo "Usage: complete-onboard <co> [<prod>] [<repo>]"
    return 1
  fi
  if [ ! -d "$base/$co" ]; then
    echo "⚠ Company '$co' not found"
    return 1
  fi

  local scope="company"
  [ -n "$prod" ] && scope="product"
  [ -n "$repo" ] && scope="repo"

  echo "→ complete-onboard: scope=$scope"
  case $scope in
    company)
      local prod_count=$(_setup_iter_products $co | wc -l | tr -d ' ')
      local repo_count=0
      for p in $(_setup_iter_products $co); do
        repo_count=$((repo_count + $(_setup_iter_repos $co $p | wc -l | tr -d ' ')))
      done
      echo "  Will interactively onboard:"
      echo "    - 1× company AGENTS.md ($co)"
      echo "    - $prod_count× product AGENTS.md"
      echo "    - $repo_count× repo AGENTS.md"
      echo "  ETA: ~$((10 + 10*prod_count + 5*repo_count)) минут interactive"
      ;;
    product)
      local repo_count=$(_setup_iter_repos $co $prod | wc -l | tr -d ' ')
      echo "  Will onboard: 1× product + $repo_count× repos"
      echo "  ETA: ~$((10 + 5*repo_count)) минут interactive"
      ;;
    repo)
      echo "  Will onboard: 1× repo"
      echo "  ETA: ~3-5 minutes"
      ;;
  esac
  echo -n "Start? (y/N): "
  read -k 1 c; echo
  [[ ! "$c" =~ [yY] ]] && echo "Aborted." && return 0

  case $scope in
    company)
      echo ""
      echo "=== Onboard COMPANY: $co ==="
      onboard $co
      for p in $(_setup_iter_products $co); do
        echo ""
        echo "=== Onboard PRODUCT: $co/$p ==="
        onboard $co $p
        for r in $(_setup_iter_repos $co $p); do
          echo ""
          echo "=== Onboard REPO: $co/$p/$r ==="
          onboard $co $p $r
        done
      done
      ;;
    product)
      onboard $co $prod
      for r in $(_setup_iter_repos $co $prod); do
        echo ""
        echo "=== Onboard REPO: $co/$prod/$r ==="
        onboard $co $prod $r
      done
      ;;
    repo)
      onboard $co $prod $repo
      ;;
  esac

  echo ""
  echo "==============================================="
  echo "✅ complete-onboard finished"
  echo "==============================================="
  echo ""
  echo "Review changes:"
  case $scope in
    company)
      echo "  cd $base/$co && find . -name AGENTS.md | head -20"
      ;;
    *) echo "  git status в каждом изменённом репо" ;;
  esac
}
