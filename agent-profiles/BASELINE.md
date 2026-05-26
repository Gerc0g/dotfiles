# Agent Baseline

- Conversation: Russian. Code, identifiers, comments: English.
- Start with repo context: nearest `AGENTS.md`/`CLAUDE.md`, `README`, `Makefile`, manifests.
- Non-trivial change (>1 file, abstraction, refactor): give 2-3 options before editing.
- Trivial obvious fix: patch it and show the diff.
- Do not push, force-push, rebase pushed commits, edit `.env*`/`.envrc`, run migrations, or install globals without explicit ask. Exception: if `AGENT_GIT_MODE=commit-local`, commit each completed logical change with `agent-commit.sh` and explicit paths; push only at task completion with `agent-task-push.sh`.
- After edits, run the smallest relevant check and report what changed/tested.
- Durable lesson: use `lesson-append` if `docs/knowledge/_inbox.md` exists.

References:
- Platform: `~/dotfiles/agent-profiles/PLATFORM.md`
- Git: `~/dotfiles/docs/platform/git-workflow.md`
- Secrets/env: `~/dotfiles/docs/platform/secrets-env.md`
- Local infra/dev-stack: `~/dotfiles/docs/platform/local-infra.md`
- ML/quant: `~/dotfiles/docs/platform/ml-quant-workflow.md`
- Commands: `launch`, `agent-workspace`, `agent-commit.sh`, `agent-task-push.sh`, `oracle-tui`, `test-tui`, `dev-stack`, `agent-skill`
- Logs: Axiom CLI. DB: `psql`. Secrets: 1Password via `secret`.
- New skills: use `skill-maintainer`; source lives in `~/dotfiles/skills`, then run `agent-skill install && agent-skill doctor`.
