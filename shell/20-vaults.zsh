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

  # Профиль зоны изолирован по семантике работы: свои скиллы, хуки и история,
  # общий логин. Нет профиля зоны — работаем на дефолтном.
  local home="$HOME/.codex-$zone"
  [ -d "$home" ] || home="$HOME/.codex"
  CODEX_HOME="$home" codex --no-alt-screen
}
