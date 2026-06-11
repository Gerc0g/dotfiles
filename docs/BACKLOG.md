# Platform Backlog

## WikiPedik: использовать полную мощь подхода

Status: todo

Цикл памяти работает на страховочном минимуме; неиспользованные возможности и
рекомендованный порядок внедрения — в [wikipedik-roadmap.md](./wikipedik-roadmap.md).
Топ-3: history-ingest backfill июня, bootstrap самого dotfiles в память,
wiki-context-pack в launch-флоу по умолчанию.

## WikiPedik memory v2 — по итогам research рынка и SOTA

Status: todo

Источник: [wikipedik-memory-research.md](./wikipedik-memory-research.md)
(deep-research 2026-06-11, 23 источника, верифицированные claims). Рынок
(Claude Code auto memory, VS Code/Copilot, Anthropic memory tool) сошёлся на
нашей архитектуре — фундамент не менять, докрутить механики:

1. **hot.md → индекс ~150-200 строк** (рыночный бюджет 200 строк/25KB) с
   однострочными указателями на topic-страницы; в SessionStart-чекпойнт —
   протокол "view memory dir first, read on demand" (Anthropic memory tool).
   Path-scoped `.claude/rules/` + симлинки как selective deep read по
   подсистемам (учесть баги #21858, #23478, #23569).
2. **JIT-верификация цитат (паттерн GitHub Copilot)**: обязательное поле
   `citations: file:line/commit` в схеме `lesson-append`; перед применением
   урока worker проверяет цитаты в коде, при противоречии пишет исправленную
   версию в inbox. Заменяет offline-курацию противоречий.
3. **Zettelkasten-линки + эволюция памяти (A-MEM, NeurIPS 2025)**: frontmatter
   wikilinks между curated-страницами; при drain нового урока куратор
   обновляет связанные старые страницы; multi-hop поиск обходом графа.
4. **Last-used экспирация**: трекинг использования curated-записей, кандидаты
   в архив через `wiki lint` (мягкий вариант 28-дневной экспирации Copilot).
5. **`autoMemoryDirectory` Claude Code → staging-зона вольта** под куратора;
   блокер: развести per-git-repo keying с privacy firewall по компаниям.

Не делать: vector DB/embeddings (ценность в графе связей, не в векторах),
hosted-платформы как замена (lock-in), авто-майнинг сессий без approve
(Cursor удалил Memories в 2.1.x именно поэтому — curator-in-the-loop).

## Automatic WikiPedik vault commits

Status: done (2026-06-11) — `scripts/wikipedik-autocommit.sh`: catch-all commit с secret-scan, без push; `launch` вызывает раз в сутки через `--if-due`, вручную — `wiki autocommit`. launchd отклонён из-за TCC-доступа к Desktop у фоновых процессов.

Problem: nothing commits the WikiPedik vault automatically — no obsidian-git plugin, no launchd/cron job, no hook. The vault has 8 manual commits total; curator work (drain, synthesize, hot.md refresh) sits uncommitted until someone runs `wiki-commit` by hand. Human edits in Obsidian are never captured at all.

Goal: vault changes get committed regularly without manual `wiki-commit`, while keeping the existing scope-atomic curator commits meaningful.

Acceptance criteria:
- A scheduled job (launchd) or `wiki sync` tail commits vault changes at least daily.
- Curator-scope changes keep the scope-atomic convention (`docs(wiki): синхронизировать память <scope>`, company `git_email` from `.company-config`); a periodic catch-all commit (e.g. `chore(vault): автокоммит несинхронизированных изменений`) covers the rest (`.obsidian/`, root files, human edits).
- Auto-commit never pushes — push stays explicit (`wiki sync --push` or manual `git push`).
- Secrets/privacy lint runs (or is at least planned) before catch-all commits so junk and sensitive content do not get silently enshrined in history.
- Decide interaction with `_wiki_dirty_outside_scope` guard: auto-commits must not break `wiki sync --commit` scope refusal logic.

## Stale active worktree handling

Status: done (2026-06-11) — `agent-workspace stale [days]` показывает active worktrees без tmux-сессии и коммитов N+ дней (dirty/unpushed счётчики); `start` печатает подсказку при наличии stale. Политика веток: `remove`/`cleanup` удаляют ветку worktree, если она merged/pushed; `agent-workspace prune-branches` чистит остальные; ветки, привязанные к worktree или с локальной работой, не трогаются никогда.

Problem: `agent-workspace start` already runs `cleanup_workspaces --days 7 --quiet`, but cleanup only removes `ready` (or merged `review`) worktrees. In practice tasks are rarely finished through `agent-finish.sh`, so worktrees stay `cleanup_state=active` forever and accumulate (2026-06-10: 20 worktrees, 4 live tmux sessions, all but one `active`).

Goal: stale `active` worktrees become visible and easy to triage instead of accumulating silently.

Acceptance criteria:
- Stale `active` worktrees (no tmux session, no commits for N days) are surfaced — e.g. a warning in `launch` output or an `agent-workspace list --stale` view with dirty/ahead status.
- Cleanup still never touches dirty or unpushed work; triage of stale worktrees stays a human decision (push / mark ready / remove).
- Decide policy for leftover `agent/work-*` branches after worktree removal (today branches accumulate with no cleanup path).

## Bootstrap-managed agent assets

Status: todo

Problem: skills, agent configs, TUI tools, and local infrastructure are currently a mix of repo files plus manual copies into `~/.codex-new`, `~/.claude-new`, and other home directories.

Goal: `~/dotfiles/bootstrap.sh` should make a fresh Mac or fresh agent profile reproducible from this repo.

Acceptance criteria:
- Bootstrap installs/syncs repo-owned skills into Codex and Claude skill directories.
- Bootstrap installs/syncs agent configs and profile files from dotfiles.
- Bootstrap builds or links local TUI tools used by `launch`.
- Bootstrap verifies required CLIs: `codex`, `claude`, `oracle`, `tmux`, `direnv`, `op`, `go`.
- Bootstrap prints manual steps only for things that cannot be automated safely, such as browser login or 1Password approval.
- Re-running bootstrap is idempotent and does not overwrite user-local secrets.

