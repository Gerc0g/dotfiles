# dotfiles

Personal Internal Developer Platform for local multi-agent development.

## Quick Start

```bash
git clone git@github.com:Gerc0g/dotfiles.git ~/dotfiles
~/dotfiles/bootstrap.sh
source ~/.zshrc
```

`bootstrap.sh` installs Homebrew packages, links shell/tmux/Ghostty config,
creates Codex/Claude profile directories, installs WikiPedik runtime assets,
and prepares `~/Desktop/Prokectfiles` plus `~/Desktop/WikiPedik`.

Manual steps remain manual: agent/browser login, 1Password approval, and adding
generated SSH keys to the relevant VCS host.

## Daily Commands

```bash
?              # command index
? <cmd>        # command details
launch         # pick company -> product -> repo
wiki           # open the WikiPedik product repo
```

Common flows:

```bash
new-company <company> <vcs[:host]> <namespace> [email]
new-project <company> <product> [--ns=<namespace>] [repo...]
setup-context <company> [product] [repo]
complete-onboard <company> [product] [repo]
launch <company> <product> <repo>
wiki bootstrap <company> [product] [repo]
wiki sync --commit <company/product/repo>
```

`docs/COMMANDS.md` is the source of truth for command help.

## Workspace Model

Code lives under:

```text
~/Desktop/Prokectfiles/<company>/<product>/<repo>
```

Context is layered:

```text
Profile -> PLATFORM.md -> company AGENTS.md -> product AGENTS.md -> repo AGENTS.md
```

Generated/onboarded project context:

- company/product/repo `AGENTS.md`
- repo `docs/design.md`
- product `docs/ARCHITECTURE.md`
- `docs/adr/*.md` where useful
- stable repo `Makefile` targets for agents and test UI

## Launch Layout

`launch <company> <product> <repo>` opens one tmux window with four panes:

| Pane | Role | Tool |
|---|---|---|
| `plan` | architecture, planning, decomposition | Codex |
| `code` | implementation | Claude Code |
| `test` | selectable local checks with logs | `tools/test-tui` |
| `oracle` | second-model review / prompts / saved answers | `tools/oracle-tui` + `@steipete/oracle` |

tmux tabs are named as `🖥 <repo>`. Pane border labels are intentionally short:
`plan`, `code`, `test`, `oracle`.

## Testing Contract

Each runnable repo should expose a `Makefile` as the canonical executable
interface for humans, agents, and `test-tui`.

Preferred targets:

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

`verify` must stay local and non-destructive: no deploys, migrations, broad
cleanup, or external state mutation.

## WikiPedik Memory

WikiPedik is the persistent project-memory layer.

Repo symlinks:

```text
docs/knowledge         -> repo memory
docs/product-knowledge -> product memory
docs/company-knowledge -> company memory
```

Worker agents in normal project repos:

- automatically receive `docs/knowledge/hot.md` through `SessionStart`;
- append durable lessons only to `docs/knowledge/_inbox.md`;
- do not edit curated wiki pages directly.

Curator flow:

```bash
wiki sync <company/product/repo>          # drain inbox interactively
wiki sync --commit <company/product/repo> # drain + commit WikiPedik vault
wiki status <company/product/repo>        # pending/drained/health snapshot
wiki synthesize <company/product>         # promote repeated cross-repo patterns
```

Canonical docs still win. `docs/design.md`, `docs/ARCHITECTURE.md`, and ADRs
describe current truth. WikiPedik stores why/how we learned things: gotchas,
failed approaches, debugging stories, informal decisions, and reusable lessons.

## Secrets

Secrets go through 1Password:

```bash
secret signin
secret add <VAR> <VALUE>
secret get op://<vault>/<VAR>/credential
```

`new-company` creates the company vault when possible. `.envrc` is for
references to 1Password items, not plaintext secrets.

## Repository Layout

- `shell/` — zsh commands loaded from `_loader.zsh`
- `scripts/` — bootstrap and command helpers
- `templates/` — company/product/repo `AGENTS.md` templates
- `skills/` — project-analysis/onboarding skills
- `skills-stash/wiki/` — WikiPedik worker/curator skills, hooks, runtime scripts
- `tools/oracle-tui/` — Oracle prompt/history TUI
- `tools/test-tui/` — local test picker/log viewer TUI
- `agent-profiles/PLATFORM.md` — universal agent operating rules
- `docs/COMMANDS.md` — command registry
- `docs/wiki-integration-design.md` — WikiPedik architecture notes
- `services/` — local dev stack
- `tmux/`, `ghostty/` — terminal environment config
- `Brewfile` — Homebrew package set

## Principle

One repo should be enough to rebuild the local developer platform. The only
things that stay outside automation are credentials, browser login state, and
explicit human trust decisions.
