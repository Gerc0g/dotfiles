# shell function `fill-agents-md` + `batch-fill` — заполнение AGENTS.md из design.md/ARCHITECTURE.md
#
# Target: ~/dotfiles/shell/70-fill-agents.zsh
#
# ### Agent context setup
#
# ## fill-agents-md
# **Что:** Заполнить AGENTS.md TODOs из готовых docs/design.md / docs/ARCHITECTURE.md.
# Per-repo или per-product, AUTO-SAVE.
# **Запуск:**
# - `fill-agents-md` — current dir (repo или product, auto-detect)
# - `fill-agents-md <co> <prod>` — product level
# - `fill-agents-md <co> <prod> <repo>` — repo level
# **Зависит от:** codex CLI, ~/dotfiles/skills/fill-agents-md/SKILL.md, существующие docs/
# **См. также:** `analyze-repo`, `analyze-product`, `batch-fill-agents`
#
# ## batch-fill-agents
# **Что:** Параллельный fill-agents-md для всех репов + всех продуктов компании.
# Запускает codex exec на каждый AGENTS.md одновременно. ETA ~5-15 мин.
# **Запуск:**
# - `batch-fill-agents` — current dir (должен быть product)
# - `batch-fill-agents <co>` — для всей компании (все продукты + все репы)
# - `batch-fill-agents <co> <prod>` — для одного продукта (product + все репы)
# **Логи:** ~/dotfiles/logs/fill-agents-<co>-<ts>.log

fill-agents-md() {
  local target
  case $# in
    0) target="$PWD" ;;
    2) target="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$1/$2" ;;
    3) target="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}/$1/$2/$3" ;;
    *) echo "Usage: fill-agents-md [<co> <prod>] | [<co> <prod> <repo>]"; return 1 ;;
  esac

  [ ! -d "$target" ] && { echo "⚠ Not a directory: $target"; return 1; }
  cd "$target" || return 1

  if [ ! -f AGENTS.md ]; then
    echo "⚠ No AGENTS.md at $(pwd)"
    return 1
  fi

  # Detect level
  if [ -d .git ]; then
    [ ! -f docs/design.md ] && { echo "⚠ No docs/design.md — run analyze-repo first"; return 1; }
    local level="repo"
  elif [ -f .product-config ]; then
    [ ! -f docs/ARCHITECTURE.md ] && { echo "⚠ No docs/ARCHITECTURE.md — run analyze-product first"; return 1; }
    local level="product"
  elif [ -f .company-config ]; then
    echo "⚠ Company AGENTS.md — use 'complete-onboard $(basename $(pwd))' instead"
    echo "  (Stance, PII, Network policy are human-only decisions, not derivable from code)"
    return 1
  else
    echo "⚠ Cannot detect level — no .git, .product-config, .company-config"
    return 1
  fi

  export CODEX_HOME="$HOME/.codex"
  export CLAUDE_CONFIG_DIR="$HOME/.claude"

  echo "→ fill-agents-md at $(pwd)  [level: $level]"

  codex exec \
    --sandbox workspace-write \
    --skip-git-repo-check \
    --model gpt-5.4 \
    -c "model_reasoning_effort=\"high\"" \
    "Use skill fill-agents-md. cwd is $level root. Read AGENTS.md (target with TODOs) and docs/$([ "$level" = "repo" ] && echo "design.md" || echo "ARCHITECTURE.md") (source of facts). Fill all TODO sections in AGENTS.md. AUTO-SAVE — don't ask. Preserve Context hierarchy table and Lessons section verbatim."
}

