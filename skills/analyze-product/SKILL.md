---
name: analyze-product
description: "Deep architectural analysis of a registered HQ product when the user asks for analyze product, анализ архитектуры, deep dive продукта, or проанализируй продукт. Read registered repos, derive roles and data flow, and draft ARCHITECTURE.md within authorized write scope. Separate from basic onboarding; ADR creation is optional."
---

# Analyze Product Architecture

Generates **product-level architecture documentation** by reading code across all
repos in the product directory.

## When to trigger

- User explicitly wants architecture analysis after creating a product
- User says "analyze this product" / "проанализируй продукт"
- After major refactor — re-sync ARCHITECTURE.md with reality
- Periodic (quarterly) review

## Resolve the analysis scope

Use the explicit HQ task scope when supplied; otherwise use `hq ctx --plain`
to identify the requested product. Read `hq entity show <company/product>
--json` and use the card's canonical `path` and registered repo `children`.
Check that any task cwd belongs to that canonical product on the selected
machine. A company/product directory does not need to be a Git repository.

Do not infer a product from a directory shape or collect every child with a
manifest. `.git` may be a file or directory. Resolve child paths through their
HQ cards; do not substitute a worktree or another similarly named repo.
If the target is unavailable, report the concrete gap without provisioning it.

This is an optional deep-analysis workflow, not a prerequisite for filling an
entity card. Empty products and repositories have no architecture to derive:
report what exists and what is still a plan. Use `onboard-agents-md` for guided
context filling without source or existing architecture docs.

## Workflow

Read `analysis-plan.md` for detailed checklist of WHAT to analyze and HOW.

High-level flow:

1. **Inventory registered repo children.** Record each inspected path and
   branch/HEAD when available. Preserve dirty files and note unavailable repos;
   do not change branches or hide gaps in product coverage.

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

6. **Draft `docs/ARCHITECTURE.md`** in the canonical product path. Read existing
   documentation first and preserve relevant human-authored content. Structure:
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
   list candidate topics. Creating files is a separate optional scope; do not
   turn observed implementation into an accepted historical decision.

8. **Deliver within the requested write scope.** If saving
   `docs/ARCHITECTURE.md` was explicitly authorized, save the bounded update and
   report the changes without another approval round. For read-only analysis,
   show the concrete draft; ask only before a newly requested write. Create
   ADRs only if authorized, using unused numbers and Draft status.

9. **Keep onboarding separate.** Architecture analysis does not confirm entity
   items. If a bounded AGENTS.md update was also requested, use the current HQ
   document revision and the save protocol in
   [onboard-agents-md](../onboard-agents-md/SKILL.md). Do not directly replace the
   file or mark source-derived content human-confirmed.

## Rules

- READ-ONLY by default. Do NOT modify source code.
- Write only authorized docs in the canonical product path and any separately
  authorized AGENTS.md edit through HQ. Re-read before saving when another
  writer may have changed the document; preserve unrelated edits.
- Never `git commit` without explicit user ask.
- Be honest about limits: if external services can't be inferred from code,
  list them as "TODO confirm with user".
- Label uncertain conclusions; ask focused questions when they materially
  change the result. Do not require confirmation of every observed fact.
- Telegraph style in draft. No prose paragraphs without commands/facts.

## Output structure

```
<canonical-product-path>/
├── AGENTS.md                ← unchanged unless a bounded HQ edit was requested
├── docs/                    ← existing or created within authorized scope
│   ├── ARCHITECTURE.md      ← main output
│   └── adr/
│       ├── <next-number>-<topic>.md  ← only if separately authorized
│       └── ...
└── <repos>/                 ← unchanged
```

## After completion

1. Read the canonical product AGENTS.md to check its architecture references;
   report missing references without changing context outside the write scope.
2. Suggested commit (если product dir в git):
   > `docs(${prod}): generate ARCHITECTURE.md + ADRs from code analysis`
3. NEVER auto-commit.

## When to re-run

- New repo added to product → re-analyze
- Major refactor → re-analyze
- Quarterly review → diff vs prev ARCHITECTURE.md
- ADRs are never deleted, only marked superseded
