# dotfiles

Personal Internal Developer Platform for local multi-agent development.

This repo is the source of truth for the local engineering environment:
terminal config, shell commands, Codex/Claude profiles, repo onboarding,
WikiPedik project memory, test/oracle TUIs, secrets conventions, and bootstrap
automation.

The design goal is simple: a new Mac plus this repo should be enough to rebuild
the working platform. Credentials, browser login state, 1Password approval, and
VCS trust decisions stay manual.

## Quick Start

```bash
git clone git@github.com:Gerc0g/dotfiles.git ~/dotfiles
~/dotfiles/bootstrap.sh
source ~/.zshrc
```

Then complete the manual steps printed by `bootstrap.sh`:

```bash
CODEX_HOME=~/.codex codex login
CODEX_HOME=~/.codex codex login
CODEX_HOME=~/.codex codex login
CLAUDE_CONFIG_DIR=~/.claude claude login
CLAUDE_CONFIG_DIR=~/.claude claude login
```

Enable 1Password CLI integration:

```bash
eval "$(op signin)"
op vault list
```

Open the WikiPedik vault in Obsidian:

```text
~/Desktop/WikiPedik/dev
```

## What Bootstrap Does

`bootstrap.sh` is idempotent enough for normal re-runs. It:

- installs Homebrew packages from `Brewfile`;
- installs Codex/Claude CLI if missing;
- symlinks Ghostty and tmux configs;
- creates agent profile directories:
  - `~/.codex`
  - `~/.codex`
  - `~/.codex`
  - `~/.claude`
  - `~/.claude`
- links the shared baseline profile;
- installs WikiPedik runtime skills and SessionStart hooks;
- installs repo-owned skills with `agent-skill`;
- configures Claude LSP plugins:
  - `pyright-lsp`
  - `vtsls`
  - `yaml-language-server`;
- prepares `~/Desktop/Prokectfiles` and `~/Desktop/WikiPedik`;
- adds the shell loader to `~/.zshrc`.

## Daily Entry Points

```bash
?                    # command index
? <cmd>              # command details
hq ls                # companies / products / repos
hq doctor            # machine state
agent-workspace start <co> <prod> <repo> <task>
wikipedik [zone]     # enter a WikiPedik vault zone
status               # live system/agent pressure dashboard
```

Common flows:

```bash
new-company <company> <vcs[:host]> <namespace> [email]
new-project <company> <product> [--ns=<namespace>] [repo...]
agent-workspace start <company> <product> <repo> <task-slug>
hq ls
hq doctor
wiki bootstrap <company> [product] [repo]
wiki sync --commit <company/product/repo>
agent-skill list
```

`docs/COMMANDS.md` is the detailed command registry.

## Workspace Model

Code lives under:

```text
~/Desktop/Prokectfiles/<company>/<product>/<repo>
```

WikiPedik memory lives under:

```text
~/Desktop/WikiPedik/dev/20-projects/<company>/<product>/repos/<repo>
```

Context hierarchy:

```text
BASELINE.md
  -> skills on demand
  -> company AGENTS.md
  -> product AGENTS.md
  -> repo AGENTS.md
  -> WikiPedik hot.md hook context
```

Generated/onboarded project context:

- company/product/repo `AGENTS.md`;
- repo `docs/design.md`;
- product `docs/ARCHITECTURE.md`;
- `docs/adr/*.md` where useful;
- repo `Makefile` as the executable contract;
- WikiPedik symlinks:
  - `docs/knowledge`
  - `docs/product-knowledge`
  - `docs/company-knowledge`.

## Agent Profiles

There is one profile per agent, and it lives at the tool's own default path.

| Profile | Path | Read by |
|---|---|---|
| Claude | `~/.claude` | `claude` CLI, the VS Code extension, any scheduled job |
| Codex | `~/.codex` | `codex` CLI, the VS Code extension, any scheduled job |

