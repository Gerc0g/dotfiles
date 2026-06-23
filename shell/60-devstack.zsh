# Shared local dev infrastructure control.
#
# DEV_STACK_HOST is where the stack lives. Default `gerc0g` — the home Ubuntu
# dev-server (MagicDNS name → tailnet 100.73.117.50), which hosts the stack 24/7.
# Override with DEV_STACK_HOST=localhost to talk to a stack running on this Mac.
# Connection strings / URLs follow it, no other change needed.
#
# EXPORTED so child processes (direnv bash subshell, agent panes, apps) see it —
# project .envrc blocks resolve endpoints from it. Without export, .envrc silently
# fell back to localhost (where nothing runs).
export DEV_STACK_HOST="${DEV_STACK_HOST:-gerc0g}"

# Where to manage the stack when it lives on a remote host: an ssh target that
# can run docker (Tailscale SSH alias). Only used when DEV_STACK_HOST != localhost.
: "${DEV_STACK_SSH:=ubuntu-server}"

# ─── remote-aware docker helpers ───
# The stack normally runs on the server, so `docker` / `docker compose` must run
# THERE. When DEV_STACK_HOST is local, run against the local engine.

_devstack_is_local() {
  local h="${DEV_STACK_HOST:-gerc0g}"
  [ "$h" = "localhost" ] || [ "$h" = "127.0.0.1" ]
}

_devstack_sh() {   # non-interactive docker subcommand on the stack host
  if _devstack_is_local; then docker "$@"; else ssh "$DEV_STACK_SSH" docker "$@"; fi
}

_devstack_ti() {   # interactive docker exec on the stack host (-t for ssh)
  if _devstack_is_local; then docker "$@"; else ssh -t "$DEV_STACK_SSH" docker "$@"; fi
}

_devstack_compose() {   # docker compose against the right host + compose file
  if _devstack_is_local; then
    docker compose -f ~/dotfiles/services/dev-stack/docker-compose.yml "$@"
  else
    ssh "$DEV_STACK_SSH" "cd /root/dev-stack && docker compose $*"
  fi
}

# Run SQL from STDIN on dev-postgres. Piping via stdin (docker exec -i) avoids the
# ssh arg-flattening that mangles quoted SQL passed with psql -c over ssh.
_devstack_psql_in() {
  # -d postgres: connect to the maintenance DB (psql -U dev alone would target a
  # database literally named "dev", which does not exist).
  if _devstack_is_local; then docker exec -i dev-postgres psql -U dev -d postgres "$@"
  else ssh "$DEV_STACK_SSH" docker exec -i dev-postgres psql -U dev -d postgres "$@"; fi
}

# Resolve the stack host to something probeable. MagicDNS short names can fail to
# resolve when a full-tunnel VPN (Happ) hijacks DNS, even though the tailnet route
# is up — fall back to the tailnet IP via the tailscale CLI.
_devstack_tailscale() {
  if command -v tailscale >/dev/null 2>&1; then tailscale "$@"
  else /Applications/Tailscale.app/Contents/MacOS/Tailscale "$@" 2>/dev/null; fi
}

_devstack_db_exists() {   # 0 if database $1 exists
  printf "SELECT 1 FROM pg_database WHERE datname='%s';" "$1" \
    | _devstack_psql_in -tA 2>/dev/null | grep -q 1
}

# ─── project connection (.envrc block) ───

# Derive "product repo db" from args ($1 $2) or the current path under
# ~/Desktop/Prokectfiles/<co>/<prod>/<repo> (also handles .worktrees/.../<id>).
_devstack_ctx() {
  local prod repo
  if [ -n "$1" ] && [ -n "$2" ]; then
    prod="$1"; repo="$2"
  else
    local base="${PROKECTFILES_ROOT:-$HOME/Desktop/Prokectfiles}" rel
    case "$PWD" in
      "$base"/*) rel="${PWD#$base/}" ;;
      *) echo "⚠ not under $base — pass <product> <repo>" >&2; return 1 ;;
    esac
    [[ "$rel" == .worktrees/* ]] && rel="${rel#.worktrees/}"
    rel="${rel#*/}"            # drop company
    prod="${rel%%/*}"; rel="${rel#*/}"
    repo="${rel%%/*}"
  fi
  [ -n "$prod" ] && [ -n "$repo" ] || { echo "⚠ could not derive product/repo from $PWD" >&2; return 1; }
  local db
  db=$(printf '%s__%s' "$prod" "$repo" | tr '[:upper:]-' '[:lower:]_' | tr -cd 'a-z0-9_')
  echo "$prod" "$repo" "$db"
}

