#!/usr/bin/env zsh
# WikiPedik prompt composer + quick ops. Runs in its own pane of the
# wikipedik tmux session.
#
# Chat actions (status/sync/synthesize/lint/ingest) do NOT run anything:
# they compose the curator prompt, copy it to the clipboard, type it into
# the dev chat pane input and jump focus there — you review and press Enter.
# Mechanical actions (rules-sync, hot-refresh, push, bootstrap) run directly.

VAULT="$HOME/Desktop/WikiPedik"
PROJECTS="$VAULT/dev/20-projects"
SCOPE_FILE="$HOME/.cache/wiki-menu-scope"
DEV_PANE="${WIKI_DEV_PANE:-wikipedik:0.0}"

scope=""
[ -f "$SCOPE_FILE" ] && scope=$(<"$SCOPE_FILE")

pick_scope() {
  local -a companies=() products=() repos=()
  local co prod repo d repos_root

  for d in "$PROJECTS"/*/; do
    co=$(basename "$d")
    [[ "$co" == _templates ]] && continue
    companies+=("$co")
  done
  (( ${#companies} )) || { echo "⚠ вольт пуст: $PROJECTS"; return 1; }

  echo ""
  echo "Компания:"
  select co in "${companies[@]}"; do
    [[ -n "$co" ]] && break
  done

  for d in "$PROJECTS/$co"/*/; do
    [[ -d "$d/repos" ]] && products+=("$(basename "$d")")
  done

  if (( ${#products} == 0 )); then
    scope="$co"
    repos_root="$PROJECTS/$co/repos"
  else
    echo "Продукт ($co):"
    select prod in "вся компания" "${products[@]}"; do
      [[ -n "$prod" ]] && break
    done
    if [[ "$prod" == "вся компания" ]]; then
      scope="$co"; repos_root=""
    else
      scope="$co/$prod"; repos_root="$PROJECTS/$co/$prod/repos"
    fi
  fi

  if [[ -n "$repos_root" && -d "$repos_root" ]]; then
    for d in "$repos_root"/*/; do
      repos+=("$(basename "$d")")
    done
    if (( ${#repos} )); then
      echo "Репо ($scope):"
      select repo in "весь scope" "${repos[@]}"; do
        [[ -n "$repo" ]] && break
      done
      [[ "$repo" != "весь scope" ]] && scope="$scope/$repo"
    fi
  fi

  mkdir -p "${SCOPE_FILE:h}"
  print -r -- "$scope" > "$SCOPE_FILE"
}

pause() {
  echo ""
  read -k1 "?— любая клавиша — меню —"
}

# Compose: clipboard + type into dev chat input + jump focus. No Enter sent —
# the user reviews and submits.
deliver_prompt() {
  local prompt=$1
  print -rn -- "$prompt" | pbcopy 2>/dev/null
  if tmux has-session -t "${DEV_PANE%%:*}" 2>/dev/null; then
    tmux send-keys -t "$DEV_PANE" -l -- "$prompt" 2>/dev/null
    tmux select-pane -t "$DEV_PANE" 2>/dev/null
    echo "→ промпт в инпуте dev-чата (и в буфере) — проверь и жми Enter"
  else
    echo "→ промпт в буфере обмена — вставь в чат куратора"
  fi
}

bootstrap_pick() {
  local -a missing=()
  local pick co prod repo d
  for d in "$HOME/Desktop/Prokectfiles"/*/*/*(/N); do
    [[ -e "$d/.git" ]] || continue
    [[ -L "$d/docs/knowledge" ]] && continue
    missing+=("${d#$HOME/Desktop/Prokectfiles/}")
  done
  if (( ${#missing} == 0 )); then
    echo "✓ все репо уже подключены к памяти"
    return 0
  fi
  echo "Репо без памяти:"
  select pick in "${missing[@]}" "отмена"; do
    [[ -n "$pick" ]] && break
  done
  [[ "$pick" == "отмена" || -z "$pick" ]] && return 0
  co="${pick%%/*}"; repo="${pick##*/}"; prod="${${pick#*/}%%/*}"
  zsh "$HOME/dotfiles/skills-stash/wiki/scripts/wiki-bootstrap-product.sh" "$co" "$prod" "$repo"
}

while true; do
  [[ -z "$scope" ]] && pick_scope

  clear
  cat <<EOF
╔══════════════════════════════════════════════════╗
║   WikiPedik · scope: ${scope}
╠═══════ промпт → dev-чат (Enter жмёшь сам) ═══════╣
║  1  status      что накопилось, что делать       ║
║  2  sync        разобрать inbox + сейф           ║
║  3  synthesize  повторы → паттерны/правила       ║
║  4  lint        протухание/противоречия          ║
║  5  ingest      майнинг сессий за 2 недели       ║
╠═══════ механика (выполняется сразу) ═════════════╣
║  6  rules-sync + hot-refresh                     ║
║  7  git status вольта                            ║
║  8  push вольта                                  ║
║  b  bootstrap репо без памяти                    ║
╠══════════════════════════════════════════════════╣
║  s  сменить scope                q  выход        ║
╚══════════════════════════════════════════════════╝
EOF
  read -k1 "choice?> "
  echo ""

  case "$choice" in
    1) deliver_prompt "Use skill wiki-status. Scope: $scope. Отчитайся: счётчики инбоксов и salvage-зон, curated-страницы, synthesis-кандидаты, устаревшие health-файлы, последние записи лога, рекомендуемые действия. Файлы не менять."
       pause ;;
    2) deliver_prompt "Use skill inbox-drain. Scope: $scope. Разбери salvage-зоны (Phase 0) и candidate-записи инбоксов по всем фазам: дедуп, derivability-тест (spec-derivable отклоняй), по каждой записи предлагай target и жди подтверждения, evolution-связи, обнови индексы, product log и обязательно hot.md (Phase 7, fallback: python3 ~/dotfiles/scripts/wiki-hot-refresh.py $scope)."
       pause ;;
    3) deliver_prompt "Use skill wiki-synthesize. Scope: $scope. Обработай _synthesis-candidates.md и повторы lessons/gotchas между репо: предлагай product-паттерны до записи в shared/. Rule Promotion по планке скилла: повторилось 2+ раз И must-follow И ложится на конкретные glob-пути — предложи правило (paths: обязателен), жди моего подтверждения."
       pause ;;
    4) deliver_prompt "Use skill wiki-lint. Scope: $scope. Найди противоречия, устаревшие claims (Last verified), orphan-страницы, кандидатов на promotion и утечки приватного. Findings в health.md scope, событие в log.md. Изменения curated-страниц — только с моего подтверждения."
       pause ;;
    5) deliver_prompt "Use skill agent-history-ingest. --source all. Сначала прогони python3 ~/dotfiles/scripts/agent-session-digest.py --source all --since $(date -v-14d +%Y-%m-%d 2>/dev/null || date +%Y-%m-%d), затем обработай новые дайджесты из 10-wiki/sources/sessions/_digests/ (манифест .ingest-manifest.json, обработанные пропусти). Кластеризуй связанные сессии, candidate captures строго по durable-сигналам с derivability-тестом."
       pause ;;
    6) python3 "$HOME/dotfiles/scripts/wiki-rules-sync.py" "$scope"
       python3 "$HOME/dotfiles/scripts/wiki-hot-refresh.py" "$scope"
       pause ;;
    7) git -C "$VAULT" status --short | sed -n '1,30p'
       echo "— unpushed: $(git -C "$VAULT" rev-list --count origin/main..HEAD 2>/dev/null)"
       pause ;;
    8) git -C "$VAULT" push; pause ;;
    b|B) bootstrap_pick; pause ;;
    s|S) pick_scope ;;
    q|Q) exit 0 ;;
    *) ;;
  esac
done
