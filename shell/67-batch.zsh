# shell function `batch-setup` — autonomous architectural analysis for whole product
#
# Target: ~/dotfiles/shell/67-batch.zsh
#
# ### Agent context setup
#
# ## batch-setup
# **Что:** Autonomously runs analyze-repo per repo + analyze-product. С heartbeat'ами.
# **Запуск:**
# - `batch-setup` — current dir, interactive confirm
# - `batch-setup --auto` — skip confirm (для setup-context)
# - `batch-setup <co> <prod>` — cd to product first
# - `batch-setup --reasoning xhigh` — override default high
# - `batch-setup --model gpt-5.4-codex-max` — override model
# **Default:** model=gpt-5.4, reasoning=high
# **Логи:** ~/dotfiles/logs/batch-setup-<co>-<prod>-<ts>.log

batch-setup() {
  local auto=0
  local model="gpt-5.4"
  local reasoning="high"
  local args=()
  while [ $# -gt 0 ]; do
    case "$1" in
      --auto|-y) auto=1; shift ;;
      --model) model="$2"; shift 2 ;;
      --reasoning) reasoning="$2"; shift 2 ;;
      *) args+=("$1"); shift ;;
    esac
  done

  local target
  case ${#args[@]} in
    0) target="$PWD" ;;
    2) target="$HOME/Desktop/Prokectfiles/${args[1]}/${args[2]}" ;;
    *) echo "Usage: batch-setup [--auto] [--model M] [--reasoning R] [<company> <product>]"; return 1 ;;
  esac

  if [ ! -d "$target" ]; then
    echo "⚠ Path not found: $target"
    return 1
  fi
  cd "$target" || return 1

  if [ ! -f .product-config ]; then
    echo "⚠ Not a product directory"
    return 1
  fi

  # Enumerate repos
  local repos=()
  for d in */; do
    repo=$(basename "$d")
    [ -d "$d/.git" ] && repos+=("$repo")
  done

  if [ ${#repos[@]} -eq 0 ]; then
    echo "⚠ No git repos found in $(pwd)"
    return 1
  fi

  # Setup log
  local co=$(basename "$(dirname "$(pwd)")")
  local prod=$(basename "$(pwd)")
  local ts=$(date +%Y%m%d-%H%M%S)
  local logdir=~/dotfiles/logs
  mkdir -p "$logdir"
  local logfile="$logdir/batch-setup-${co}-${prod}-${ts}.log"

  # Force correct codex/claude home BEFORE anything
  export CODEX_HOME="$HOME/.codex-new"
  export CLAUDE_CONFIG_DIR="$HOME/.claude-new"

  echo "→ Batch architectural analysis for $(pwd)"
  echo "  Repos found: ${repos[*]}"
  echo "  Plan: ${#repos[@]} × analyze-repo + 1 × analyze-product"
  echo "  Model: $model"
  echo "  Reasoning: $reasoning"
  echo "  CODEX_HOME: $CODEX_HOME"
  echo "  Log file: $logfile  (output mirrored here, file kept for review later)"
  echo ""

  if [ $auto -eq 0 ]; then
    echo -n "Start? (y/N): "
    read -k 1 confirm
    echo
    [[ ! "$confirm" =~ [yY] ]] && echo "Aborted." && return 0
  else
    echo "→ Auto mode — starting immediately"
  fi

  local start_time=$(date +%s)
  local product_root="$PWD"

  # Helper — heartbeat during codex run (every 20s)
  _heartbeat() {
    local label=$1
    local docs_dir=$2
    while true; do
      sleep 20
      local now=$(date '+%H:%M:%S')
      local msg="  [$now] $label still working..."
      if [ -d "$docs_dir" ]; then
        local fcount=$(find "$docs_dir" -type f 2>/dev/null | wc -l | tr -d ' ')
        local lcount=0
        [ -f "$docs_dir/design.md" ] && lcount=$(wc -l < "$docs_dir/design.md" | tr -d ' ')
        [ -f "$docs_dir/ARCHITECTURE.md" ] && lcount=$(wc -l < "$docs_dir/ARCHITECTURE.md" | tr -d ' ')
        msg+=" [files:$fcount, lines:$lcount]"
      fi
      echo "$msg" | tee -a "$logfile"
    done
  }

  echo "===============================================" | tee -a "$logfile"
  echo "Batch setup started at $(date '+%Y-%m-%d %H:%M:%S')" | tee -a "$logfile"
  echo "Product: $co/$prod" | tee -a "$logfile"
  echo "Repos: ${repos[*]}" | tee -a "$logfile"
  echo "Model: $model, Reasoning: $reasoning" | tee -a "$logfile"
  echo "===============================================" | tee -a "$logfile"

  # Phase 1: analyze each repo
  echo "" | tee -a "$logfile"
  echo "=== Phase 1: analyze repos ===" | tee -a "$logfile"
  local i=1
  for repo in "${repos[@]}"; do
    local phase_start=$(date +%s)
    echo "" | tee -a "$logfile"
    echo "[$i/${#repos[@]}] Analyzing $repo at $(date '+%H:%M:%S')..." | tee -a "$logfile"

    # Start heartbeat in background
    _heartbeat "analyze-repo:$repo" "$product_root/$repo/docs" &
    local watcher=$!

    (
      cd "$product_root/$repo" || exit 1
      codex exec \
        --sandbox workspace-write \
        --skip-git-repo-check \
        --model "$model" \
        -c "model_reasoning_effort=\"$reasoning\"" \
        "Use skill analyze-repo. cwd is repo root. Read code, manifest, README, recent git log. Generate docs/design.md and propose docs/adr/ files. AUTO-SAVE drafts — don't ask for confirmation. Mark uncertain inferences with 'TODO confirm with user'. Be honest about limits." \
        2>&1 | tee -a "$logfile"
    )

    # Stop heartbeat
    kill $watcher 2>/dev/null
    wait $watcher 2>/dev/null

    local phase_end=$(date +%s)
    echo "  ✓ $repo done in $((phase_end - phase_start))s" | tee -a "$logfile"
    i=$((i+1))
  done

  # Phase 2: analyze product
  echo "" | tee -a "$logfile"
  echo "=== Phase 2: analyze product ===" | tee -a "$logfile"
  echo "Reading all ${#repos[@]} repos for inter-service mapping at $(date '+%H:%M:%S')..." | tee -a "$logfile"

  _heartbeat "analyze-product" "$product_root/docs" &
  local watcher=$!

  local phase_start=$(date +%s)
  (
    cd "$product_root" || exit 1
    codex exec \
      --sandbox workspace-write \
      --skip-git-repo-check \
      --model "$model" \
      -c "model_reasoning_effort=\"$reasoning\"" \
      "Use skill analyze-product. cwd is product dir with subdirectories = repos. Read all repos, identify services + inter-service communication + data flow. Generate docs/ARCHITECTURE.md and propose docs/adr/. AUTO-SAVE — don't ask. Mark uncertainties as 'TODO confirm with user'." \
      2>&1 | tee -a "$logfile"
  )

  kill $watcher 2>/dev/null
  wait $watcher 2>/dev/null

  local phase_end=$(date +%s)
  echo "  ✓ Product analysis done in $((phase_end - phase_start))s" | tee -a "$logfile"

  local total=$(($(date +%s) - start_time))
  echo "" | tee -a "$logfile"
  echo "===============================================" | tee -a "$logfile"
  echo "✅ Batch setup complete in $((total / 60))m $((total % 60))s" | tee -a "$logfile"
  echo "===============================================" | tee -a "$logfile"
  echo "" | tee -a "$logfile"
  echo "Generated files:" | tee -a "$logfile"
  [ -f "$product_root/docs/ARCHITECTURE.md" ] && echo "  ✓ $product_root/docs/ARCHITECTURE.md ($(wc -l < $product_root/docs/ARCHITECTURE.md | tr -d ' ') lines)" | tee -a "$logfile"
  [ -d "$product_root/docs/adr" ] && echo "  ✓ $product_root/docs/adr/ ($(ls $product_root/docs/adr 2>/dev/null | wc -l | tr -d ' ') ADRs)" | tee -a "$logfile"
  for repo in "${repos[@]}"; do
    if [ -f "$product_root/$repo/docs/design.md" ]; then
      echo "  ✓ $product_root/$repo/docs/design.md ($(wc -l < $product_root/$repo/docs/design.md | tr -d ' ') lines)" | tee -a "$logfile"
    fi
    [ -d "$product_root/$repo/docs/adr" ] && echo "    └─ ADRs: $(ls $product_root/$repo/docs/adr 2>/dev/null | wc -l | tr -d ' ')" | tee -a "$logfile"
  done
  echo "" | tee -a "$logfile"
  echo "Full log: $logfile" | tee -a "$logfile"
}
