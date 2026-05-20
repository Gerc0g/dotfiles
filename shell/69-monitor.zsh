# shell function `agents-status` + `agents-watch` — monitoring setup-context / batch-setup runs
#
# Target: ~/dotfiles/shell/69-monitor.zsh
#
# ### Agent context setup
#
# ## agents-status
# **Что:** Snapshot текущего состояния анализа: codex процессы, последний лог, прогресс по docs/.
# **Запуск:** `agents-status` или `agents-status <co>` (только эта компания)
#
# ## agents-watch
# **Что:** Live мониторинг с авто-обновлением (refresh каждые 10 секунд).
# **Запуск:** `agents-watch` или `agents-watch <co>`
# **Выход:** Ctrl+C
#
# ## agents-tail
# **Что:** Live tail последнего batch-setup лога — видишь streaming output codex (tool calls, reasoning, edits).
# **Запуск:** `agents-tail` или `agents-tail <co>`
# **Выход:** Ctrl+C (batch-setup продолжит работать в фоне)
#
# ## agents-sessions
# **Что:** Dashboard всех живых codex сессий. Видно PID, skill, репо, elapsed, status. Сессии исчезают по мере завершения.
# **Запуск:** `agents-sessions` или `agents-sessions <co>`
# **Выход:** Ctrl+C (refresh каждые 3 сек)

