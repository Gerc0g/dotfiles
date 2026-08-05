# Oracle prompt — integrate Karpathy LLM Wiki as agent memory for development

## Briefing

I'm Egor — software engineer + data scientist (ML, NLP) + quant analyst. I'm building a Personal Internal Developer Platform on my Mac for solo / lead-of-small-team production work.

My current stack:
- Codex CLI (`gpt-5.5`, `high` reasoning) as primary agent
- Claude Code (Sonnet 4.6) as implementer
- `@steipete/oracle` CLI (GPT-5.4 Pro, browser mode with ChatGPT Pro) for deep review
- tmux 4-window layout per repo: `plan` (codex) / `code` (claude) / `test` / `oracle`
- Obsidian + WikiPedik vault for knowledge management

I have two companies and N personal projects:
- **neurodesk** — production work. 5 products (`agents`, `legacy`, `saas`, `infra`, `wiki`) × 16 git repos. Already fully onboarded with multi-layer AGENTS.md hierarchy: Profile → PLATFORM → Company → Product → Repo. Every repo has `docs/design.md`, every product has `docs/ARCHITECTURE.md`, and 95 ADRs are written.
- **justchimera** — secondary, sketched but not yet onboarded.
- **Personal** — research, learning, business / brand work.

My WikiPedik vault is already running the Karpathy LLM Wiki pattern:
- `~/Desktop/WikiPedik/dev/` — personal projects, business, brand work. Has `00-inbox`, `10-wiki/{index,log}`, `20-projects` (currently empty — this is where I want to integrate project knowledge), plus `AGENTS.md` and `llm-wiki.md` (Karpathy spec copy).
- `~/Desktop/WikiPedik/research/` — ML / quant / SWE / math / cross-domain research. Has `00-inbox`, `01-raw/{business,cross-domain,math,ml,quant,swe}`, `10-wiki`, `15-study`, `20-outputs`. First ingest already done — vaswani-2017 attention paper + concept pages (attention, multi-head-attention, positional-encoding, transformer-architecture, etc.).

I have `wikipedik` tmux command that opens both vaults in two codex sessions side-by-side (separate `~/.codex` profile).

## Goal

Decide how to make the Karpathy wiki the **persistent memory layer for my development work** — so that lessons, debugging stories, design decisions, gotchas, "this didn't work because X", and recurring patterns accumulate in the wiki **across sessions and projects**, and are available to the agent at the start of any new session in any repo.

Specifically:
- When I work in `~/Desktop/Prokectfiles/neurodesk/agents/cortex` with codex/claude — they should reach into my wiki when relevant (e.g. "we already solved similar Kafka offset issue in another repo, look here").
- When I finish a task, lessons should be filed back into wiki — automatically or with one command.
- The wiki should **synthesize across repos and across companies** — agent shouldn't have to re-discover the same patterns.
- Cross-company privacy: nothing from `neurodesk` should leak into `justchimera` or vice versa. But cross-repo within same company is fair game.

## Where things live

- `~/Desktop/Prokectfiles/<co>/<prod>/<repo>/` — git repos with `AGENTS.md` + `docs/{design.md, ARCHITECTURE.md, adr/*}`
- `~/Desktop/WikiPedik/dev/20-projects/` — currently empty, ready for project-memory integration
- `~/Desktop/WikiPedik/research/` — research knowledge (papers, ML concepts) — different audience, don't merge
- `~/dotfiles/` — shell tools (`launch`, `setup-context`, `batch-fill-agents`, `agents-sessions`, etc.) + skills (`analyze-repo`, `analyze-product`, `fill-agents-md`, `onboard-agents-md`) + templates (`AGENTS.md.{company,product,repo}.tmpl`)
- `~/.codex/` and `~/.claude/` — profile (autoloaded by codex/claude)
- `~/.codex/AGENTS.md` is a symlink to `~/.codex/AGENTS.md` so both default and named profile see the same Profile AGENTS.md

## What I've built so far (decisions already made — DON'T propose alternatives)

