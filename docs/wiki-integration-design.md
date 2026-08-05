# Karpathy LLM Wiki Integration Design

**Status:** APPROVED design from Oracle (GPT-5.4 Pro) review, 2026-05-19.
**Next step:** Implementation by another agent.
**Source:** `~/dotfiles/.wiki-context/oracle-prompt-wiki-integration.md` (prompt) → Oracle response (this file).

---

## Core decisions

1. **Pure symlink / filesystem-first** — primary storage = plain markdown in WikiPedik. NOT MCP (too much moving parts). MCP can be added later as read-only search layer.

2. **Topology mirrors Prokectfiles**: `vault/20-projects/<co>/<prod>/repos/<repo>/`. Agent derives namespace from cwd.

3. **Three symlinks per repo** (different write permissions):
   ```
   docs/knowledge        → repo memory       (DEFAULT WRITE)
   docs/product-knowledge → product memory   (READ only, except sync)
   docs/company-knowledge → company memory   (READ only, except promotion)
   ```

4. **Company = privacy firewall**. Separate git per company. No symlinks cross-company. Root `20-projects/index.md` is non-sensitive manifest only.

5. **Lesson promotion ladder**:
   ```
   _inbox.md (raw)
     ↓ wiki sync command
   lessons.md / debugging-stories.md / gotchas.md (curated)
     ↓ repeated 2+ repos
   product/shared/*.md
     ↓ repeated 2+ products
   company/shared/*.md
     ↓ binding decision
   docs/adr/*
     ↓ canonical
   docs/design.md / ARCHITECTURE.md
   ```
   **Canonical docs always win over wiki memory.** Wiki = experiential ("why/how we learned"), not canonical architecture.

---

## What lives where

### Canonical (in repo `docs/`)
- `docs/design.md` — current architecture
- `docs/ARCHITECTURE.md` — product-level architecture
- `docs/adr/*` — binding decisions
- `AGENTS.md` — universal rules for the agent

### Experiential memory (in vault, symlinked to repo)
- `_inbox.md` — raw captures, auto-write by agent
- `lessons.md` — curated durable lessons
- `debugging-stories.md` — root causes, failed approaches
- `gotchas.md` — counter-intuitive things that bite
- `decisions-not-adr.md` — decisions too informal for ADR yet
- `open-questions.md` — things to investigate
- `links.md` — pointers to commits, paths, ADRs
- `index.md` — repo memory catalog

### Cross-cutting (in vault, at product/company levels)
- `product/shared/patterns.md` — patterns shared across repos in product
- `product/shared/gotchas.md`, `interfaces.md`, `integration-points.md`
- `product/index.md` — lesson catalog across all repos in product
- `product/log.md` — chronological log
- `product/open-questions.md` — product-level uncertainties
- `product/health.md` — last lint status
- `company/shared/cross-product-lessons.md` — patterns across products
- `company/index.md`, `log.md`, `health.md`, `privacy.md`

---

## Folder structure

```
~/Desktop/WikiPedik/dev/20-projects/
  index.md            ← non-sensitive root manifest
  README.md
  _templates/
    lesson-entry.md
    debugging-story.md
    open-question.md
    wiki-sync-checklist.md

  neurodesk/
    .git/             ← own git repo (private)
    index.md
    log.md
    health.md
    privacy.md
    shared/
      patterns.md
      gotchas.md
      glossary.md
      cross-product-lessons.md
      decisions-not-adr.md
      playbooks/{debugging,release,migration}.md
    agents/
      index.md
      log.md
      health.md
      open-questions.md
      shared/
        patterns.md, gotchas.md, interfaces.md,
        integration-points.md, decisions-not-adr.md,
        debugging-playbook.md
      repos/
        cortex/
          index.md
          _inbox.md
          lessons.md
          debugging-stories.md
          gotchas.md
          decisions-not-adr.md
          open-questions.md
          links.md
        <other-repo>/ ...
    legacy/   ...
    saas/     ...
    infra/    ...
    wiki/     ...

  justchimera/        ← own .git, separate
    index.md, log.md, health.md, privacy.md
    shared/
    <product>/
```

---

## Default behavior

