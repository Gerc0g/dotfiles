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

  # Хуки зоны (контекст сессии, коммит на каждый шаг) — наши собственные: они
  # лежат в ~/dotfiles под git и правятся нами же. Codex по умолчанию требует
  # подтверждать их заново после каждой правки, и один пропущенный запрос
  # молча выключает автокоммиты — поэтому доверие выдаётся флагом.
  # WIKIPEDIK_HOOK_TRUST=ask возвращает штатный интерактивный запрос.
  local trust=()
  [ "${WIKIPEDIK_HOOK_TRUST:-bypass}" = "bypass" ] && trust=(--dangerously-bypass-hook-trust)

  CODEX_HOME="$home" codex "${trust[@]}" --no-alt-screen

  # Последний рубеж: что бы ни случилось в сессии — агент забыл закоммитить,
  # Stop-хук не доверен, сессия оборвалась — вольт не остаётся грязным.
  hq wikipedik autosync
}