batch-fill-agents() {
  local model="gpt-5.4"
  local reasoning="high"
  local target_co target_prod
  local args=()

  while [ $# -gt 0 ]; do
    case "$1" in
      --model) model="$2"; shift 2 ;;
      --reasoning) reasoning="$2"; shift 2 ;;
      *) args+=("$1"); shift ;;
    esac
  done

  local base=${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}
  case ${#args[@]} in
    0)
      # current dir = product
      [ -f .product-config ] || { echo "⚠ cwd is not a product dir (no .product-config)"; return 1; }
      target_co=$(basename "$(dirname "$(pwd)")")
      target_prod=$(basename "$(pwd)")
      ;;
    1)
      # company scope — all products
      target_co="${args[1]}"
      [ ! -d "$base/$target_co" ] && { echo "⚠ Company not found: $target_co"; return 1; }
      ;;
    2)
      target_co="${args[1]}"
      target_prod="${args[2]}"
      [ ! -d "$base/$target_co/$target_prod" ] && { echo "⚠ Product not found"; return 1; }
      ;;
    *)
      echo "Usage: batch-fill-agents [<co>] [<prod>]"
      return 1
      ;;
  esac

  # Build target list: product dirs + repo dirs
  local -a targets   # absolute paths
  local -a labels    # short labels for logging

  _add_product_targets() {
    local co=$1 prod=$2
    local pdir="$base/$co/$prod"
    [ ! -d "$pdir" ] && return

    # product itself (only if has AGENTS.md + docs/ARCHITECTURE.md)
    if [ -f "$pdir/AGENTS.md" ] && [ -f "$pdir/docs/ARCHITECTURE.md" ]; then
      targets+=("$pdir")
      labels+=("$co/$prod (product)")
    fi

    # all repos inside
    for rdir in "$pdir"/*/; do
      [ -d "$rdir/.git" ] || continue
      [ -f "$rdir/AGENTS.md" ] || continue
      [ -f "$rdir/docs/design.md" ] || continue
      targets+=("${rdir%/}")
      labels+=("$co/$prod/$(basename ${rdir%/})")
    done
  }

  if [ -n "$target_prod" ]; then
    _add_product_targets "$target_co" "$target_prod"
  else
    # all products
    for pdir in "$base/$target_co"/*/; do
      [ -f "$pdir/.product-config" ] || continue
      _add_product_targets "$target_co" "$(basename ${pdir%/})"
    done
  fi

  if [ ${#targets[@]} -eq 0 ]; then
    echo "⚠ No targets found (no AGENTS.md+docs/design.md/ARCHITECTURE.md combos)"
    return 1
  fi

  # Setup log
  local ts=$(date +%Y%m%d-%H%M%S)
  local logdir=$HOME/dotfiles/logs
  mkdir -p "$logdir"
  local logfile="$logdir/fill-agents-${target_co}${target_prod:+-$target_prod}-${ts}.log"

  export CODEX_HOME="$HOME/.codex"
  export CLAUDE_CONFIG_DIR="$HOME/.claude"

  echo "→ batch-fill-agents"
  echo "  Company: $target_co${target_prod:+ / $target_prod}"
  echo "  Targets: ${#targets[@]} (products + repos)"
  echo "  Model: $model, reasoning: $reasoning"
  echo "  Log: $logfile"
  echo ""
  echo "  Targets:"
  local i=1
  for label in "${labels[@]}"; do
    echo "    [$i/${#labels[@]}] $label"
    i=$((i+1))
  done
  echo ""
  echo "→ Launching all in parallel..."

  local start=$(date +%s)
  local pids=()
  i=1
  for tgt in "${targets[@]}"; do
    local label="${labels[$i]}"

    # Detect level
    local level prompt_doc
    if [ -d "$tgt/.git" ]; then
      level="repo"
      prompt_doc="docs/design.md"
    else
      level="product"
      prompt_doc="docs/ARCHITECTURE.md"
    fi

    (
      cd "$tgt" || exit 1
      {
        echo ""
        echo "═══ [$label] starting at $(date '+%H:%M:%S') ═══"
        codex exec \
          --sandbox workspace-write \
          --skip-git-repo-check \
          --model "$model" \
          -c "model_reasoning_effort=\"$reasoning\"" \
          "Use skill fill-agents-md. cwd is $level root. Read AGENTS.md (target with TODOs) and $prompt_doc (source of facts). Fill all TODO sections in AGENTS.md. AUTO-SAVE — don't ask. Preserve Context hierarchy table and Lessons section verbatim."
        echo ""
        echo "═══ [$label] done at $(date '+%H:%M:%S') ═══"
      } >> "$logfile" 2>&1
    ) &
    pids+=($!)
    i=$((i+1))
  done

  echo "  ${#pids[@]} parallel jobs launched."
  echo "  Watch: tail -F $logfile  OR  agents-sessions ${target_co}"
  echo "  Waiting for all to finish..."
  echo ""

  i=1
  for pid in "${pids[@]}"; do
    wait "$pid"
    local rc=$?
    if [ $rc -eq 0 ]; then
      echo "  ✓ [$i/${#pids[@]}] ${labels[$i]}"
    else
      echo "  ✗ [$i/${#pids[@]}] ${labels[$i]} (rc=$rc)"
    fi
    i=$((i+1))
  done

  local total=$(($(date +%s) - start))
  echo ""
  echo "==============================================="
  echo "✅ batch-fill-agents complete in $((total/60))m $((total%60))s"
  echo "==============================================="
  echo ""
  echo "Verify: grep -rl 'TODO:' $base/$target_co${target_prod:+/$target_prod}/*/AGENTS.md  (should be empty)"
  echo "Full log: $logfile"
}
