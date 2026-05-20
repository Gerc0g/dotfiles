# Repo Analysis Plan

Detailed checklist for `analyze-repo` skill — what to read, what to derive,
what to write in `docs/design.md`.

---

## Phase 1: Inventory

Read (don't edit):
- `README.md` (full if reasonable, else top 200 lines)
- Manifest (`pyproject.toml` / `package.json` / `go.mod` / etc.) — full
- `Dockerfile` if present
- `Makefile` / `scripts/` directory
- `.env.example` / `.env.template`
- CI config (`.gitlab-ci.yml` / `.github/workflows/*.yml`)
- Top-level dirs listing: `ls -la src/` or repo root

---

## Phase 2: Module structure

Walk source root (`src/`, `lib/`, `cmd/`, `internal/`, etc.). For each subdir:

```bash
# Inventory subdirs
for d in src/*/; do
  echo "=== $d ==="
  ls "$d" | head -10
  # Read 1-2 representative files
done
```

Build module tree with responsibility per subdir. Example output:

```
src/
├── api/             ← HTTP routes + handlers
│   ├── routes.py    ← FastAPI app definition, route registration
│   └── deps.py      ← FastAPI Depends() factories
├── services/        ← business logic, no HTTP/DB direct
│   ├── auth.py
│   └── dispatch.py
├── adapters/        ← external service clients
│   ├── cortex_client.py
│   └── redis_client.py
├── models/          ← Pydantic models, request/response schemas
├── db/              ← SQLAlchemy models, Alembic migrations
└── utils/           ← shared helpers (result.py, logging.py)
```

---

## Phase 3: Entry points

Identify all entry points to code:

| Type | Search | Example |
|---|---|---|
| HTTP main | `if __name__ == "__main__"` + uvicorn / FastAPI app | `src/main.py` |
| CLI command | `[project.scripts]` in pyproject / `bin/` dir | `cli/admin.py` |
| Worker | grep `nats.subscribe` / `aiokafka.consume` / scheduled | `workers/consumer.py` |
| Migration | Alembic / dbmate | `migrations/` |
| Test | pytest entry, jest config | `tests/conftest.py` |

---

## Phase 4: External integrations

For each external system, derive:

| External | Detection | Output |
|---|---|---|
| HTTP services | grep `httpx.AsyncClient` / `requests.Session` / `fetch(` | Service name, base URL, auth method, retry policy |
| Databases | grep `asyncpg` / `sqlalchemy` / `pymongo` | Engine, connection pattern, ORM or raw SQL |
| Caches | grep `redis` / `memcached` | Pattern: cache-aside / write-through, TTL strategy |
| Message bus | grep `nats` / `kafka` / `pika` | Producer/consumer, topics, schema |
| Auth | grep `jwt` / `oauth` / `python-jose` | Token type, validation strategy |
| Storage | grep `boto3` / `aws-sdk` / `gcs` | S3-compat? specific bucket convention? |
| Email/SMS | grep `mailgun` / `sendgrid` / `twilio` | provider, templates location |
| Search | grep `elasticsearch` / `meilisearch` / `typesense` | which engine, indexing pattern |

Output table в design.md:

```markdown
## External dependencies

| Service | Type | Client | Retry / Fail-mode |
|---|---|---|---|
| cortex | HTTP | httpx async | 3x exponential backoff, return 503 |
| Postgres | DB | asyncpg + SQLAlchemy 2.x | Pool 20, retry on stale connection |
| Redis | Cache | redis-py async | Fail-open if down (log warning) |
| Auth | JWT | python-jose | RS256, public key from env |
```

---

## Phase 5: Key abstractions

Identify custom types and base classes that define how the code is shaped:

```bash
# Search for likely abstractions
grep -rn "class.*Result\|class.*Either\|class.*Option" src/
grep -rn "class.*Base\|class.*Abstract" src/
grep -rn "Protocol\[" src/
grep -rn "@runtime_checkable" src/
```

For each found abstraction:
- Show 3-10 line snippet
- Explain в plain language
- Show usage example from real code

Example output:
```markdown
### Result[T]

Replaces exception-based error handling. Defined in `src/utils/result.py`.

```python
@dataclass
class Result(Generic[T]):
    ok: T | None = None
    err: Error | None = None
    
    def is_err(self) -> bool: return self.err is not None
```

All `api.*` methods return `Result[T]` — NEVER raise. Caller pattern:

```python
result = await api.create_user(payload)
if result.is_err():
    return error_response(result.err)
return success_response(result.ok)
```
```

---

## Phase 6: Non-obvious conventions

Most-valuable section. Find counterintuitive things from:

### 6a. Recent fix commits
```bash
git log --oneline --grep="^fix" -20
git log --oneline -50 | grep -iE "fix|hotfix|patch"
```
Read each fix diff — what was wrong, what's the now-enforced rule?

### 6b. WARNING / IMPORTANT / NOTE comments в коде
```bash
grep -rn "# IMPORTANT\|# WARNING\|# NOTE\|# HACK\|# TODO" src/
```

### 6c. Counter-intuitive imports / module placement
- `lib/` (3rd-party) vs `src/` (own code)
- Why a helper is here vs there

### 6d. Explicit "don't do X" patterns
- linter ignore comments — what's being suppressed and why
- `# type: ignore` clusters

For each, write 1 sentence: "X is done THIS way because Y".

Aim for 5-15 non-obvious patterns. If <5 found — repo is straightforward, no
counter-intuitive things, write `(nothing non-obvious found — codebase is conventional)`.

---

## Phase 7: Hot paths

Find performance-critical / most-called code:

```bash
# Most-imported files
grep -rh "^from src" src/ | sort | uniq -c | sort -rn | head -10

# Most-modified files (recent activity)
git log --pretty=format: --name-only | grep src/ | sort | uniq -c | sort -rn | head -10
```

List in design.md as "hot paths" — places to be especially careful editing.

---

## Phase 8: Tests architecture

From `tests/` or test dir:

```bash
ls tests/
# unit/  integration/  e2e/
cat tests/conftest.py  # fixtures
```

Document:
- Split (unit / integration / e2e)
- Mocking strategy (autouse fixtures, env injection)
- Where dev-stack is used (integration tests need running Postgres?)
- How to run only one test, one file, one tag

---

## Phase 9: Output `docs/design.md`

Structure:

```markdown
# Design — ${REPO}

## Purpose
<1 paragraph from Phase 1 README + Phase 3 entry points>

## Architecture style
<layered / hexagonal / function-based / lambda-style — based on Phase 2>

## Module structure
<Phase 2 tree with responsibilities>

## Entry points
<Phase 3 list>

## External dependencies
<Phase 4 table>

## Key abstractions
<Phase 5 snippets>

## Non-obvious conventions
<Phase 6 bullets — most valuable section>

## Hot paths
<Phase 7 list>

## Tests
<Phase 8 structure>

## Decisions reference
See `docs/adr/`:
<list from Phase 10>

## Known issues / tech debt
<from grep TODO/FIXME/HACK comments>
```

---

## Phase 10: Candidate ADRs

When analysis surfaces non-trivial decisions, propose ADRs:

Typical repo-level ADRs:
- "Why Result type, not exceptions"
- "Why this DB schema design"
- "Why no ORM (or specific ORM)"
- "Why this auth scheme"
- "Why these middleware in this order"

ADR file format (Michael Nygard):
```markdown
# 0001. <slug>

## Status
Accepted (date)

## Context
<problem statement, options considered>

## Decision
<chosen option + reasoning>

## Consequences
+ <positive outcomes>
- <negative outcomes / trade-offs>
```

Mark each ADR as "draft" until user reviews and approves.

---

## Phase 11: Optional AGENTS.md update

Suggest top 3-5 non-obvious patterns from Phase 6 для embedding в repo
AGENTS.md `## Non-obvious patterns` section.

Format for user:
> "Found 12 non-obvious patterns. Top 3-5 для AGENTS.md (показываются в каждой сессии):
> 1. ...
> 2. ...
> ...
> Apply to AGENTS.md? (y/n/edit selection)"

Only edit AGENTS.md with explicit user approval.

---

## Quality bar

Good design.md (in <300 lines) lets a new contributor:
- Understand purpose в 30 секунд
- Know where to look for X в 1 минуту
- Avoid common mistakes (Phase 6 patterns)
- Trace request lifecycle если хочет (Phase 3-4)

If draft fails any of above — re-analyze that area, ask user clarifying questions.

---

## Limits

- **Static analysis only.** Runtime behavior (perf, concurrency races, OOM patterns)
  not derivable from code reading.
- **Tests are signals, not truth.** Tests may pass while production fails — note
  test coverage gaps.
- **Branch history may differ from main.** Analyze main/dev unless user specifies.
- **Don't fabricate confidence.** "Looks like X" beats "is X" when uncertain.
