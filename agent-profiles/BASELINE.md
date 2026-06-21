# Agent Baseline

- Conversation: Russian. Code, identifiers, comments: English.
- Start with repo context: nearest `AGENTS.md`/`CLAUDE.md`, `README`, `Makefile`, manifests.
- Dev infra: есть общий слой **dev-stack** (Postgres/Redis/Qdrant/ClickHouse/MinIO/OTel/Langfuse/Ollama на `$DEV_STACK_HOST`, по умолчанию `gerc0g`). Нужна инфра для локальной разработки — подключи репо командой `dev-stack connect` (проверка `dev-stack doctor`), НЕ поднимай своё (`brew install`, отдельный docker-compose). Не каждому репо это нужно — решай по задаче. Канон: `services/dev-stack/README.md`, live-список `dev-stack urls`.
- Tailnet/сервер: домашний сервер и tailnet достижимы по **IP** (`gerc0g` = `100.73.117.50`). MagicDNS намеренно выключен (ради split-DNS Pritunl); tailnet-имена держит фоновый демон `tailnet-route-pin` (маршруты мимо full-tunnel Happ + `/etc/hosts`-пин имён). Если tailnet-имя НЕ резолвится — это не баг доступа, а DNS: `sudo sh ~/dotfiles/scripts/tailnet-route-pin.sh` (или используй tailnet-IP). Доступ к серверу: `ssh ubuntu-server`.
- Non-trivial change (>1 file, abstraction, refactor): give 2-3 options before editing.
- Trivial obvious fix: patch it and show the diff.
- Do not push, force-push, rebase pushed commits, edit `.env*`/`.envrc`, run migrations, or install globals without explicit ask. Exception: if `AGENT_GIT_MODE=commit-local`, commit each completed logical change with `agent-commit.sh` and explicit paths; finish completed tasks with `agent-finish.sh`.
- After edits, run the smallest relevant check and report what changed/tested.
- Durable lesson: use `lesson-append` if `docs/knowledge/_inbox.md` exists.
- No tool attribution: never add `Co-Authored-By: Claude/Codex` to commits or a `Generated with Claude Code`/`🤖` footer to PR/MR bodies. Commits and reviews are authored as the user.

References:
- Platform: `~/dotfiles/agent-profiles/PLATFORM.md`
- Git: `~/dotfiles/docs/platform/git-workflow.md`
- Secrets/env: `~/dotfiles/docs/platform/secrets-env.md`
- Local infra/dev-stack: `~/dotfiles/services/dev-stack/README.md` (canonical; live: `dev-stack urls`; connect a repo: `dev-stack connect`)
- ML/quant: `~/dotfiles/docs/platform/ml-quant-workflow.md`
- Commands: `launch`, `agent-workspace`, `agent-commit.sh`, `agent-finish.sh`, `agent-task-push.sh`, `oracle-tui`, `test-tui`, `dev-stack`, `agent-skill`
- Logs: Axiom CLI. DB: `psql`. Secrets: 1Password via `secret`.
- New skills: use `skill-maintainer`; source lives in `~/dotfiles/skills`, then run `agent-skill install && agent-skill doctor`.
