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
CODEX_HOME=~/.codex-new codex login
CODEX_HOME=~/.codex-setup codex login
CODEX_HOME=~/.codex-wiki codex login
CLAUDE_CONFIG_DIR=~/.claude-new claude login
CLAUDE_CONFIG_DIR=~/.claude-setup claude login
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
  - `~/.codex-new`
  - `~/.codex-setup`
  - `~/.codex-wiki`
  - `~/.claude-new`
  - `~/.claude-setup`
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
launch               # interactive safe worktree launcher
wiki                 # open WikiPedik product repo
agent status         # current Codex/Claude profile env
```

Common flows:

```bash
new-company <company> <vcs[:host]> <namespace> [email]
new-project <company> <product> [--ns=<namespace>] [repo...]
setup-context <company> [product] [repo]
complete-onboard <company> [product] [repo]
launch <company> <product> <repo> <task>
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

The platform keeps profiles separate to avoid prompt bloat.

| Profile | Env | Purpose | Skills |
|---|---|---|---|
| `fresh` | `~/.codex-new`, `~/.claude-new` | daily repo work through `launch` | WikiPedik worker skills, `oracle`, universal `skill-maintainer` |
| `setup` | `~/.codex-setup`, `~/.claude-setup` | onboarding, analysis, AGENTS generation | `analyze-*`, `fill-agents-md`, `onboard-agents-md`, `skill-maintainer` |
| `wiki` | `~/.codex-wiki`, Claude fresh | curator work in WikiPedik | curator skills, `skill-maintainer` |
| `legacy` | `~/.codex`, `~/.claude` | old profiles only when explicitly needed | unmanaged |

Switch manually:

```bash
agent fresh
agent setup
agent wiki
agent legacy
agent status
```

Routing is automatic for main flows:

- `launch` uses `fresh`;
- `setup-context`, `analyze-*`, `fill-agents-md`, `onboard` use `setup`;
- `wiki sync/status/synthesize` use `wiki`.

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

## Launch Layout

`launch <company> <product> <repo> <task>` creates a managed agent worktree and opens one tmux window with four panes:

| Pane | Role | Tool |
|---|---|---|
| `plan` | architecture, planning, decomposition | Codex |
| `code` | implementation | Claude Code |
| `test` | selectable local checks with logs | `tools/test-tui` |
| `oracle` | second-model review / saved answers | `tools/oracle-tui` + `@steipete/oracle` |

Git flow:

```text
worktree path: ~/Desktop/Prokectfiles/.worktrees/<company>/<product>/<repo>/<id>
branch: agent/<task>-<id>
commit: agent-commit.sh "type(scope): русское описание" -- <explicit paths>
finish: agent-finish.sh  # verify/test → push → draft PR/MR → review metadata
cleanup: agent-workspace cleanup
```

tmux tab title:

```text
🖥 <repo>/<id>
```

Pane border labels are intentionally short:

```text
plan
code
test
oracle
```

## Test TUI

`test-tui` discovers local commands and stores run logs under:

```text
.agents/test-runs/
```

It prefers `Makefile` targets and lets the user select checks instead of
remembering every test command.

Expected repo `Makefile` targets:

```text
install
dev
test
test-one
lint
format
typecheck
verify
```

`verify` must stay local and non-destructive:

- no deploys;
- no migrations;
- no broad cleanup;
- no external state mutation.

## Oracle TUI

`oracle-tui` is the local second-model review interface.

It stores generated requests under:

```text
.agents/oracle/requests/
```

And saved answers under:

```text
.agents/oracle/
```

Flow:

1. User types one prompt line.
2. The TUI composes a compact repo-aware request.
3. Oracle runs in the background.
4. The final answer is saved as Markdown.
5. The UI shows the new file and preview.

The saved answer format is intentionally small:

- Time
- Repo
- Prompt
- Answer

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

Generate context:

```bash
setup-context <company> [product] [repo]
```

Complete human-only TODOs:

```bash
complete-onboard <company> [product] [repo]
```

Bootstrap WikiPedik links:

```bash
wiki bootstrap <company> [product] [repo]
```

Open the repo:

```bash
launch <company> <product> <repo> <task>
```

## Secrets

Secrets go through 1Password.

```bash
secret signin
secret add <VAR> <VALUE>
secret get op://<vault>/<VAR>/credential
```

Rules:

- no plaintext secrets in git;
- `.envrc` references 1Password items;
- company-level vaults are created by `new-company` when possible;
- repo/product-specific naming should make duplicated variable names clear.

See `docs/platform/secrets-env.md`.

## Local Infrastructure

Shared dev services are managed with:

```bash
dev-stack up
dev-stack down
dev-stack status
```

Default stack:

- Postgres
- Redis
- Prometheus
- Grafana
- MinIO
- MLflow

Use it only when a repo needs shared local services. Repos should still expose
their own `Makefile` targets.

See `docs/platform/local-infra.md`.

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
- `tools/oracle-tui/` — Oracle prompt/history TUI;
- `tools/test-tui/` — local test picker/log viewer TUI;
- `services/` — local dev stack;
- `tmux/`, `ghostty/` — terminal environment config;
- `Brewfile` — Homebrew package set.

## Validation

Useful local checks before committing:

```bash
bash -n bootstrap.sh scripts/agent-skill.sh
zsh -n shell/*.zsh
agent-skill doctor
git diff --check
```

Prompt routing smoke checks:

```bash
CODEX_HOME=~/.codex-new codex debug prompt-input smoke
CODEX_HOME=~/.codex-setup codex debug prompt-input smoke
CODEX_HOME=~/.codex-wiki codex debug prompt-input smoke
```

Claude plugin checks:

```bash
CLAUDE_CONFIG_DIR=~/.claude-new claude plugin list
CLAUDE_CONFIG_DIR=~/.claude-setup claude plugin list
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
