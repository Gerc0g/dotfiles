# Dev Stack

Один общий локальный слой инфры для всех проектов. `dev-stack up` поднимает
всё, проекты подключаются по адресам ниже. `dev-stack urls` — живой список.

Собрано **local-first, но server-ready**: ничего не светит в интернет. Когда
переедет на отдельную машину вместе с серверным слоем — compose
тот же (через Tailscale, без открытых портов).

## Сервисы

| Группа | Сервис | Адрес | Доступ |
|---|---|---|---|
| core | Postgres 16 | `:5432` | dev/dev |
| core | Redis 7 | `:6379` | — |
| storage | MinIO (S3) | `:9000` / UI `:9001` | dev / devsecret123 |
| vectors | Qdrant | `:6333` REST, `:6334` gRPC | — |
| olap | ClickHouse | `:8123` | dev/dev |
| obs | Prometheus | `:9090` | — |
| obs | Grafana | `:3030` | dev/dev (Prom/Loki/Tempo pre-wired) |
| obs | Loki | `:3100` | логи |
| obs | Tempo | `:3200` | трейсы |
| obs | OTel Collector | `:4317` gRPC / `:4318` http | единая точка телеметрии |
| llm | Langfuse | `:3001` | LLM-observability (PG+CH+Redis+MinIO) |
| llm | Ollama | `:11434` | локальные модели |
| bi | Metabase | `:3002` | дашборды/BI |
| bi | CloudBeaver | `:8978` | SQL-вьювер (pg + clickhouse) |

Порты конфликт-free: ClickHouse native `9000` не публикуется на хост (его
занимает MinIO) — Langfuse ходит к ClickHouse внутри docker-сети.

## Управление

```bash
dev-stack up                 # всё
dev-stack up qdrant langfuse # только нужное (если RAM поджимает)
dev-stack urls               # все адреса (учитывает DEV_STACK_HOST)
dev-stack status             # контейнеры
dev-stack logs langfuse      # логи сервиса
dev-stack psql [db]          # psql
dev-stack clickhouse         # clickhouse-client
dev-stack db-create <name>   # БД проекта
dev-stack nuke               # снести всё с данными (confirm)
```

Первый `up` тянет образы (~несколько ГБ) и Langfuse мигрирует ClickHouse —
дай ему ~минуту. Полный стек ~3–4 ГБ RAM; точечный `up <svc>` если нужно легче.

## Подключение из проектов

В `.envrc` проекта (direnv подхватит на `cd`):

```bash
H=${DEV_STACK_HOST:-localhost}
export DATABASE_URL="postgresql://dev:dev@$H:5432/<product>__<repo>"
export REDIS_URL="redis://$H:6379/0"
export QDRANT_URL="http://$H:6333"
export CLICKHOUSE_URL="http://dev:dev@$H:8123"
export S3_ENDPOINT_URL="http://$H:9000"   # key=dev secret=devsecret123
export OTEL_EXPORTER_OTLP_ENDPOINT="http://$H:4317"
export LANGFUSE_HOST="http://$H:3001"
export OLLAMA_HOST="http://$H:11434"
```

Создать БД проекта: `dev-stack db-create <product>__<repo>`.