- **AGENTS.md hierarchy**: 5 layers (Profile → PLATFORM → Company → Product → Repo). 22 AGENTS.md files in neurodesk already populated (via `batch-fill-agents` codex skill that reads design.md/ARCHITECTURE.md and condenses to ≤120-line AGENTS.md).
- **Skills**: `analyze-repo` writes design.md, `analyze-product` writes ARCHITECTURE.md + ADRs, `fill-agents-md` condenses docs into AGENTS.md, `onboard-agents-md` is Q&A wizard for human-only decisions.
- **launch command**: tmux 4-window layout per repo. `plan` (codex high reasoning) / `code` (claude code) / `test` (auto-detected watcher in Suggest mode) / `oracle` (oracle CLI in headless browser mode for deep review).
- **Atomicity = repo** (one repo at a time, atomic commits). **Coordination = product** (cross-repo changes = series of MRs).
- **dev-stack** — local Docker: Postgres :5432, Redis :6379, Prometheus :9090, Grafana :3030, MinIO :9000, MLflow :5000.
- **1Password** for secrets (no plain-text keys).

## What I've tried for wiki integration so far

- Empty `dev/20-projects/` exists, ready
- I see two possible patterns from research:
  1. **Symlinks** from each repo: `<repo>/docs/knowledge -> ~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/<repo>/`. Agent reads via filesystem.
  2. **MCP server** like `dfalci/mcp-advwiki` (Rust): agent calls wiki via tool/API. No filesystem coupling.
- Karpathy gist says: "the LLM writes and maintains the wiki, you do sourcing and asking questions"
- Steinberger's pattern is per-project AGENTS.md grown organically through "записи уроков" — but that doesn't accumulate across projects

## Constraints

- **Respect existing setup**: 5-layer AGENTS.md hierarchy stays. PLATFORM.md, Profile, Company/Product/Repo AGENTS.md — all in place and filled.
- **No vector DB / RAG infrastructure**: Karpathy paradigm is "compilation, not retrieval". Index file + grep + LLM scanning is enough at this scale.
- **No monorepo migration**: products and repos stay polyrepo. ADR-002 (or equivalent) commits to this.
- **No abandoning launch + 4-window pattern**: that's locked in.
- **Cross-company privacy**: knowledge between `neurodesk` and `justchimera` is firewalled. Within a company is shared.
- **Two-direction sync**: changes the agent makes in vault should be visible in repo (and vice versa). I edit in both places.
- **Plain markdown only**: no proprietary formats, no Obsidian-only Bases for primary data (the BASE file with the table view is acceptable as an Obsidian view, not as source-of-truth).
- **Don't propose proprietary tools** like Mem.ai, Notion, Reflect. Karpathy paradigm + Obsidian only.
- **Codex CLI + Claude Code + Oracle** are the agents — don't propose switching.
- **Personal vs project separation**: `dev/40-personal/` (if created) is off-limits to project agents.
- **`research/` stays separate** — different audience (papers, ML concepts), not project-memory.

## The exact questions I want answered

1. **Folder topology**: how should `~/Desktop/WikiPedik/dev/20-projects/` be organized to mirror `~/Desktop/Prokectfiles/`? By company → product → repo? Or flat by repo? Or by topic?

2. **Symlink topology**: for each repo at `~/Desktop/Prokectfiles/<co>/<prod>/<repo>/`, what symlinks should I create? Just `docs/knowledge -> vault/20-projects/<co>/<prod>/<repo>/`? Or also per-product knowledge (`<repo>/docs/product-knowledge -> vault/20-projects/<co>/<prod>/`)? Or per-company (cross-product)?

3. **What lives in vault vs in repo's `docs/`**: if `docs/design.md` is the deep architecture file (already exists in repo), and `docs/ARCHITECTURE.md` is the product-level architecture (in product dir, already exists) — what new content lives in `vault/20-projects/<co>/<prod>/<repo>/`? Lessons? Debugging stories? Open questions? "Why this didn't work" notes? Decisions that didn't become ADRs?

4. **AGENTS.md schema updates**: how should I update `dev/AGENTS.md` (vault root schema) and the repo-level `AGENTS.md`s to:
   - Tell the agent "when you learn a lesson in this repo, write it to docs/knowledge/lessons.md"
   - Tell the agent "when asked about a recurring pattern, search vault first"
   - Tell the agent "stay in your project subfolder; don't reach into other companies"

5. **Workflow / triggers**: when exactly should the agent write to vault? After every commit? Only when I say "запиши урок"? Periodically (lint pass)? What's the **default behavior** that won't annoy me but also won't lose context?