### Read (automatic)
- On `setup-context`: print memory paths
- Before plan for nontrivial task: read `docs/{knowledge,product-knowledge,company-knowledge}/index.md`
- For debug/design/migration/cross-repo task: `rg --follow` over memory paths

### Read (NOT automatic)
- Full wiki scan
- Reading another company namespace
- Reading `research/` or `40-personal/`

### Write (automatic)
- Append to `docs/knowledge/_inbox.md` ONLY when durable signal:
  - Root cause of nontrivial bug
  - Failed approach
  - Cross-repo invariant
  - Production gotcha
  - Design tradeoff not yet ADR
  - Repeated pattern
  - Decision not yet ADR
  - Debugging story worth keeping

### Write (NOT automatic, requires explicit command)
- `lessons.md`, `debugging-stories.md`, `gotchas.md` — only on "запиши урок" or `wiki sync`
- `product/shared/*` — only when promoting cross-repo pattern
- `company/shared/*` — only when promoting cross-product pattern
- Another repo's memory leaf

---

## Lint at 4 levels

| Level | Trigger | Scope |
|---|---|---|
| Repo | After complex debug; `_inbox.md` > 10 entries; before closing big task | `docs/knowledge/*` |
| Product | Weekly; after 10+ new lessons; before architecture review | `vault/<co>/<prod>/` |
| Company | Monthly; after new product onboarded; pattern repeats across products | `vault/<co>/` |
| Root | Structural only; non-sensitive | `vault/20-projects/index.md` |

---

## 10 workflows (full prompts in oracle response)

1. **Memory-aware plan** — before debug/design tasks, auto
2. **"Запиши урок"** — explicit after debugging
3. **Capture failed approach** — to `gotchas.md`
4. **Wiki sync after commit** — manual, validates _inbox → curated
5. **Cross-repo check** — auto when task mentions "уже было"
6. **Product open questions** — manual before sprint planning
7. **Promote lesson to ADR candidate** — manual
8. **Oracle review with memory** — manual for deep review
9. **Lint product memory** — manual weekly
10. **Start task with "what we know"** — manual for complex tasks

---

## Symlink bootstrap

Script generates skeleton + creates 3 symlinks per repo + adds to `.git/info/exclude`. Located in:
- `~/dotfiles/scripts/wiki-bootstrap-product.sh` (to be created)

Usage:
```bash
wiki-bootstrap-product neurodesk agents
```

Pilot target: only `neurodesk/agents` first (6 repos, 30 min setup).

---

## Risks ranked by blast radius

1. **Cross-company leakage** (highest) — mitigated by company-scoped symlinks + AGENTS firewall + separate .git
2. **Wiki becomes competing source of truth** (high) — canonical docs always win, lessons must promote to ADR if binding
3. **Maintenance burden from noisy captures** (medium-high) — strict durable-signal criteria; _inbox-only auto-writes
4. **Stale lessons mislead agents** (medium) — `Last verified` field, lint checks
5. **Symlink breakage** (medium) — re-runnable bootstrap
6. **Secret/sensitive log capture** (high if data sensitive) — sanitized summaries only, no raw logs, secret scanning in lint

---

## launch / wikipedik changes

- **Do NOT add 5th tmux window**. Memory access via symlinks in existing windows.
- **setup-context addition**: print memory paths + suggested `rg` command + check symlinks
- **wikipedik-project command**: new variant `wikipedik project <co> <prod>` opens only that company's project memory for lint/synthesis

---

## Final skill list (after community research + user decisions)

### Worker skills (2) — `~/.codex/skills/`

Worker = project agent in launch tmux (plan/code windows). Reads memory before planning. Writes only to `_inbox.md`.

