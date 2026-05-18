dev-stack() {
  local cmd="${1:-status}"
  local compose=~/dotfiles/services/dev-stack/docker-compose.yml
  
  if ! docker ps >/dev/null 2>&1; then
    echo "⚠ Docker не запущен. Запусти OrbStack (Spotlight → OrbStack)."
    return 1
  fi
  
  case "$cmd" in
    up|start)
      docker compose -f "$compose" up -d
      echo ""
      echo "✓ Dev stack running:"
      echo "   Postgres    localhost:5432  (user=dev, pass=dev)"
      echo "   Redis       localhost:6379"
      echo "   Prometheus  http://localhost:9090"
      echo "   Grafana     http://localhost:3030  (user=dev, pass=dev)"
      ;;
    down|stop)
      docker compose -f "$compose" down
      ;;
    restart)
      docker compose -f "$compose" restart
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
  up | start          Start all services in background
  down | stop         Stop services (data persists)
  restart             Restart services
  status | ps         Show running containers
  logs [service]      Tail logs (all or specific)
  nuke                Stop AND delete all data (confirm)

Quick access:
  psql [db]           Open psql (default db: postgres)
  redis-cli           Open redis-cli
  db-create <name>    Create new DB in dev-postgres
  db-drop <name>      Drop DB (confirms)

Services:
  Postgres    localhost:5432  (user=dev, pass=dev)
  Redis       localhost:6379
  Prometheus  http://localhost:9090
  Grafana     http://localhost:3030  (user=dev, pass=dev)
USAGE
      ;;
  esac
}