# Print the .envrc block. Only the DB name is substituted now; $H/${DEV_STACK_HOST}
# stay LITERAL so direnv resolves the host at `cd` time.
_devstack_envrc_block() {
  local db="$1"
  cat <<'HEAD'
# >>> dev-stack (managed: dev-stack connect) >>>
H="${DEV_STACK_HOST:-gerc0g}"
HEAD
  printf 'export DATABASE_URL="postgresql://dev:dev@$H:5432/%s"\n' "$db"
  cat <<'TAIL'
export REDIS_URL="redis://$H:6379/0"
export QDRANT_URL="http://$H:6333"
export CLICKHOUSE_URL="http://dev:dev@$H:8123"
export S3_ENDPOINT_URL="http://$H:9000"   # MinIO; creds key=dev secret=devsecret123 (set AWS_* per SDK if needed)
export OTEL_EXPORTER_OTLP_ENDPOINT="http://$H:4317"   # telemetry is PUSHED via OTLP (Prometheus does not scrape you)
export LANGFUSE_HOST="http://$H:3001"
export OLLAMA_HOST="http://$H:11434"
# <<< dev-stack <<<
TAIL
}

dev-stack() {
  local cmd="${1:-status}"
  local h="${DEV_STACK_HOST:-gerc0g}"

  case "$cmd" in
    urls)
      cat <<URLS
Dev stack @ ${h}   (manage on host via: ssh ${DEV_STACK_SSH})

  core
    Postgres      ${h}:5432            user=dev pass=dev
    Redis         ${h}:6379
  storage / vectors
    MinIO  S3     http://${h}:9000     key=dev secret=devsecret123
    MinIO  UI     http://${h}:9001
    Qdrant        http://${h}:6333     (gRPC ${h}:6334)
    ClickHouse    http://${h}:8123     user=dev pass=dev  (HTTP only; native :9000 internal)
  observability   (push telemetry via OTLP — Prometheus does NOT scrape your app)
    OTel ingest   ${h}:4317 (grpc) / ${h}:4318 (http)
    Prometheus    http://${h}:9090
    Grafana       http://${h}:3030     user=dev pass=dev  (Prom/Loki/Tempo pre-wired)
    Loki          http://${h}:3100
    Tempo         http://${h}:3200
  llm / ml
    Langfuse      http://${h}:3001
    Ollama        http://${h}:11434
  bi / viewer
    Metabase      http://${h}:3002
    CloudBeaver   http://${h}:8978
  hub / proxy
    Homepage      http://${h}/          (landing; Traefik name-routing *.${h}.sslip.io)
    Traefik dash  http://${h}:8080