| # | Skill | Source | Adoption | Purpose |
|---|---|---|---|---|
| 1 | `wiki-context-pack` | [Ar9av/obsidian-wiki](https://github.com/Ar9av/obsidian-wiki/tree/main/.skills/wiki-context-pack) | take-as-is | Token-bounded read of relevant wiki pages before planning |
| 2 | `lesson-append` | new custom (~60 lines) | write from scratch | Append durable lesson YAML block to `_inbox.md`, nothing else |

### Curator skills (7) — `~/.codex/skills/`

Curator = wiki agent in wikipedik tmux. Reads vault, edits curated pages, synthesizes patterns.

| # | Skill | Source | Adoption | Purpose |
|---|---|---|---|---|
| 3 | `source-ingest` | [kfchou/wiki-skills/wiki-ingest](https://github.com/kfchou/wiki-skills/tree/main/skills/wiki-ingest) | take-as-is (rename) | External raw source (`00-inbox/`) → `10-wiki/sources/` |
| 4 | `agent-history-ingest` | merged from [Ar9av/codex-history-ingest](https://github.com/Ar9av/obsidian-wiki/tree/main/.skills/codex-history-ingest) + [Ar9av/claude-history-ingest](https://github.com/Ar9av/obsidian-wiki/tree/main/.skills/claude-history-ingest) | adapt + merge | Mine JSONL session histories → wiki entries with delta tracking |
| 5 | `inbox-drain` | inspired by [Ar9av/wiki-stage-commit](https://github.com/Ar9av/obsidian-wiki/tree/main/.skills/wiki-stage-commit) | new custom (block-level) | `_inbox.md` candidates → `lessons.md` / `gotchas.md` / `debugging-stories.md` / etc. |
| 6 | `wiki-synthesize` | [Ar9av/obsidian-wiki/wiki-synthesize](https://github.com/Ar9av/obsidian-wiki/tree/main/.skills/wiki-synthesize) | adapt (drop taxonomy dep) | Cross-repo pattern detection → `product/shared/patterns.md` |
| 7 | `wiki-lint` | [kfchou/wiki-skills/wiki-lint](https://github.com/kfchou/wiki-skills/tree/main/skills/wiki-lint) | take-as-is | Find orphans, dead links, contradictions, stale |
| 8 | `wiki-status` | [Ar9av/obsidian-wiki/wiki-status](https://github.com/Ar9av/obsidian-wiki/tree/main/.skills/wiki-status) | adapt (delta section only) | What's ingested, what's pending |
| 9 | `autoresearch` | [AgriciDaniel/claude-obsidian/autoresearch](https://github.com/AgriciDaniel/claude-obsidian/tree/main/skills/autoresearch) | adapt (drop DragonScale) | Web research for gap-filling |

### Hooks (1) — at session start

| # | Hook | Source | Adoption | Purpose |
|---|---|---|---|---|
| 10 | `auto-load` | inspired by [AgriciDaniel/AGENTS.md bootstrap](https://github.com/AgriciDaniel/claude-obsidian/blob/main/AGENTS.md) + `hooks/hooks.json` | new custom | On codex/claude SessionStart in repo, inject `hot.md` context |

### Shell scripts (1) — `~/dotfiles/scripts/`

| # | Script | Source | Purpose |
|---|---|---|---|
| 11 | `wiki-bootstrap-product.sh` | Oracle response §4 (already drafted) | Create `20-projects/<co>/<prod>/repos/<repo>/` structure + 3 symlinks per repo |

### Dropped from initial plan

- ❌ `wiki-init` — vault already exists, redundant with `wiki-bootstrap-product.sh`
- ❌ `wiki-audit` — citation audit not needed at this stage
- ❌ `skill-creator` — use anthropic-skills/skill-creator if needed later
- ⏳ `wiki-rebuild` — deferred until proven need

---

## Storage layout for skills (during implementation)

Draft skills are staged in `~/dotfiles/skills-stash/wiki/` (not yet activated):

```
~/dotfiles/skills-stash/wiki/
├── README.md                       ← installation instructions
├── worker/
│   ├── wiki-context-pack/SKILL.md  ← fetched from Ar9av (take-as-is)
│   └── lesson-append/SKILL.md      ← new custom (in this repo)
├── curator/
│   ├── source-ingest/SKILL.md      ← fetched from kfchou (renamed)
│   ├── agent-history-ingest/SKILL.md  ← merged from Ar9av's two
│   ├── inbox-drain/SKILL.md        ← new custom (in this repo)
│   ├── wiki-synthesize/SKILL.md    ← fetched from Ar9av (adapted)
│   ├── wiki-lint/SKILL.md          ← fetched from kfchou (take-as-is)
│   ├── wiki-status/SKILL.md        ← fetched from Ar9av (adapted)
│   └── autoresearch/SKILL.md       ← fetched from AgriciDaniel (adapted)
├── hooks/
│   ├── auto-load-codex.sh          ← new custom
│   └── auto-load-claude.sh         ← new custom
└── scripts/
    └── wiki-bootstrap-product.sh   ← from Oracle response
```

Once tested, symlink/copy into:
- `~/.codex/skills/<worker-skills>` and `~/.claude/skills/<worker-skills>`
- `~/.codex/skills/<curator-skills>`
- `~/.codex/hooks/` and `~/.codex/hooks/`
- `~/dotfiles/scripts/wiki-bootstrap-product.sh`

---

## Open implementation items (for next agent)

1. **Fetch take-as-is/adapt sources** — run `~/dotfiles/scripts/fetch-wiki-skills.sh` to clone community repos and stage SKILL.md files
2. **Custom skills already drafted in repo:**
   - `~/dotfiles/skills-stash/wiki/worker/lesson-append/SKILL.md`
   - `~/dotfiles/skills-stash/wiki/curator/inbox-drain/SKILL.md`
   - `~/dotfiles/skills-stash/wiki/hooks/auto-load-{codex,claude}.sh`
3. **Adapt fetched sources** per notes in each adoption note (drop taxonomy from synthesize, extract delta from status, drop DragonScale from autoresearch, merge history-ingests)
4. **Update PLATFORM.md** — add "Persistent developer memory" rule (Oracle response §8)
5. **Update `dev/AGENTS.md`** — add "Project memory" section (Oracle response §6)
6. **Update `AGENTS.md.repo.tmpl`** — add WikiPedik section (Oracle response §7)
7. **Update `setup-context`** — print memory paths on repo session start
8. **Update `wikipedik`** — add `project <co> <prod>` variant
9. **Write `wiki-bootstrap-product.sh`** — port from Oracle response §4
10. **Set up separate git** for `20-projects/neurodesk/` and `20-projects/justchimera/` (each company own private repo, gitignored from vault root)
11. **Activate skills** — symlink/copy from `skills-stash/` to right profile dirs
12. **Pilot** — bootstrap `neurodesk/agents` (6 repos), run 7 sanity checks from Oracle §16, do first real lesson + first sync

---

## Three alternatives (rejected as primary)

- **Pure symlink (filesystem-first)** ✅ chosen — simplest, no service dependency
- **MCP server (e.g. `dfalci/mcp-advwiki`)** ❌ — overkill for 16 repos, adds moving parts. Add later if grep+index becomes insufficient.
- **Hybrid (symlinks for writes + MCP for read search)** ❌ — confusing dual paths. Adopt only after pilot pain visible.

---

## Tests / sanity checks

7 checks defined in Oracle response §16:
1. Expected dirs exist
2. Every repo has links
3. Links don't enter other company
4. Links not committed
5. Grep finds memory across product
6. Agent behavior test (memory-aware plan)
7. Write behavior test (only _inbox modified)

---

## How it works — simple explanation

### Two agent roles: Worker and Curator

```
WORKER (project agent)                  CURATOR (wiki agent)
═════════════════════                   ════════════════════
where: launch tmux (plan/code)          where: wikipedik tmux
context: repo + current task            context: vault + synthesis
model: codex/claude                     model: codex in wikipedik
                                        
AUTO READS:                             EXPLICITLY INVOKED:
- indexes before planning               - wiki sync
- relevant memory paths via rg          - wiki lint
                                        - promote-to-ADR
AUTO WRITES:                            
- _inbox.md when durable signal         WRITES:
  (root cause / failed approach /       - lessons.md
   cross-repo invariant / etc)          - debugging-stories.md
                                        - gotchas.md
DOES NOT WRITE:                         - product/shared/*.md
- lessons.md / curated files            - company/shared/*.md
- another repo's memory leaf            - indexes / logs
- another company namespace             
                                        DOES NOT WRITE:
                                        - _inbox.md (only reads to process)
```

Both work on the same filesystem via symlinks. No locks needed because:
- Worker only appends to `_inbox.md` (single-writer file)
- Curator only edits curated files (worker never touches)

### Daily flow

**1. Start of work — Worker reads memory**

```
launch neurodesk agents cortex
→ plan window opens, codex starts
→ setup-context prints memory paths
```

When you ask Worker to plan a task, it (by rules in PLATFORM.md):
1. Reads `docs/knowledge/index.md` (repo memory)
2. Reads `docs/product-knowledge/index.md` (product memory)
3. Reads `docs/company-knowledge/shared/*.md` (cross-product patterns)
4. Reports: "Found prior lesson cortex/lessons.md#2026-01-12 about cache invalidation, will incorporate"
5. Builds plan considering existing knowledge

This is **automatic read**. Worker checks memory before recommending anything.

**2. During work — Worker captures durable lessons**

When Worker hits one of these signals:
- Root cause of nontrivial bug
- Failed approach worth not repeating
- Cross-repo invariant
- Production gotcha
- Repeated pattern in codebase
- Design tradeoff not yet ADR
- Decision worth keeping

It **automatically appends** to `_inbox.md`:

```markdown
## [2026-05-19] capture | idempotency key collision under concurrent retries

Scope: neurodesk/agents/cortex
Tags: #idempotency #concurrency #retries
Source: commit abc123
Status: candidate
Notes:
- hash(payload) alone insufficient — concurrent retries create dupes
- added timestamp_bucket → unique
- failed approach: DB-level lock — too slow under load
```

This is **automatic write**. Worker doesn't ask. Just adds to inbox.

**3. After many tasks — Curator processes inbox**

When inbox grows (>10 entries) or before sprint planning, you switch to wikipedik:

```
wikipedik project neurodesk agents
→ tmux opens with vault root for this product
→ codex starts in vault context (clean, no project context)

> wiki sync neurodesk/agents/cortex
```

Curator (by rules in `dev/AGENTS.md`):
1. Reads `repos/cortex/_inbox.md` — all candidate entries
2. **Deduplicates** — merges related captures
3. **Decides what's durable** — keeps validated, drops noise
4. **Promotes** to right file:
   - Generic lesson → `lessons.md`
   - Production gotcha → `gotchas.md`
   - Bug story → `debugging-stories.md`
   - Open question → `open-questions.md`
   - Decision pre-ADR → `decisions-not-adr.md`
5. **Updates indexes** — repo `index.md`, product `index.md`, product `log.md`
6. **Cross-repo promotion** — if pattern seen in 2+ repos → `product/shared/patterns.md`
7. **Empties** `_inbox.md` (keeps archive backup)

This is **explicit command** — Curator never runs in background.

**4. Periodic lint — Curator finds rot**

Weekly or monthly:
```
> wiki lint neurodesk/agents
```

Curator checks:
- Contradictions (one lesson says NATS, another says Kafka)
- Stale claims (`Last verified` too old, code changed)
- Orphan pages (nothing links to them)
- Promotion candidates (lesson now repeats in 3 repos → push to shared)
- Privacy violations (anything mentioning another company)
- Secret leaks (passwords/tokens/raw logs)

Writes findings to `health.md` and appends to `log.md`.

**5. Promotion to ADR**

If lesson becomes binding rule across the product:
```
> promote lesson cortex/lessons.md#idempotency to ADR
```

Curator drafts ADR text. You review. ADR file created in repo's `docs/adr/`. Wiki page stays as **historical reasoning + backlink** to ADR.

### Why two roles instead of one

**Context isolation**: Worker has rich context about current task (50+ turns deep into debug). If Worker tried to curate inbox, its judgment about what's "durable" would be biased by current focus.

**Different writeprivileges**: Worker writes only `_inbox.md` (append-only, low stakes). Curator writes curated files (high stakes, requires thoughtful synthesis). Separating them prevents Worker from accidentally polluting `lessons.md` with low-signal noise.

**Cross-repo synthesis**: Curator looks across multiple repos/products. Worker only sees one repo. Curator job — find patterns Worker can't see.

**Trigger discipline**: Worker writes inbox automatically (no friction). Curator runs only when invoked. Means you control when curation happens — not interrupted mid-debug.

### Key invariant: Canonical docs always win

```
docs/design.md, ARCHITECTURE.md, ADRs  ← canonical, current truth
                  │
                  │ if wiki conflicts:
                  │   update wiki, not docs
                  │ if wiki teaches new binding rule:
                  │   promote to ADR, update docs
                  ▼
docs/knowledge, product-knowledge       ← experiential memory
  "why we learned X"
  "this approach failed because Y"
  "remember this pattern next time"
```

Wiki is **how / why we got here**, not **what is now**.

### What user does explicitly vs automatically

| Action | Who | Trigger |
|---|---|---|
| Read memory before plan | Worker | Automatic for nontrivial tasks |
| Append to `_inbox.md` | Worker | Automatic when durable signal |
| Process inbox → curated | Curator | Explicit: `wiki sync` |
| Promote across repos | Curator | Explicit: during sync or lint |
| Lint health | Curator | Explicit: weekly/monthly |
| Promote to ADR | Curator | Explicit: when binding |
| Edit lessons manually | You (human) | In Obsidian or via vault paths |

Both you and agents work on same files. Two-direction sync via symlinks.

### Mental model summary

```
PROJECTS  →  symlinks  →  VAULT  ←  Obsidian / human edits
   ↑                        ↑
   │                        │
WORKER                   CURATOR
auto reads,              explicit sync,
auto inbox-writes        curates archive,
                         finds patterns,
                         lints health
```

Wiki = compiled experiential memory. Built by Curator from Worker's raw captures. Read by Worker before planning. Edited by you in Obsidian when needed. All same plain markdown.

---

## What goes into wiki vs what stays in repo docs

### Core distinction

```
docs/  = "MAP OF WHAT IS"      (current state, ships with code, updates with code)
wiki/  = "JOURNAL OF WHY"       (history, reasoning, experience, not derivable from code)
```

Don't contradict the map with the journal. Don't duplicate the journal into the map.

### Rule of thumb

Ask 4 questions for any piece of information:

1. **Can it be derived from code/manifests automatically?** YES → docs/ (or auto-generated). Wiki captures only what cannot be re-derived.
2. **Does it change when code changes?** YES (state) → docs/. NO (history, reasoning) → wiki/.
3. **Should an external person see it?** (new dev, auditor, customer, freelancer) YES → docs/ (carefully reviewed). Internal-only → wiki/.
4. **Is this "what currently exists" or "what we learned along the way"?**
   - "We use NATS for X" → docs/ (current state)
   - "We chose NATS over Kafka because Y" → wiki/ (reasoning)
   - "We tried switching back to Kafka, it failed because Z" → wiki/ (history)

### What stays in repo docs/

Canonical, current truth. Reviewed via PR. Synced with code in same commit.

- `docs/design.md` — current architecture of this repo
- `docs/ARCHITECTURE.md` — current architecture of this product (mermaid, services, data flow)
- `docs/adr/*.md` — binding architectural decisions
- `docs/openapi.yaml` (or similar) — API spec (source for code generation)
- `AGENTS.md` (all levels) — universal rules
- `README.md` — repo summary, how to run
- `docs/runbooks/*.md` — operational procedures
- Auto-generated lists (endpoints, deps, schemas) — generated from code

### What goes into wiki

Experiential, historical, "why we got here". Edited by Curator (explicit sync) or human (Obsidian).

| Category | File | Example |
|---|---|---|
| **Lessons learned** | `lessons.md` | "asyncio.gather without return_exceptions=True leaks tasks on exception" |
| **Failed approaches** | `gotchas.md` / `debugging-stories.md` | "Tried Redis Cluster mode — broke our SET pattern; rolled back" |
| **Production gotchas** | `gotchas.md` | "Never restart auth-svc pod without draining connections first" |
| **Debugging stories** | `debugging-stories.md` | Full narrative of complex bug investigation |
| **Cross-repo patterns** | `product/shared/patterns.md` | "Same N+1 issue appeared in cortex, backend, synapse — pattern" |
| **Pre-ADR decisions** | `decisions-not-adr.md` | "Considering switching message bus, leaning NATS; not yet formal ADR" |
| **Open questions** | `open-questions.md` | "Why does cortex use Redis SET while backend uses HSET? Investigate before next migration" |
| **Counter-intuitive code** | `gotchas.md` | "routing.yaml fail-closes, calibration.yaml fail-opens — similar files, opposite semantics" |
| **History of alternatives** | `lessons.md` / `decisions-not-adr.md` | "Evaluated Temporal, chose custom orchestrator because X" |
| **Reasoning behind status quo** | `decisions-not-adr.md` | "Why POST /transcribe returns 202 not 200 — async, client must poll" |
| **Deprecation context** | `gotchas.md` | "/v1/legacy-endpoint kept for client Z until Q3 2026, don't remove" |

### What does NOT go into wiki

| ❌ Don't put in wiki | Where it lives instead |
|---|---|
| Every commit | git log |
| Every PR | gitlab/github |
| Current architecture (state) | `docs/ARCHITECTURE.md` |
| Current repo design (state) | `docs/design.md` |
| Formal binding decisions | `docs/adr/*.md` |
| Code itself | the code |
| API contracts (machine-readable) | `docs/openapi.yaml`, OpenAPI spec |
| Pure endpoint list | derivable from code, or `docs/design.md` |
| Routine commits ("renamed var", "fixed typo") | git log, nowhere else |
| Configuration values | `.env.example`, config files |
| Repo README | `README.md` |
| Roadmap, features | issue tracker, ROADMAP.md |
| Secrets, credentials | 1Password (NEVER in wiki) |
| Raw production logs | logging system; only sanitized summaries OK in wiki |

### Boundary cases — examples

**Architecture:**
- "Cortex uses Kafka for ingest, Qdrant for storage" → `docs/design.md`
- "We tried microservices first, rolled back to modular monolith because deployment complexity exceeded benefit at our scale" → `wiki/decisions-not-adr.md`

**API contracts:**
- OpenAPI spec, endpoint list → `docs/openapi.yaml` (auto, in repo)
- "POST /transcribe returns 202 because async; clients must poll /status. Synchronous version was rejected because median latency 4s blew browser timeouts" → `wiki/gotchas.md`

**Endpoints / routes:**
- Complete list (machine-derivable) → repo or auto-generated
- "These 3 endpoints are deprecated but kept for legacy client Z until Q3" → `wiki/gotchas.md`
- "Why /a/b is on backend but /a/c is on cortex (architectural inconsistency we noticed but haven't fixed)" → `wiki/open-questions.md`

**Design patterns:**
- Pattern description in code → `docs/design.md` ("Storage adapters use Result type instead of exceptions")
- "We learned this the hard way — initially used exceptions, debugging across async boundaries was nightmare" → `wiki/lessons.md`

### Why this separation matters (5 reasons)

1. **Code sync via PR review** — `docs/` updates ride along with code changes; reviewer rejects if drift. Wiki has no such gate.

2. **Onboarding without vault access** — `git clone` gives a new dev complete repo docs. External vault is private; can't onboard without sharing it.

3. **Open-source / publish readiness** — `docs/` is what you'd publish to customers. Wiki is your private memory with personal notes and incident context.

4. **Tooling integration** — CI linters, code generation, docs builders, service catalogs all index `docs/`. None index your external vault.

5. **Agent semantic clarity** — agent reads `docs/` knowing "this is current state". Reads wiki knowing "this is historical experience". Mixing breaks the mental model: "is this NATS reference current or 6 months old?"

### Why expanding wiki to include canonical docs would break the pattern

If you put architecture/API/endpoints into wiki:

- **Drift starts day one** — no PR gate on wiki
- **External onboarding blocks** — need to share vault to share architecture
- **Lose CI checks** — no automated drift detection between code and architecture
- **Lose codegen** — can't generate types from OpenAPI if it lives in vault
- **Agent confusion** — agent can't distinguish "this is now" from "this was reasoning a year ago"
- **Karpathy paradigm violated** — wiki was for "compilation of irretrievable knowledge"; architecture is retrievable from code, so it adds no value to recompile it

### Practical heuristic for capture decisions

When in doubt during work, ask:

> "If I deleted this thought now, could a new dev reading the repo discover it again from code + design.md + ADRs?"

- **YES** → don't capture (it's discoverable, no need to compile)
- **NO** → capture in wiki (this is the irretrievable knowledge Karpathy wants)

Examples:
- "Service X talks to service Y over Kafka" → discoverable from code → don't capture
- "We tried gRPC first, abandoned because schema evolution was too rigid for our use case" → not discoverable → capture in `wiki/lessons.md` or `decisions-not-adr.md`