6. **Cross-repo synthesis**: how does the agent find that "we solved similar issue in repo A" while working in repo B? Index file at `dev/20-projects/<co>/index.md` listing every repo's lessons? Or grep across all `lessons.md` files? Or something smarter?

7. **Lint / health checks**: Karpathy spec says wiki should be linted periodically (contradictions, stale claims, orphans). Should I run lint per company, per product, or globally? When? Trigger?

8. **First steps** — what's the **minimal pilot setup** to test the integration on one product (say `neurodesk/agents`, 6 repos, 1 product-level)? What's the order of operations? What can I check at each step?

9. **Risks / failure modes**: what's the **most likely way this integration becomes a maintenance burden** rather than a multiplier? How do I detect it early?

10. **Optional: `wikipedik` tmux command rewrite**: my current `wikipedik` opens both `dev/` and `research/` in tmux. After integration with projects, should the workflow change? Should I add a per-repo "wiki context" tmux window in `launch`?

## Desired output

A concrete integration design with:

1. **Architecture diagram** (ASCII) showing the relationship between `~/Desktop/Prokectfiles/`, `~/Desktop/WikiPedik/dev/20-projects/`, and `~/dotfiles/`.

2. **Exact folder structure** for `~/Desktop/WikiPedik/dev/20-projects/` with `neurodesk` populated and `justchimera` as placeholder.

3. **Exact symlink commands** to wire it up (for one example product `neurodesk/agents`).

4. **Schema updates** for:
   - `~/Desktop/WikiPedik/dev/AGENTS.md` — wiki root schema with new section on projects
   - Repo-level `AGENTS.md.repo.tmpl` (template I use for new repos) — what to add about wiki integration
   - PLATFORM.md — what universal rule (if any) to add

5. **5–10 concrete workflows** with example prompts:
   - "запиши урок про этот N+1 запрос"
   - "перед тем как мне рекомендовать, проверь не сталкивались ли мы с этим"
   - "продумай задачу учитывая что мы знаем"
   - "сделай sync вики после моего commit"
   - "выведи open questions по этому продукту"
   - и так далее

6. **Triggers / automation** — when each workflow should run automatically vs explicitly invoked.

7. **Minimal pilot setup** — step-by-step commands for `neurodesk/agents` only.

8. **3 alternative designs** with tradeoffs:
   - Pure symlink (filesystem-based)
   - MCP server (API-based, e.g. `dfalci/mcp-advwiki`)
   - Hybrid (symlink for primary, MCP for cross-cutting queries)

9. **Risks ranked by blast radius** — what can go wrong, how to detect, how to recover.

10. **Tests / sanity checks** — how I verify the integration is working after each setup step.

Aim for actionable specificity over abstract recommendations. Cite Karpathy's gist text where directly relevant. If multiple options exist, pick one and justify; mention alternatives in passing.

## Attached files (with --file flags)

Recommended attachments for full context:

```
--file ~/dotfiles/agent-profiles/PLATFORM.md \
--file ~/Desktop/Prokectfiles/neurodesk/AGENTS.md \
--file ~/Desktop/Prokectfiles/neurodesk/agents/AGENTS.md \
--file ~/Desktop/Prokectfiles/neurodesk/agents/cortex/AGENTS.md \
--file ~/Desktop/Prokectfiles/neurodesk/agents/cortex/docs/design.md \
--file ~/Desktop/Prokectfiles/neurodesk/agents/docs/ARCHITECTURE.md \
--file ~/Desktop/WikiPedik/dev/AGENTS.md \
--file ~/Desktop/WikiPedik/dev/llm-wiki.md \
--file ~/Desktop/WikiPedik/research/AGENTS.md \
--file ~/dotfiles/templates/AGENTS.md.repo.tmpl \
--file ~/dotfiles/templates/AGENTS.md.product.tmpl \
--file ~/dotfiles/templates/AGENTS.md.company.tmpl \
--file ~/dotfiles/skills/onboard-agents-md/SKILL.md \
--file ~/dotfiles/skills/analyze-repo/SKILL.md \
--file ~/dotfiles/skills/fill-agents-md/SKILL.md \
--file ~/dotfiles/shell/31-launch.zsh \
--slug "wiki-integration-design"
```