Connect a project:  cd <repo> && dev-stack connect
URLS
      return 0
      ;;

    envrc)
      shift
      local ctx; ctx=$(_devstack_ctx "$1" "$2") || return 1
      local prod repo db; read -r prod repo db <<<"$ctx"
      _devstack_envrc_block "$db"
      return 0
      ;;

    connect)
      shift
      local ctx; ctx=$(_devstack_ctx "$1" "$2") || return 1
      local prod repo db; read -r prod repo db <<<"$ctx"
      local envrc="./.envrc"
      if [ -f "$envrc" ] && grep -q ">>> dev-stack" "$envrc"; then
        echo "✓ .envrc already has a dev-stack block"
      else
        [ -f "$envrc" ] || printf 'source_up\n' > "$envrc"
        { printf '\n'; _devstack_envrc_block "$db"; } >> "$envrc"
        echo "✓ appended dev-stack block to $envrc"
      fi
      echo "  product=$prod repo=$repo db=$db host=\$DEV_STACK_HOST ($h)"
      dev-stack db-ensure "$db"
      if command -v direnv >/dev/null 2>&1; then
        direnv allow . 2>/dev/null && echo "✓ direnv allow"
      fi
      echo "→ verify: dev-stack doctor"
      return 0
      ;;

    doctor)
      echo "dev-stack doctor @ $h   (manage: ssh ${DEV_STACK_SSH})"
      local target="$h"
      if ! _devstack_is_local && ! nc -z -w2 "$h" 5432 >/dev/null 2>&1; then
        local ip; ip=$(_devstack_tailscale ip -4 "$h" 2>/dev/null | head -1)
        if [ -n "$ip" ]; then
          echo "  ⚠ '$h' не резолвится напрямую (DNS, вероятно, перехвачен Happ);"
          echo "    проверяю по tailnet IP $ip. ВНИМАНИЕ: приложения тоже не разрезолвят"
          echo "    имя '$h', пока DNS перехвачен — для dev либо чини MagicDNS/route-pin,"
          echo "    либо ставь DEV_STACK_HOST=$ip."
          target="$ip"
        fi
      fi
      echo "reachability (@ $target):"
      local probe name port
      for probe in "postgres 5432" "redis 6379" "qdrant 6333" "clickhouse 8123" \
                   "minio 9000" "otel 4317" "grafana 3030" "langfuse 3001" "ollama 11434"; do
        name="${probe%% *}"; port="${probe##* }"
        if nc -z -w2 "$target" "$port" >/dev/null 2>&1; then
          echo "  ✓ $name ($port)"
        else
          echo "  ✗ $name ($port) UNREACHABLE"
        fi
      done
      echo "project:"
      if [ -f ./.envrc ] && grep -q ">>> dev-stack" ./.envrc; then
        echo "  ✓ ./.envrc has dev-stack block"
      else
        echo "  ✗ ./.envrc not connected — run: dev-stack connect"
      fi
      local ctx prod repo db
      if ctx=$(_devstack_ctx 2>/dev/null); then
        read -r prod repo db <<<"$ctx"
        if _devstack_db_exists "$db"; then
          echo "  ✓ db '$db' exists"
        else
          echo "  ✗ db '$db' missing — run: dev-stack db-ensure $db"
        fi
      fi
      return 0
      ;;
  esac

  # ─── lifecycle / access (run against the stack host) ───
  if _devstack_is_local && ! docker ps >/dev/null 2>&1; then
    echo "⚠ Docker не запущен (DEV_STACK_HOST=localhost). Запусти OrbStack."
    return 1
  fi

  case "$cmd" in
    up|start)
      shift
      _devstack_compose up -d "$@"
      [ $# -eq 0 ] && echo "✓ Dev stack up @ $h. Адреса: dev-stack urls" || echo "✓ Started: $*"
      ;;
    down|stop)    shift; _devstack_compose down "$@" ;;
    restart)      shift; _devstack_compose restart "$@" ;;
    pull)         _devstack_compose pull ;;
    logs)         shift; _devstack_compose logs -f "$@" ;;
    ps|status)    _devstack_compose ps ;;
    psql)         _devstack_ti exec -it dev-postgres psql -U dev "${2:-postgres}" ;;
    redis-cli)    _devstack_ti exec -it dev-redis redis-cli ;;
    ch|clickhouse) _devstack_ti exec -it dev-clickhouse clickhouse-client -u dev --password dev ;;
    db-ensure)
      [ -z "$2" ] && { echo "Usage: dev-stack db-ensure <name>"; return 1; }
      if _devstack_db_exists "$2"; then
        echo "✓ DB '$2' exists"
      else
        printf 'CREATE DATABASE "%s";' "$2" | _devstack_psql_in >/dev/null \
          && echo "✓ DB '$2' created" || echo "✗ failed to create DB '$2'"
      fi
      ;;
    db-create)
      [ -z "$2" ] && { echo "Usage: dev-stack db-create <name>"; return 1; }
      printf 'CREATE DATABASE "%s";' "$2" | _devstack_psql_in && echo "✓ DB '$2' created"
      ;;
    db-drop)
      [ -z "$2" ] && { echo "Usage: dev-stack db-drop <name>"; return 1; }
      echo "⚠ Drop database '$2'? (y/N)"
      read -r ans
      [ "$ans" = "y" ] && printf 'DROP DATABASE "%s";' "$2" | _devstack_psql_in
      ;;
    nuke)
      echo "⚠ Это снесёт все данные dev-stack @ $h. Продолжить? (y/N)"
      read -r ans
      [ "$ans" = "y" ] && _devstack_compose down -v && echo "✓ Nuked"
      ;;
    *)
      cat <<USAGE
Usage: dev-stack <command>          (stack @ ${h}; manage via ssh ${DEV_STACK_SSH})

Connect a project (self-onboard):
  connect [prod repo]   Append dev-stack block to ./.envrc, create its DB, direnv allow
  envrc   [prod repo]   Print the .envrc block (no write)
  doctor                Check stack reachability + whether THIS project is wired

Lifecycle:
  up | start [svc...]   Start all (or only named) services
  down | stop [svc...]  Stop (data persists)
  restart [svc...] · pull · status|ps · logs [service] · nuke

Access:
  urls                  All service URLs (honours DEV_STACK_HOST)
  psql [db] · redis-cli · ch|clickhouse
  db-ensure <name>      Idempotent CREATE DATABASE
  db-create <name> · db-drop <name>

Stack (18): Postgres Redis MinIO Qdrant ClickHouse Prometheus Grafana Loki Tempo
  OTel-Collector Langfuse Ollama Open-WebUI Metabase CloudBeaver Redis-Commander
  Traefik Homepage
USAGE
      ;;
  esac
}
