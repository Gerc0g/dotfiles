# Local Infrastructure

> **Canonical source:** `~/dotfiles/services/dev-stack/README.md` + live `dev-stack urls`.
> This page is a short pointer; the compose file and `dev-stack urls` are the truth.

## Shared dev-stack

One shared infra layer for all projects (`services/dev-stack/docker-compose.yml`),
running 24/7 on the home Ubuntu box. Host = `$DEV_STACK_HOST` (default **`gerc0g`**,
tailnet; set `localhost` for a Mac-local stack). All addresses below are
`$DEV_STACK_HOST:<port>`.

Services (18): Postgres `:5432` (dev/dev), Redis `:6379`, MinIO `:9000`/UI `:9001`
(dev/devsecret123), Qdrant `:6333` REST `:6334` gRPC, ClickHouse `:8123` HTTP
(dev/dev; native `:9000` internal-only), Prometheus `:9090`, Grafana `:3030`
(dev/dev, Prom/Loki/Tempo pre-wired), Loki `:3100`, Tempo `:3200`, OTel Collector
`:4317`/`:4318`, Langfuse `:3001`, Ollama `:11434`, Open-WebUI `:8083`, Metabase
`:3002`, CloudBeaver `:8978`, Redis-Commander `:8081`, Traefik `:80`/dash `:8080`,
Homepage `http://$DEV_STACK_HOST/`.

There is **no MLflow** service — only a MinIO `mlflow` bucket. Use Langfuse for LLM
tracing; run MLflow yourself if a project needs the tracking server.

## Connect a project (self-onboard)

```bash
cd <repo>
dev-stack connect      # appends endpoints to .envrc, creates DB <product>__<repo>, direnv allow
dev-stack doctor       # checks stack reachability + whether this repo is wired
```

`dev-stack connect` writes a managed block to `.envrc`:
`DATABASE_URL`, `REDIS_URL`, `QDRANT_URL`, `CLICKHOUSE_URL`, `S3_ENDPOINT_URL`,
`OTEL_EXPORTER_OTLP_ENDPOINT`, `LANGFUSE_HOST`, `OLLAMA_HOST` — all keyed off
`${DEV_STACK_HOST}` (so flipping the host re-points everything). `dev-stack envrc`
prints the block without writing.

## Management

The stack lives on the server. `dev-stack` is remote-aware: lifecycle/exec run on
`$DEV_STACK_HOST` via `ssh $DEV_STACK_SSH` (default `ubuntu-server`) when the host
is not local. `dev-stack up/down/status/logs`, `psql`, `redis-cli`, `clickhouse`,
`db-ensure`/`db-create`/`db-drop`, `urls`, `nuke`.

## Rules

- Telemetry (metrics/logs/traces) is **pushed via OTLP** to the OTel Collector
  (`$DEV_STACK_HOST:4317`); Prometheus does NOT scrape your app's `/metrics`.
- Do not add shared Postgres/Redis/MinIO/Qdrant/ClickHouse/Langfuse to a repo's
  `docker-compose.yml`; use the shared stack.
- Do not `brew install postgresql`/`redis`/`minio`, and don't use cloud AWS S3 for
  local dev — use MinIO `:9000`.

Deep reference: `~/dotfiles/agent-profiles/PLATFORM.md` ("Local infrastructure").
