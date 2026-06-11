# Platform Backlog

## Automatic WikiPedik vault commits

Status: todo

Problem: nothing commits the WikiPedik vault automatically — no obsidian-git plugin, no launchd/cron job, no hook. The vault has 8 manual commits total; curator work (drain, synthesize, hot.md refresh) sits uncommitted until someone runs `wiki-commit` by hand. Human edits in Obsidian are never captured at all.

Goal: vault changes get committed regularly without manual `wiki-commit`, while keeping the existing scope-atomic curator commits meaningful.

Acceptance criteria:
- A scheduled job (launchd) or `wiki sync` tail commits vault changes at least daily.
- Curator-scope changes keep the scope-atomic convention (`docs(wiki): синхронизировать память <scope>`, company `git_email` from `.company-config`); a periodic catch-all commit (e.g. `chore(vault): автокоммит несинхронизированных изменений`) covers the rest (`.obsidian/`, root files, human edits).
- Auto-commit never pushes — push stays explicit (`wiki sync --push` or manual `git push`).
- Secrets/privacy lint runs (or is at least planned) before catch-all commits so junk and sensitive content do not get silently enshrined in history.
- Decide interaction with `_wiki_dirty_outside_scope` guard: auto-commits must not break `wiki sync --commit` scope refusal logic.

## Stale active worktree handling

Status: todo

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

