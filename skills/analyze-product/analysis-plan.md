# Product Analysis Plan

Detailed checklist for what to analyze and what to write in `docs/ARCHITECTURE.md`.

Follow SKILL.md for the canonical HQ product scope and write authorization.
Examples below illustrate structure only; service names, ports, infrastructure,
deployment and ADR rationale must come from the actual registered repos.

---

## Phase 1: Inventory

Read the resolved product card and use its registered repo `children`:

```text
hq entity show <company/product> --json
```

Read each child's card to resolve its canonical path. A `.git` file is a valid
worktree marker; a manifest or arbitrary child directory is not registration.
Record unavailable or empty repos as coverage limits without provisioning them.
If no code exists, describe that fact instead of inventing service architecture.

For each registered repo, collect available evidence (read but don't edit):
- `README.md` (top 50 lines)
- `pyproject.toml` / `package.json` / `go.mod` / `Cargo.toml` (full)
- `Dockerfile` (full)
- Entry point file (e.g. `src/main.py`, `cmd/server/main.go`, `src/index.ts`)
- `.env.example` (full)
- `docker-compose.yml` if present
- `.gitlab-ci.yml` / `.github/workflows/*.yml` (high-level)

---

## Phase 2: Per-repo fingerprint

For each repo, derive:

| Attribute | Where to look |
|---|---|
| Language + version | manifest |
| Framework | manifest deps (FastAPI / Gin / Express / etc.) |
| Service type | classify: gateway / orchestrator / worker / store / lib |
| Default port | Dockerfile EXPOSE / config / env defaults |
| Health endpoint | grep `/health` / `/healthz` / `/readyz` in code |
| External deps (network) | grep `httpx.AsyncClient` / `requests.get` / `fetch(` / `nats.connect` / `kafka.Producer` / `grpc.client` |
| Database client | grep `asyncpg` / `psycopg` / `sqlalchemy` / `redis` / `mongo` / `clickhouse` |
| Message bus client | grep `nats` / `aiokafka` / `confluent-kafka` / `pika` (rabbitmq) |
| Internal RPC | grep `grpc` / specific shared client lib |
| Auth | grep `jwt` / `oauth` / `bearer` / specific lib |

Output per repo:
```yaml
repo: barrier
language: Python 3.11
framework: FastAPI 0.110
service_type: gateway
port: 8080
health: /health
external_calls:
  - cortex (HTTP via httpx, localhost:8001 hint)
  - cerebellum (HTTP via httpx, localhost:8002)
db_client: none
mq_client: none
auth: JWT bearer + Redis blacklist
```

---

## Phase 3: Build communication graph

Cross-reference per-repo `external_calls` with sibling repos' declared ports.
Matching ports are a clue, not proof of communication; verify configuration,
call sites and receiving routes before adding an asserted edge.

Output: communication table.

```
Source → Target | Protocol | Endpoints / Topics
barrier → cortex | HTTP/REST (httpx async) | POST /dispatch, GET /status
cortex → synapse | NATS pub-sub | publishes: agent.invoke
nerve → synapse | NATS subscribe | subscribes: agent.invoke
nerve → cerebellum | HTTP/REST | POST /state
vox → cortex | HTTP/REST | POST /callback
```

External services (outside the product):
```
* → Postgres | localhost:5432 (dev-stack)
* → Redis | localhost:6379 (dev-stack)
```

---

## Phase 4: Data flow analysis

For 1-3 main user-facing flows, write a sequence diagram in text:

```
### Main flow: POST /dispatch
1. Client → barrier (auth, validate, rate-limit)
2. barrier → cortex (HTTP /dispatch)
3. cortex → synapse (publish agent.invoke)
4. nerve subscribes → processes → publishes agent.complete
5. cortex polls cerebellum for state
6. cortex → barrier (response)
7. barrier → Client
```

Use mermaid sequence diagrams if user prefers visual:
```mermaid
sequenceDiagram
    Client->>barrier: POST /dispatch
    barrier->>cortex: HTTP /dispatch
    cortex->>synapse: pub agent.invoke
    synapse->>nerve: deliver
    nerve->>cerebellum: write state
    cortex->>cerebellum: read state
    cortex->>barrier: response
    barrier->>Client: response
```

---

## Phase 5: Tech stack distribution

Table:

| Service | Language | Framework | Runtime | DB | MQ |
|---|---|---|---|---|---|
| barrier | Python 3.11 | FastAPI | uvicorn | — | — |
| cortex | Python 3.11 | FastAPI | uvicorn | — | NATS |
| nerve | Python 3.11 | — | python -m | — | NATS |
| synapse | Go 1.22 | — | go binary | — | NATS broker |
| cerebellum | Go 1.22 | chi | go binary | Postgres | — |
| vox | Python 3.11 | FastAPI | uvicorn | — | — |

---

## Phase 6: Deployment topology

From `Dockerfile`, CI configs, k8s manifests:

```
Build: GitLab CI builds Docker images, pushes to GitLab Registry
Deploy: <TODO infer from k8s manifests if present, ask user otherwise>
Orchestration: Kubernetes / Docker Compose / Nomad / bare metal
Environments: dev (local), staging (auto-deploy from dev branch), prod (manual release from main)
```

If can't infer — leave as TODO with concrete questions for user.

---

## Phase 7: Architectural patterns

Identify patterns observed:

- **Event-driven via NATS** — agent work goes async through synapse
- **Centralized state** — cerebellum is single source of truth
- **API gateway** — barrier is sole external entry point
- **Polyglot** — Python for IO-bound services, Go for high-throughput
- **Service mesh не используется** — direct HTTP calls between services
- (...other patterns)

---

## Phase 8: Candidate ADRs

When analysis surfaces non-trivial design decisions, propose ADRs:

```
Proposed ADRs:
1. 0001-message-bus-via-nats.md — why NATS (not Kafka/Rabbit/SQS)
2. 0002-state-centralized-in-cerebellum.md — why one service holds state
3. 0003-no-service-mesh.md — direct HTTP without Istio/Linkerd
4. 0004-polyglot-python-go.md — language choice rationale
```

Each ADR file structure (Michael Nygard format):
```markdown
# 0001. Message bus via NATS

## Status
Draft

## Context
We need async work distribution from cortex to multiple agent workers.
Options considered: Kafka, RabbitMQ, NATS, SQS, Redis pub-sub.

## Decision
Use NATS via synapse service as central message broker.

## Consequences
+ Lightweight, low latency
+ Single binary, easy ops
- No durable storage by default (use NATS JetStream if needed later)
- Lock-in to NATS-specific patterns

## Status history
- <date> Draft proposal
```

Keep candidates in the report unless ADR files are explicitly authorized. Use
unused numbering when writing, retain Draft status, and mark unknown historical
context as TODO. Do not claim the example options were actually considered.

---

## Phase 9: Output

Draft supported findings for `docs/ARCHITECTURE.md` under the canonical product
path in this structure. Persist only within the write scope defined in SKILL.md:

```markdown
# Architecture — ${PROD}

## Overview
<1-2 paragraphs synthesized from Phase 1-2>

## Services
<one section per repo with Phase 2 attributes>

## Inter-service communication
<Phase 3 table + per-pair details>

## Data flow
<Phase 4 sequence diagrams>

## Tech stack distribution
<Phase 5 table>

## Deployment
<Phase 6>

## Architectural patterns
<Phase 7 list>

## Decisions (ADRs)
See `docs/adr/`:
<list from Phase 8>

## Known issues / non-obvious
<from code analysis: deprecated patterns, TODO comments в коде, etc.>
```

Then propose ADR files separately (Phase 8 output).

---

## Limits and caveats — communicate to user

- **Code analysis is heuristic.** Pattern detection works for common frameworks
  but may miss non-standard usage. Mark uncertainty explicitly.
- **External services outside the product** may be invisible. If repo calls `https://api.external.com`,
  note it but ask user to confirm what service that is.
- **Async patterns** harder than sync to trace. Ask user for clarification on
  ambiguous flows.
- **Don't fabricate.** If can't find — write `TODO confirm with user` not guess.
