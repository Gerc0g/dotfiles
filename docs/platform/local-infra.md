# Local Infrastructure

Reference for local services, dev-stack, Docker/OrbStack, Postgres, Redis,
MinIO, Prometheus, Grafana, MLflow, ports, and local runtime setup.

## Shared dev-stack

Default local services are provided by `dev-stack`:

- Postgres: `localhost:5432`, user `dev`, pass `dev`, db `postgres`;
- Redis: `localhost:6379`, no auth;
- Prometheus: `http://localhost:9090`;
- Grafana: `http://localhost:3030`, `dev` / `dev`;
- MinIO S3 API: `http://localhost:9000`, key `dev`, secret `devsecret123`;
- MinIO Console: `http://localhost:9001`;
- MLflow: `http://localhost:5000`.

Commands:

```bash
dev-stack up
dev-stack down
dev-stack status
dev-stack psql [db]
dev-stack redis-cli
dev-stack db-create <name>
dev-stack db-drop <name>
```

## Rules

- Do not add shared Postgres/Redis/MinIO/MLflow services to a repo compose file
  unless the repo explicitly owns that infrastructure.
- Do not suggest `brew install postgresql`, `redis`, or `minio` for local dev.
- For S3-compatible local storage, use MinIO endpoint
  `http://localhost:9000`.
- For ML experiment tracking, use MLflow at `http://localhost:5000` when the
  task needs persistent experiment logs.

## Repo Setup

When adding local runtime docs:

- prefer existing repo commands;
- keep `Makefile` as the canonical interface;
- document required env vars without values;
- make `verify` local and non-destructive.

Deep reference: `~/dotfiles/agent-profiles/PLATFORM.md` section "Local
infrastructure".