Both link `agent-profiles/BASELINE.md` — as `CLAUDE.md` and `AGENTS.md`
respectively — so an instruction written once reaches both agents. Every
repo-owned skill is installed into both.

Switch manually:

```bash
agent fresh
agent setup
agent wiki
agent legacy
agent status
```

There is one profile per agent and it lives at the tool's own default path, so
nothing routes and no environment variable has to be set: the CLI, the VS Code
extension and any scheduled job all read the same managed profile.

## Skills Architecture

Repo-owned skills live in:

```text
~/dotfiles/skills/<skill-name>/SKILL.md
```

Runtime profile directories contain symlinks, not source files.

Current repo-owned skills:

- `analyze-repo` — generate repo-level design docs from source code;
- `analyze-product` — generate multi-repo product architecture docs;
- `fill-agents-md` — condense design docs into short `AGENTS.md`;
- `onboard-agents-md` — interactively fill TODOs in generated `AGENTS.md`;
- `skill-maintainer` — universal skill for creating/updating skills in this
  dotfiles architecture.

Manage skills:

```bash
agent-skill list
agent-skill new <name>
agent-skill install
agent-skill doctor
```

Default isolation:

- new repo-owned skills are setup-only;
- `skill-maintainer` is universal and installed into daily/setup/wiki;
- `agent-skill doctor` verifies frontmatter, symlinks, and isolation.

Do not write reusable skills directly into `~/.codex-*` or `~/.claude-*`.

## WikiPedik Memory

WikiPedik is durable project memory, not a replacement for canonical docs.

Canonical truth stays in:

- `AGENTS.md`;
- `docs/design.md`;
- `docs/ARCHITECTURE.md`;
- `docs/adr/*.md`;
- code and tests.

WikiPedik stores durable lessons:

- root causes;
- failed approaches;
- debugging stories;
- gotchas;
- informal decisions;
- cross-repo patterns.

Repo symlinks:

```text
docs/knowledge         -> repo memory
docs/product-knowledge -> product memory
docs/company-knowledge -> company memory
```

Daily agents:

- automatically receive `docs/knowledge/hot.md` through SessionStart hooks;
- append candidate lessons only to `docs/knowledge/_inbox.md`;
- do not edit curated wiki pages directly.

Curator flow:

```bash
wiki bootstrap <company> [product] [repo]
wiki status <company/product/repo>
wiki sync <company/product/repo>
wiki sync --commit <company/product/repo>
wiki sync --push <company/product/repo>
wiki synthesize <company/product>
```

`wiki sync` drains `_inbox.md` into curated pages and refreshes `hot.md`.
`hot.md` is what future agents receive at session start.

Wiki git sync is explicit and scope-atomic:

- `wiki sync` updates local WikiPedik files only.
- `wiki sync --commit <scope>` commits only the requested memory scope, its product log, and the root project index if bootstrap touched it.
- `wiki sync --push <scope>` does the same commit and then pushes.
- Wiki commits use `git_email` from `~/Desktop/Prokectfiles/<company>/.company-config` when present, so company memory is authored by the matching account.
- WikiPedik commits go directly to its `main` branch; the app repo delivery model (`dev` as stage, `main` as prod, feature branches) does not apply to wiki curation.
- Commit messages keep the platform convention: English Conventional Commit type/scope, Russian description.
- Daily repo agents never push automatically; they only append candidates to `docs/knowledge/_inbox.md`.

## Onboarding Flow

Create a company:

```bash
new-company <company> <vcs[:host]> <namespace> [email]
```

Create a product and repos:

```bash
new-project <company> <product> [--ns=<namespace>] [repo...]
```

Fill in the generated `AGENTS.md` placeholders by asking an agent — the
`onboard-agents-md` skill is installed in both profiles and drives the
questions. Deeper architecture notes come from `analyze-repo` and
`analyze-product`, also skills.

