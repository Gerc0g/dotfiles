# WikiPedik. Одна дверь: `wikipedik` переходит в рабочую зону и запускает
# агента — работа с вольтом идёт разговором, не командами.
#
# Зона по умолчанию — research (обучение). Память проектов (dev) курируется
# отдельным флоу (`wiki sync`), поэтому чатом туда ходить незачем.
wikipedik() {
  local zone=${1:-research}
  local dir
  dir=$(hq wikipedik path "$zone") || return 1
  mkdir -p "$dir"
  cd "$dir" || return 1

  hq research 2>/dev/null

  if [ "${WIKIPEDIK_AUTOSTART:-1}" = "0" ]; then
    print -P "%F{244}агент не запущен (WIKIPEDIK_AUTOSTART=0): codex%f"
    return 0
  fi
  command -v codex >/dev/null 2>&1 || { echo "⚠ codex не найден" >&2; return 1; }

  # Профиль зоны (~/.codex/<zone>.config.toml) убирает из промпта инструменты,
  # которые в ней не нужны. Нет профиля — работаем на базовом конфиге.
  local profile=()
  [ -f "$HOME/.codex/$zone.config.toml" ] && profile=(-p "$zone")
  CODEX_HOME="$HOME/.codex" codex "${profile[@]}" --no-alt-screen
}
