# shell function `launch` — repo-level tmux launcher (Steinberger pattern)
#
# Target: ~/dotfiles/shell/31-launch.zsh
#
# ### Workspaces
#
# ## launch
# **Что:** Запустить tmux session для одного репо с 4 окнами: plan / code / test / oracle.
# Атомарность работы — repo (по ресерчу snapshot). Координация — на уровне продукта.
# **Запуск:**
# - `launch` — interactive: company → product → repo
# - `launch <co>` — start from company; pick product + repo
# - `launch <co> <prod>` — start from product; pick repo
# - `launch <co> <prod> <repo>` — direct repo with default task `work`
# - `launch <co> <prod> <repo> <task-slug>` — fully direct safe worktree launch
# **Окна:**
# - 🧠 plan    — codex high reasoning (architect)
# - 💻 code    — claude code (implementer)
# - 🧪 test    — auto-detected test watcher (Suggest mode — нужно Enter)
# - 🔮 oracle  — @steipete/oracle CLI (deep review)
# **Зависит от:** codex, claude, oracle CLI, tmux

# ─── Config ───
: ${LAUNCH_PLAN_MODEL:=gpt-5.5}
: ${LAUNCH_PLAN_REASONING:=high}

# ─── Helpers: picking via numbered menu ───

_launch_pick() {
  local prompt="$1"; shift
  local opts=("$@")
  local n=${#opts[@]}
  [ $n -eq 0 ] && return 1
  [ $n -eq 1 ] && { echo "${opts[1]}"; return 0; }

  print -P "%F{cyan}$prompt%f" >&2
  local i=1
  for o in "${opts[@]}"; do
    print -P "  %F{244}$i)%f  $o" >&2
    i=$((i+1))
  done
  print -P "%F{244}Choose 1-$n (Enter to cancel):%f " >&2
  local choice
  read -k 2 choice
  echo "" >&2
  choice=${choice//[^0-9]/}
  [ -z "$choice" ] && return 1
  [ "$choice" -lt 1 ] && return 1
  [ "$choice" -gt $n ] && return 1
  echo "${opts[$choice]}"
}

_launch_list_companies() {
  local base=$HOME/Desktop/Prokectfiles
  for d in "$base"/*/; do
    [ -f "$d/.company-config" ] && basename "$d"
  done
}

_launch_list_products() {
  local co=$1
  local base=$HOME/Desktop/Prokectfiles
  for d in "$base/$co"/*/; do
    [ -f "$d/.product-config" ] && basename "$d"
  done
}

_launch_list_repos() {
  local co=$1 prod=$2
  local base=$HOME/Desktop/Prokectfiles
  for d in "$base/$co/$prod"/*/; do
    [ -d "$d/.git" ] && basename "$d"
  done
}

# ─── Test command auto-detection ───

_launch_detect_test() {
  local repo_dir=$1

  if [ -f "$repo_dir/Makefile" ]; then
    if grep -qE '^test-watch:' "$repo_dir/Makefile" 2>/dev/null; then
      echo "make test-watch"; return
    fi
  fi

  if [ -f "$repo_dir/package.json" ]; then
    if grep -q '"test:watch"' "$repo_dir/package.json" 2>/dev/null; then
      echo "npm run test:watch"; return
    fi
    if grep -qE '"vitest"' "$repo_dir/package.json" 2>/dev/null; then
      echo "npx vitest --watch"; return
    fi
    if grep -qE '"jest"' "$repo_dir/package.json" 2>/dev/null; then
      echo "npx jest --watch"; return
    fi
    if grep -q '"test"' "$repo_dir/package.json" 2>/dev/null; then
      echo "npm test"; return
    fi
  fi

  if [ -f "$repo_dir/pyproject.toml" ] || [ -f "$repo_dir/setup.py" ]; then
    if grep -qE "pytest-watcher|pytest-watch" "$repo_dir/pyproject.toml" 2>/dev/null; then
      echo "ptw . --runner 'pytest -fff --tb=short'"; return
    fi
    if [ -f "$repo_dir/Makefile" ] && grep -qE '^test:' "$repo_dir/Makefile" 2>/dev/null; then
      echo "make test"; return
    fi
    echo "uv run pytest -v --tb=short"; return
  fi

  if [ -f "$repo_dir/Cargo.toml" ]; then
    echo "cargo watch -x test"; return
  fi

  if [ -f "$repo_dir/go.mod" ]; then
    echo "go test ./..."; return
  fi

  if [ -f "$repo_dir/docs.json" ] || [ -f "$repo_dir/mint.json" ]; then
    echo "mintlify dev"; return
  fi

  echo "echo 'no test watcher auto-detected — write your test command'"
}

# ─── Pane script generators ───

_launch_write_banner_script() {
  # $1 = output script path
  # $2 = emoji + label (e.g. "🧠 PLAN")
  # $3-$5 kept for backward-compatible call shape
  # $6 = session name
  # $7 = command to run

  local script=$1 emoji_label=$2 role=$3 tool=$4 use_for=$5 session=$6 cmd=$7

  cat > "$script" <<EOF
#!/usr/bin/env bash
clear
printf '\\033]2;$emoji_label   $session\\007'
exec $cmd
EOF
  chmod +x "$script"
}

_launch_write_test_banner() {
  local script=$1 session=$2 test_cmd=$3 repo_dir=$4
  cat > "$script" <<EOF
#!/usr/bin/env bash
clear
printf '\\033]2;🧪 TEST   $session\\007'
exec bash "\$HOME/dotfiles/scripts/test-menu.sh" "$repo_dir" "$test_cmd"
EOF
  chmod +x "$script"
}

# ─── Main launcher ───

launch() {
  local co prod repo task

  case $# in
    0)
      local companies=($(_launch_list_companies))
      [ ${#companies[@]} -eq 0 ] && { echo "⚠ No companies in ~/Desktop/Prokectfiles"; return 1; }
      co=$(_launch_pick "Companies:" "${companies[@]}") || return 1

      local products=($(_launch_list_products "$co"))
      [ ${#products[@]} -eq 0 ] && { echo "⚠ No products in $co"; return 1; }
      prod=$(_launch_pick "Products in ${co}:" "${products[@]}") || return 1

      local repos=($(_launch_list_repos "$co" "$prod"))
      [ ${#repos[@]} -eq 0 ] && { echo "⚠ No repos in $co/$prod"; return 1; }
      repo=$(_launch_pick "Repos in ${co}/${prod}:" "${repos[@]}") || return 1
      ;;
    1)
      co=$1
      local products=($(_launch_list_products "$co"))
      prod=$(_launch_pick "Products in ${co}:" "${products[@]}") || return 1
      local repos=($(_launch_list_repos "$co" "$prod"))
      repo=$(_launch_pick "Repos in ${co}/${prod}:" "${repos[@]}") || return 1
      ;;
    2)
      co=$1; prod=$2
      local repos=($(_launch_list_repos "$co" "$prod"))
      repo=$(_launch_pick "Repos in ${co}/${prod}:" "${repos[@]}") || return 1
      ;;
    3)
      co=$1; prod=$2; repo=$3
      ;;
    4)
      co=$1; prod=$2; repo=$3; task=$4
      ;;
    *)
      echo "Usage: launch [<co> [<prod> [<repo> [<task-slug>]]]]"
      return 1
      ;;
  esac

  if [ -z "${task:-}" ]; then
    task=work
  fi

  agent-workspace launch "$co" "$prod" "$repo" "$task"
}

# ─── Session builder ───

_launch_session() {
  local co=$1 prod=$2 repo=$3
  local repo_dir="$HOME/Desktop/Prokectfiles/$co/$prod/$repo"

  [ -d "$repo_dir" ] || { echo "⚠ Not found: $repo_dir"; return 1; }
  [ -d "$repo_dir/.git" ] || { echo "⚠ Not a git repo: $repo_dir"; return 1; }

  _launch_session_path "${co}-${prod}-${repo}" "$repo_dir" "🖥 ${repo}"
}

# WikiPedik memory health: print a short status and put a badge into the tmux
# status bar, so a dead memory loop is visible at launch instead of silent.
_launch_memory_status() {
  local repo_dir=$1 session=$2
  local kdir="$repo_dir/docs/knowledge"

  # Live reap indicator (only for managed agent worktrees).
  local reap_seg=""
  if [ -f "$repo_dir/.agent-workspace" ]; then
    reap_seg="#(bash $HOME/dotfiles/scripts/agent-workspace.sh reap-status '$repo_dir') | "
  fi

  if [ ! -f "$kdir/hot.md" ]; then
    echo "⚠ wiki memory: не подключена (нет docs/knowledge/hot.md) — запусти wiki bootstrap"
    tmux set-option -t "$session" status-right "${reap_seg}wiki:✗ | %H:%M " 2>/dev/null
    return 0
  fi

  local cand stamp age_note="" badge
  cand=$(grep -c '^Status: candidate$' "$kdir/_inbox.md" 2>/dev/null) || cand=0
  stamp=$(grep -o 'last refreshed: [0-9-]*' "$kdir/hot.md" 2>/dev/null | head -1 | awk '{print $3}')

  if [ -z "$stamp" ] || [ "$stamp" = "never" ]; then
    age_note="hot.md ещё не собирался"
  else
    local then_s now_s days
    then_s=$(date -j -f '%Y-%m-%d' "$stamp" +%s 2>/dev/null || echo 0)
    now_s=$(date +%s)
    if [ "$then_s" -gt 0 ]; then
      days=$(( (now_s - then_s) / 86400 ))
      [ "$days" -gt 14 ] && age_note="hot.md устарел: $days дн."
    fi
  fi

  badge="wiki:✓"
  [ "$cand" -gt 0 ] && badge="$badge in:$cand"
  tmux set-option -t "$session" status-right "${reap_seg}${badge} | %H:%M " 2>/dev/null

  if [ "$cand" -ge 5 ] || [ -n "$age_note" ]; then
    echo "ℹ wiki memory: inbox $cand candidate${age_note:+ · $age_note} — пора wiki sync"
  fi

  # Hygiene nudges (cheap, deterministic): lint age and history-ingest age.
  local health="$repo_dir/docs/product-knowledge/health.md" lint_line
  if [ -f "$health" ]; then
    lint_line=$(grep -m1 '^Last lint:' "$health" 2>/dev/null)
    case "$lint_line" in
      *never*) echo "ℹ wiki lint: ни разу не запускался для этого продукта — wiki lint <co>/<prod>" ;;
      *)
        local lint_date lint_s lint_now
        lint_date=$(print -r -- "$lint_line" | grep -o '[0-9]\{4\}-[0-9]\{2\}-[0-9]\{2\}' | head -1)
        if [ -n "$lint_date" ]; then
          lint_s=$(date -j -f '%Y-%m-%d' "$lint_date" +%s 2>/dev/null || echo 0)
          lint_now=$(date +%s)
          [ "$lint_s" -gt 0 ] && [ $(( (lint_now - lint_s) / 86400 )) -gt 30 ] && \
            echo "ℹ wiki lint: последний $lint_date (>30 дн.) — пора wiki lint"
        fi
        ;;
    esac
  fi

  local manifest="$HOME/Desktop/WikiPedik/dev/10-wiki/sources/sessions/.ingest-manifest.json" ingest_s ingest_days
  if [ -f "$manifest" ]; then
    ingest_s=$(stat -f %m "$manifest" 2>/dev/null || echo 0)
    if [ "$ingest_s" -gt 0 ]; then
      ingest_days=$(( ($(date +%s) - ingest_s) / 86400 ))
      [ "$ingest_days" -ge 14 ] && \
        echo "ℹ history-ingest: последний прогон $ingest_days дн. назад — пора майнить сессии (agent-history-ingest)"
    fi
  fi
}

_launch_session_path() {
  local session=$1 repo_dir=$2 work_label=$3 mode=${4:-attach}

  [ -d "$repo_dir" ] || { echo "⚠ Not found: $repo_dir"; return 1; }
  [ -d "$repo_dir/.git" ] || [ -f "$repo_dir/.git" ] || { echo "⚠ Not a git worktree: $repo_dir"; return 1; }

  tmux kill-session -t "$session" 2>/dev/null

  export CODEX_HOME="$HOME/.codex-new"
  export CLAUDE_CONFIG_DIR="$HOME/.claude-new"
  export AGENT_GIT_MODE="commit-local"
  export AGENT_GIT_PUSH="task"
  export AGENT_WORKSPACE_MODE="shared"
  if [ -f "$repo_dir/.agent-workspace" ]; then
    export AGENT_WORKSPACE_MODE="agent"
  fi

  local fresh_env='export CODEX_HOME="$HOME/.codex-new"; export CLAUDE_CONFIG_DIR="$HOME/.claude-new"; export AGENT_GIT_MODE="commit-local"; export AGENT_GIT_PUSH="task"; export AGENT_WORKSPACE_MODE="'"$AGENT_WORKSPACE_MODE"'";'

  local test_cmd
  test_cmd=$(_launch_detect_test "$repo_dir")

  # Banner scripts written to temp dir. Clean up after a delay (disowned, NOT a
  # function-EXIT trap): in zsh a trap…EXIT inside a function fires on function
  # return, which in detached mode would delete the scripts before the panes
  # run them. The panes execute each script once at startup, so a short grace
  # period is enough for both attach and detached modes.
  local tmpdir=$(mktemp -d -t launch-${session}.XXXXXX)
  ( sleep 30; rm -rf "$tmpdir" ) &!

  _launch_write_banner_script "$tmpdir/plan.sh" \
    "🧠 PLAN" \
    "architect / planner" \
    "codex $LAUNCH_PLAN_MODEL  reasoning=$LAUNCH_PLAN_REASONING" \
    "spec writing, decomposition, design discussion" \
    "$session" \
    "bash \"\$HOME/dotfiles/scripts/masko-agent-wrap.sh\" codex \"$repo_dir\" \"$session\" -- codex --model $LAUNCH_PLAN_MODEL -c model_reasoning_effort=\"$LAUNCH_PLAN_REASONING\""

  _launch_write_banner_script "$tmpdir/code.sh" \
    "💻 CODE" \
    "implementer" \
    "claude code" \
    "writing, refactoring, applying changes" \
    "$session" \
    "bash \"\$HOME/dotfiles/scripts/masko-agent-wrap.sh\" claudeCode \"$repo_dir\" \"$session\" -- claude"

  _launch_write_test_banner "$tmpdir/test.sh" "$session" "$test_cmd" "$repo_dir"

  _launch_write_banner_script "$tmpdir/oracle.sh" \
    "🔮 ORACLE" \
    "deep review" \
    "@steipete/oracle  ChatGPT.com Pro (headless)" \
    "design decisions, refactors, debugging stuck" \
    "$session" \
    "bash \"\$HOME/dotfiles/scripts/oracle-loop.sh\" \"$repo_dir\""

  # === Build session: one window, four panes ===
  local plan_pane code_pane test_pane oracle_pane
  plan_pane=$(tmux new-session -d -s "$session" -c "$repo_dir" -n "work" -P -F '#{pane_id}')
  tmux set-environment -t "$session" CODEX_HOME "$HOME/.codex-new"
  tmux set-environment -t "$session" CLAUDE_CONFIG_DIR "$HOME/.claude-new"
  tmux set-environment -t "$session" AGENT_GIT_MODE "commit-local"
  tmux set-environment -t "$session" AGENT_GIT_PUSH "task"
  tmux set-environment -t "$session" AGENT_WORKSPACE_MODE "$AGENT_WORKSPACE_MODE"
  # Reap the worktree when this session closes — safe: removes only if the
  # worktree is clean+pushed (or ready), otherwise leaves it for stale/cleanup.
  if [ "$AGENT_WORKSPACE_MODE" = "agent" ] && [ -f "$repo_dir/.agent-workspace" ]; then
    # GLOBAL session-closed hook (not per-session: a per-session one does not
    # fire when its own session is killed). On any session close, sweep all
    # managed worktrees and reap those whose session is gone (clean+pushed only).
    tmux set-hook -g session-closed \
      "run-shell \"bash $HOME/dotfiles/scripts/agent-workspace.sh reap-closed >>$HOME/Library/Logs/agent-workspace-reap.log 2>&1\""
    # Live reap indicator goes into status-right (set by _launch_memory_status):
    # green ●synced = closing removes it, yellow ●…hold = unsynced work kept.
    tmux set-option -t "$session" status-interval 15
  fi
  tmux set-option -t "$session" status-left " ${work_label} "
  tmux set-option -t "$session" set-titles on
  tmux set-option -t "$session" set-titles-string "${work_label}"
  tmux set-window-option -t "${session}:work" pane-border-status top

  code_pane=$(tmux split-window -h -t "$plan_pane" -c "$repo_dir" -P -F '#{pane_id}')
  test_pane=$(tmux split-window -v -t "$plan_pane" -c "$repo_dir" -P -F '#{pane_id}')
  oracle_pane=$(tmux split-window -v -t "$code_pane" -c "$repo_dir" -P -F '#{pane_id}')

  tmux set-option -p -t "$plan_pane" @launch_role "plan"
  tmux set-option -p -t "$code_pane" @launch_role "code"
  tmux set-option -p -t "$test_pane" @launch_role "test"
  tmux set-option -p -t "$oracle_pane" @launch_role "oracle"
  tmux set-window-option -t "${session}:work" pane-border-format " #{@launch_role} "

  tmux send-keys -t "$plan_pane" "$fresh_env bash $tmpdir/plan.sh" Enter
  tmux send-keys -t "$code_pane" "$fresh_env bash $tmpdir/code.sh" Enter
  tmux send-keys -t "$test_pane" "$fresh_env bash $tmpdir/test.sh" Enter
  tmux send-keys -t "$oracle_pane" "$fresh_env bash $tmpdir/oracle.sh" Enter

  # Tool UIs may set pane titles while starting; set our durable labels last.
  sleep 0.2
  tmux select-pane -t "$plan_pane" -T "plan"
  tmux select-pane -t "$code_pane" -T "code"
  tmux select-pane -t "$test_pane" -T "test"
  tmux select-pane -t "$oracle_pane" -T "oracle"

  # Plan pane = active on attach
  tmux select-pane -t "$plan_pane"
  tmux rename-window -t "${session}:work" "${work_label}"
  _launch_memory_status "$repo_dir" "$session"
  bash "$HOME/dotfiles/scripts/wikipedik-autocommit.sh" --if-due 2>/dev/null || true
  [ "$mode" = "detached" ] && return 0
  tmux attach -t "$session"
}