agents-status() {
  local co_filter="$1"
  local base=$HOME/Desktop/Prokectfiles

  echo "═══════════════════════════════════════════════════"
  echo " Agent Analysis Status  ($(date '+%H:%M:%S'))"
  echo "═══════════════════════════════════════════════════"

  # 1. Active codex (excluding VSCode / ChatGPT app)
  echo ""
  echo "▸ Active codex sessions:"
  local codex_count
  codex_count=$(pgrep -fl codex 2>/dev/null | awk '!/vscode|chatgpt|app-server|--login|--help/ {print $1}' | wc -l | tr -d ' ')
  if [ "$codex_count" -eq 0 ]; then
    echo "  (none — analysis not running)"
  else
    pgrep -fl codex 2>/dev/null | awk '!/vscode|chatgpt|app-server|--login|--help/' | head -5 | awk '{
      pid=$1; cmd=$2;
      for(i=3; i<=NF && i<8; i++) cmd=cmd" "$i;
      printf "  PID %s: %s\n", pid, cmd
    }'
  fi

  # 2. batch-setup process
  echo ""
  echo "▸ batch-setup process:"
  local batch_pid
  batch_pid=$(pgrep -f "batch-setup" 2>/dev/null | head -1)
  if [ -n "$batch_pid" ]; then
    echo "  PID $batch_pid running"
  else
    echo "  (none)"
  fi

  # 3. Latest log
  echo ""
  echo "▸ Latest log file:"
  local latest
  if [ -n "$co_filter" ]; then
    latest=$(ls -t ~/dotfiles/logs/batch-setup-${co_filter}-*.log 2>/dev/null | head -1)
  else
    latest=$(ls -t ~/dotfiles/logs/batch-setup-*.log 2>/dev/null | head -1)
  fi
  if [ -n "$latest" ]; then
    local log_size=$(stat -f '%z' "$latest" 2>/dev/null | awk '{printf "%.1fk", $1/1024}')
    local log_age=$(stat -f '%Sm' -t '%H:%M:%S' "$latest" 2>/dev/null)
    echo "  $(basename $latest)  [$log_size, modified $log_age]"
    echo ""
    echo "  Last 8 lines:"
    tail -8 "$latest" 2>/dev/null | sed 's/^/    /'
  else
    echo "  (no logs found)"
  fi

  # 4. Generated docs counts (per company)
  echo ""
  echo "▸ Generated docs:"
  for co_dir in "$base"/*/; do
    [ -f "$co_dir/.company-config" ] || continue
    local co=$(basename "$co_dir")
    [ -n "$co_filter" ] && [ "$co" != "$co_filter" ] && continue

    local arch_count=$(find "$co_dir" -maxdepth 3 -name "ARCHITECTURE.md" 2>/dev/null | wc -l | tr -d ' ')
    local design_count=$(find "$co_dir" -maxdepth 4 -name "design.md" 2>/dev/null | wc -l | tr -d ' ')
    local adr_count=$(find "$co_dir" -maxdepth 4 -path "*/docs/adr/*.md" 2>/dev/null | wc -l | tr -d ' ')

    # Count repos and products
    local prod_count=$(find "$co_dir" -maxdepth 2 -name ".product-config" 2>/dev/null | wc -l | tr -d ' ')
    local repo_count=$(find "$co_dir" -maxdepth 3 -name ".git" -type d 2>/dev/null | wc -l | tr -d ' ')

    printf "  %-15s  ARCH:%s/%s  design:%s/%s  ADRs:%s\n" \
      "$co" "$arch_count" "$prod_count" "$design_count" "$repo_count" "$adr_count"
  done

  # 5. Most recent design.md (file just created?)
  echo ""
  echo "▸ Most recent files (last 5):"
  if [ -n "$co_filter" ]; then
    find "$base/$co_filter" \( -name "design.md" -o -name "ARCHITECTURE.md" -o -path "*/adr/*.md" \) -type f 2>/dev/null \
      | xargs ls -lt 2>/dev/null | head -5 | awk '{print "  ", $6, $7, $8, $NF}'
  else
    find "$base" \( -name "design.md" -o -name "ARCHITECTURE.md" -o -path "*/adr/*.md" \) -type f 2>/dev/null \
      | xargs ls -lt 2>/dev/null | head -5 | awk '{print "  ", $6, $7, $8, $NF}'
  fi

  echo ""
  echo "═══════════════════════════════════════════════════"
  echo " Quick actions:"
  echo "   tail -f $latest"
  echo "   pkill -f 'codex|batch-setup'   # kill all runs"
  echo "   agents-watch [<co>]            # live mode"
  echo "═══════════════════════════════════════════════════"
}

agents-tail() {
  local co_filter="$1"
  local latest
  if [ -n "$co_filter" ]; then
    latest=$(ls -t ~/dotfiles/logs/batch-setup-${co_filter}-*.log 2>/dev/null | head -1)
  else
    latest=$(ls -t ~/dotfiles/logs/batch-setup-*.log 2>/dev/null | head -1)
  fi

  if [ -z "$latest" ]; then
    echo "⚠ No log file found"
    echo "  Run 'setup-context <co>' first, OR wait for new log to be created"
    echo ""
    echo "  Waiting for new log... (Ctrl+C to stop)"
    while [ -z "$latest" ]; do
      sleep 2
      if [ -n "$co_filter" ]; then
        latest=$(ls -t ~/dotfiles/logs/batch-setup-${co_filter}-*.log 2>/dev/null | head -1)
      else
        latest=$(ls -t ~/dotfiles/logs/batch-setup-*.log 2>/dev/null | head -1)
      fi
    done
    echo "  ✓ Detected new log: $(basename $latest)"
    echo ""
  fi

  echo "→ Tailing: $latest"
  echo "  (codex output streams here. Ctrl+C to stop watching, batch-setup продолжит работать в фоне)"
  echo ""
  exec tail -F "$latest"
}

agents-watch() {
  local co_filter="$1"
  trap 'echo ""; echo "stopped."; return' INT
  while true; do
    clear
    agents-status "$co_filter"
    echo ""
    echo "(refresh in 10s, Ctrl+C to stop)"
    sleep 10
  done
}

agents-sessions() {
  # Kill any inherited trace/verbose options for this function and descendants
  emulate -L zsh
  setopt LOCAL_OPTIONS NO_XTRACE NO_VERBOSE NO_SOURCE_TRACE NO_PRINT_EXIT_VALUE NO_WARN_CREATE_GLOBAL NO_WARN_NESTED_VAR NO_TYPESET_SILENT 2>/dev/null
  trap - DEBUG 2>/dev/null   # drop inherited DEBUG trap

  local co_filter="$1"

  # Alternate screen — no scroll pollution; restore on exit
  tput smcup
  trap 'tput rmcup; echo "stopped."; return' INT

  # Frame buffer file — render here, filter to remove trace, then display
  local _frame_buf=$(mktemp -t agents-sessions.XXXXXX)
  trap "tput rmcup; rm -f $_frame_buf; echo stopped; return" INT

  while true; do
    {
      # CRITICAL: reset arrays each frame (local doesn't reset between while iterations in zsh)
      entries=()

      local now=$(date '+%H:%M:%S')
      print -P "%F{cyan}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%f"
      print -P "  %Bagents-sessions%b   %F{244}$now${co_filter:+   ·   $co_filter}%f"
      print -P "%F{cyan}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%f"
      echo ""

      # Get codex node-wrapper PIDs (skip child binary, vscode, login flows)
      local pids
      pids=$(pgrep -fl codex 2>/dev/null | awk '/^[0-9]+ node / && !/vscode|chatgpt|app-server|--login|--help/ {print $1}')

      if [ -z "$pids" ]; then
        print -P "   %F{244}(нет активных codex сессий)%f"
        echo ""
        print -P "   %F{244}— Либо setup-context ещё стартует, либо все продукты доделаны.%f"
      else
        # Build flat entries: "prod|repo|etime|skill|progress"
        local -a entries
        local total=0
        local pid cwd args etime skill progress rel prod repo doc_lines

        for pid in ${(f)pids}; do
          cwd=$(lsof -p "$pid" 2>/dev/null | awk '$4=="cwd"{for(i=9;i<=NF;i++)printf "%s%s",$i,(i<NF?" ":""); exit}')
          [ -z "$cwd" ] && continue

          [ -n "$co_filter" ] && [[ "$cwd" != *"/$co_filter"* ]] && continue

          rel="${cwd#$HOME/Desktop/Prokectfiles/}"
          prod="${rel%/*}"   # everything except last component
          repo="${rel##*/}"  # last component

          # if rel == "<co>/<prod>" (analyze-product, no repo), prod==co, repo==prod
          # Detect: if rel contains exactly 2 path components, it's product-level
          if [[ "$rel" != */*/* ]]; then
            prod="$rel"
            repo="(whole product)"
          fi

          args=$(ps -p "$pid" -o args= 2>/dev/null)
          skill="codex"
          case "$args" in
            *analyze-product*) skill="🌐 product" ;;
            *analyze-repo*)    skill="📦 repo   " ;;
          esac

          etime=$(ps -p "$pid" -o etime= 2>/dev/null | tr -d ' ')

          progress="reading code..."
          if [ -f "$cwd/docs/design.md" ]; then
            doc_lines=$(wc -l < "$cwd/docs/design.md" 2>/dev/null | tr -d ' ')
            progress="design.md  $doc_lines L"
          elif [ -f "$cwd/docs/ARCHITECTURE.md" ]; then
            doc_lines=$(wc -l < "$cwd/docs/ARCHITECTURE.md" 2>/dev/null | tr -d ' ')
            progress="ARCH.md    $doc_lines L"
          fi

          entries+=("${prod}|${repo}|${etime}|${skill}|${progress}")
          total=$((total+1))
        done

        # Sort by product (first field) for grouping
        entries=("${(@on)entries}")

        # Render — group by product
        local prev_prod=""
        local entry rest
        for entry in "${entries[@]}"; do
          prod="${entry%%|*}";    rest="${entry#*|}"
          repo="${rest%%|*}";     rest="${rest#*|}"
          etime="${rest%%|*}";    rest="${rest#*|}"
          skill="${rest%%|*}";    progress="${rest#*|}"

          if [ "$prod" != "$prev_prod" ]; then
            [ -n "$prev_prod" ] && echo ""
            print -P "  %B%F{magenta}■ $prod%f%b"
            prev_prod="$prod"
          fi
          # Layout: skill  repo (20 wide)  etime  →  progress
          printf "      \033[32m%s\033[0m  \033[1m%-22s\033[0m  \033[2m%-8s →  %s\033[0m\n" "$skill" "$repo" "$etime" "$progress"
        done

        echo ""
        print -P "  %F{244}─ всего $total $([ $total -eq 1 ] && print -n "активная сессия" || print -n "активных сессий")%f"
      fi

      # Orchestrator count
      local bp_count
      bp_count=$(pgrep -f "batch-setup" 2>/dev/null | wc -l | tr -d ' ')
      echo ""
      print -P "  %F{244}─ batch-setup orchestrators: $bp_count running%f"

      echo ""
      print -P "%F{cyan}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%f"
      print -P "  %F{244}refresh 3s · Ctrl+C — stop · agents-tail${co_filter:+ $co_filter} для деталей%f"
    } > $_frame_buf 2>&1

    # Render: clear screen, then dump filtered buffer (drops varname=... trace lines)
    tput cup 0 0
    tput ed
    awk '
      # Drop lines that look like zsh trace output: varname=value, varname=(...), arr[idx]=...
      /^[a-zA-Z_][a-zA-Z0-9_]*=/                   { next }
      /^[a-zA-Z_][a-zA-Z0-9_]*\[[^]]*\]=/          { next }
      /^[a-zA-Z_][a-zA-Z0-9_]*\+=/                 { next }
      { print }
    ' "$_frame_buf"

    sleep 3
  done
}
