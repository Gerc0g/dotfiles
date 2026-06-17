# Shared local dev infrastructure control.
#
# DEV_STACK_HOST is where the stack lives. Default `gerc0g` — the home Ubuntu
# dev-server (MagicDNS name → tailnet 100.73.117.50), which hosts the stack 24/7.
# Override with DEV_STACK_HOST=localhost to talk to a stack running on this Mac.
# Connection strings / URLs follow it, no other change needed.
: "${DEV_STACK_HOST:=gerc0g}"

dev-stack() {
  local cmd="${1:-status}"
  local compose=~/dotfiles/services/dev-stack/docker-compose.yml
  local h="${DEV_STACK_HOST:-localhost}"

  case "$cmd" in
    urls)
      cat <<URLS
Dev stack @ ${h}

  core
    Postgres      ${h}:5432            user=dev pass=dev
    Redis         ${h}:6379
  storage / vectors
    MinIO  S3     http://${h}:9000     key=dev secret=devsecret123
    MinIO  UI     http://${h}:9001
    Qdrant        http://${h}:6333     (gRPC ${h}:6334)
    ClickHouse    http://${h}:8123     user=dev pass=dev
  observability
    Prometheus    http://${h}:9090
    Grafana       http://${h}:3030     user=dev pass=dev
    Loki          http://${h}:3100
    Tempo         http://${h}:3200
    OTel ingest   ${h}:4317 (grpc) / ${h}:4318 (http)
  llm / ml
    Langfuse      http://${h}:3001
    Ollama        http://${h}:11434
  bi / viewer
    Metabase      http://${h}:3002
    CloudBeaver   http://${h}:8978
URLS
      return 0
      ;;
  esac

  # Everything below talks to the local docker engine.
  if ! docker ps >/dev/null 2>&1; then
    echo "⚠ Docker не запущен. Запусти OrbStack (Spotlight → OrbStack)."
    return 1
  fi

  case "$cmd" in
    up|start)
      shift
      docker compose -f "$compose" up -d "$@"   # no args = all; or `up qdrant langfuse`
      echo ""
      if [ $# -eq 0 ]; then
        echo "✓ Dev stack up. Adresa: dev-stack urls"
      else
        echo "✓ Started: $*"
      fi
      ;;
    down|stop)
      shift
      docker compose -f "$compose" down "$@"
      ;;
    restart)
      shift
      docker compose -f "$compose" restart "$@"
      ;;
    pull)
      docker compose -f "$compose" pull
      ;;
    logs)
      shift
      docker compose -f "$compose" logs -f "$@"
      ;;
    ps|status)
      docker compose -f "$compose" ps
      ;;
    psql)
      docker exec -it dev-postgres psql -U dev "${2:-postgres}"
      ;;
    redis-cli)
      docker exec -it dev-redis redis-cli
      ;;
    ch|clickhouse)
      docker exec -it dev-clickhouse clickhouse-client -u dev --password dev
      ;;
    db-create)
      [ -z "$2" ] && { echo "Usage: dev-stack db-create <name>"; return 1; }
      docker exec dev-postgres psql -U dev -c "CREATE DATABASE \"$2\";" \
        && echo "✓ DB '$2' created"
      ;;
    db-drop)
      [ -z "$2" ] && { echo "Usage: dev-stack db-drop <name>"; return 1; }
      echo "⚠ Drop database '$2'? (y/N)"
      read -r ans
      [ "$ans" = "y" ] && docker exec dev-postgres psql -U dev -c "DROP DATABASE \"$2\";"
      ;;
    nuke)
      echo "⚠ Это снесёт все данные dev-stack. Продолжить? (y/N)"
      read -r ans
      [ "$ans" = "y" ] && docker compose -f "$compose" down -v && echo "✓ Nuked"
      ;;
    *)
      cat <<USAGE
Usage: dev-stack <command>

Lifecycle:
  up | start [svc...]   Start all (or only named services) in background
  down | stop [svc...]  Stop (data persists)
  restart [svc...]      Restart
  pull                  Pull latest images
  status | ps           Show containers
  logs [service]        Tail logs
  nuke                  Stop AND delete all data (confirm)

Access:
  urls                  Print all service URLs (honours DEV_STACK_HOST)
  psql [db]             Open psql (default: postgres)
  redis-cli             Open redis-cli
  ch | clickhouse       Open clickhouse-client
  db-create <name>      Create DB in dev-postgres
  db-drop <name>        Drop DB (confirms)

Stack: Postgres Redis MinIO Qdrant ClickHouse Prometheus Grafana Loki Tempo
       OTel-Collector Langfuse Ollama Metabase CloudBeaver
USAGE
      ;;
  esac
}
