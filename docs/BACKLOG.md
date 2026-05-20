# Platform Backlog

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