Bootstrap WikiPedik links:

```bash
wiki bootstrap <company> [product] [repo]
```

Start work in an isolated worktree:

```bash
agent-workspace start <company> <product> <repo> <task-slug>
```

## Secrets

Secrets go through 1Password.

```bash
secret signin
secret add --repo <VAR> <VALUE>
secret add --product <VAR> <VALUE>
secret add --company <VAR> <VALUE>
```

Rules:

- no plaintext secrets in git;
- `.envrc` references scoped 1Password items through `secret-cache`;
- company-level vaults are created by `new-company` when possible;
- repo/product-specific item names are generated as
  `<product>__<repo>__<VAR>` or `<product>__<VAR>`.

See `docs/platform/secrets-env.md`.

## Local Infrastructure

Shared dev services are managed with:

```bash
dev-stack up
dev-stack down
dev-stack status
```

The stack runs locally on this machine. Multi-machine targeting comes back with the server layer (`hq server`).
Stack (18 services): Postgres, Redis, MinIO, Qdrant, ClickHouse, Prometheus,
Grafana, Loki, Tempo, OTel Collector, Langfuse, Ollama, Open-WebUI, Metabase,
CloudBeaver, Redis-Commander, Traefik, Homepage. Live list: `dev-stack urls`.
(No MLflow — only a MinIO `mlflow` bucket; use Langfuse for LLM tracing.)

Connect a repo (self-onboard): `cd <repo> && dev-stack connect` → `.envrc` endpoints
+ DB `<product>__<repo>`; check with `dev-stack doctor`.

Use it only when a repo needs shared local services. Repos should still expose
their own `Makefile` targets.

Canonical: `services/dev-stack/README.md`.

## Claude LSP Baseline

Claude daily/setup profiles include:

- Python: `pyright-lsp@claude-plugins-official`;
- TypeScript/JavaScript: `vtsls@claude-code-lsps`;
- YAML: `yaml-language-server@claude-code-lsps`.

This is the intentionally small plugin set. Large plugin packs, browser
automation, generic MCP bundles, and external-service plugins are not enabled
globally by default.

## Repository Layout

- `agent-profiles/BASELINE.md` — short always-on agent rules;
- `agent-profiles/PLATFORM.md` — detailed platform reference;
- `docs/COMMANDS.md` — command registry;
- `docs/platform/` — detailed platform docs linked from baseline;
- `docs/wiki-integration-design.md` — WikiPedik design notes;
- `shell/` — zsh commands loaded from `_loader.zsh`;
- `scripts/` — bootstrap and command helpers;
- `templates/` — company/product/repo `AGENTS.md` templates;
- `skills/` — repo-owned setup/universal skills;
- `skills-stash/wiki/` — WikiPedik worker/curator skills, hooks, runtime scripts;
- `services/` — local dev stack;
- `tmux/`, `ghostty/` — terminal environment config;
- `Brewfile` — Homebrew package set.

## Validation

Run the complete local, non-destructive check before committing:

```bash
make verify
```

Focused targets are available through `make lint` and `make test`.

Prompt routing smoke checks:

```bash
CODEX_HOME=~/.codex codex debug prompt-input smoke
CODEX_HOME=~/.codex codex debug prompt-input smoke
CODEX_HOME=~/.codex codex debug prompt-input smoke
```

Claude plugin checks:

```bash
CLAUDE_CONFIG_DIR=~/.claude claude plugin list
CLAUDE_CONFIG_DIR=~/.claude claude plugin list
```

## Operating Principles

- Keep always-on agent context short.
- Put detailed platform behavior in docs or skills.
- Use skills for repeatable workflows, not as dumping grounds.
- Use `Makefile` as the repo execution contract.
- Use WikiPedik for lessons learned, not current source of truth.
- Keep secrets in 1Password.
- Avoid global MCP/plugin sprawl.
- Prefer explicit local checks over agent guesswork.
