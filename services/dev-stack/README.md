# Dev Stack

Shared local services для всех твоих проектов.

## Services
- **Postgres 16** — `localhost:5432`, user/pass: `dev/dev`
- **Redis 7** — `localhost:6379`
- **Prometheus** — `http://localhost:9090`
- **Grafana** — `http://localhost:3030`, user/pass: `dev/dev`

## Управление

См. `? dev-stack` или просто `dev-stack` без аргументов.

## Подключение из проектов

В `.envrc` проекта:

```bash
# Local development
export DATABASE_URL=postgresql://dev:dev@localhost:5432/<db-name>
export REDIS_URL=redis://localhost:6379/0

# Или через 1Password если cloud:
# export DATABASE_URL="$(op read 'op://Work-Acme/Postgres-Dev/url')"
```

## Создать БД для проекта

```bash
dev-stack db-create payments
```

## Полностью снести (с данными)

```bash
dev-stack nuke
```
