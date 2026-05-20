---
name: analyze-product
description: "Deep architectural analysis of a multi-repo product. Reads all repos in product dir, identifies services + roles + inter-service communication + data flow, writes draft to docs/ARCHITECTURE.md and proposed docs/adr/. Use when: user just onboarded new product, after major refactor, periodic architecture re-sync. Triggers: \"analyze product\", \"анализ архитектуры\", \"deep dive продукта\", \"проанализируй продукт\"."
---

# Analyze Product Architecture

Generates **product-level architecture documentation** by reading code across all
repos in the product directory.

## When to trigger

- User just ran `new-project` and wants architecture doc generated automatically
- User says "analyze this product" / "проанализируй продукт"
- After major refactor — re-sync ARCHITECTURE.md with reality
- Periodic (quarterly) review

## Prerequisites

cwd must be a **product directory** with subdirectories = repos. Detect via:
- `.product-config` exists in cwd
- OR cwd contains multiple subdirs each with `.git/`

If not detected — STOP, ask user to `cd ~/Desktop/Prokectfiles/<co>/<prod>`.

## Workflow

Read `analysis-plan.md` for detailed checklist of WHAT to analyze and HOW.

High-level flow:

1. **Scan repos in cwd.** List subdirs that look like repos (have `.git` or manifest).

2. **Per repo — quick fingerprint:**
   - Read `README.md` (first 50 lines)
   - Read manifest (`pyproject.toml` / `package.json` / `go.mod`)
   - Identify language + framework + ports + key deps
   - Find entry point
   - Find external service calls (httpx / requests / nats / kafka / grpc)

3. **Identify inter-service communication.** From step 2 calls, build a graph:
   `repo A → calls → repo B / external service`. Match `localhost:<port>` to
   sibling repos' declared ports.

4. **Identify shared dependencies / patterns.** Common across repos:
   - Same DB / message bus / cache
   - Shared schema repo
   - Common auth lib

5. **Identify deployment topology.** From `Dockerfile`, `.gitlab-ci.yml`,
   `.github/workflows/`, `k8s/`, `infrastructure/`.

6. **Draft `docs/ARCHITECTURE.md`** (in cwd). Structure:
   - Overview (1-2 paragraphs)
   - Services (один subsection per repo)
   - Inter-service communication (table + per-pair details)
   - Data flow (sequence diagrams for main paths)
   - Tech stack distribution (table)
   - Deployment topology
   - Architectural patterns observed
   - Known issues / non-obvious

7. **Identify candidate ADRs.** If finding non-trivial design decisions (e.g.
   "uses NATS for async work" / "state in single service" / "no monorepo") —
   suggest creating `docs/adr/0001-<slug>.md` files.

8. **Show draft to user. Don't auto-save.** Ask:
   - "Save docs/ARCHITECTURE.md as-is? (y/n/edit)"
   - "Create N ADR files for identified decisions? (y/n/list)"

9. **Apply with user approval.** Write files, suggest commit message.

## Rules

- READ-ONLY by default. Do NOT modify source code.
- Write only to `./docs/` directory in product cwd.
- Never `git commit` without explicit user ask.
- Be honest about limits: if external services can't be inferred from code,
  list them as "TODO confirm with user".
- Pair every "could be X or Y" with "ask user to confirm".
- Telegraph style in draft. No prose paragraphs without commands/facts.

## Output structure

```
~/Desktop/Prokectfiles/<co>/<prod>/
├── AGENTS.md                ← unchanged
├── docs/                    ← NEW (created by this skill)
│   ├── ARCHITECTURE.md      ← main output
│   └── adr/
│       ├── 0001-<topic>.md  ← optional, with user approval
│       └── ...
└── <repos>/                 ← unchanged
```

## After completion

1. `grep -nE 'docs/(ARCHITECTURE|adr)' ../<co>/<prod>/AGENTS.md` —
   confirm product AGENTS.md уже ссылается (через Context hierarchy table).
2. Suggested commit (если product dir в git):
   > `docs(${prod}): generate ARCHITECTURE.md + ADRs from code analysis`
3. NEVER auto-commit.

## When to re-run

- New repo added to product → re-analyze
- Major refactor → re-analyze
- Quarterly review → diff vs prev ARCHITECTURE.md
- ADRs are never deleted, only marked superseded
