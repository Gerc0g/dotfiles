# Git Workflow

Reference for commits, branches, PRs/MRs, merge, release, push policy, and
reviewing staged/unstaged changes.

## Rules

- Read first: `git status --short --branch`, then only relevant diffs/logs.
- Preserve user/parallel-agent work. Never revert unrelated changes.
- Use explicit paths for staging and commits. Never use `git add .` or
  `git add -A`.
- Commit after each logical change when the user asks to commit, or automatically in `AGENT_GIT_MODE=commit-local`.
- Conventional Commit prefix/scope in English; description in Russian by
  default.
- Do not raw-push, force-push, rebase pushed commits, merge PRs/MRs, or delete
  branches without explicit user ask. Exception: a completed agent task finishes through `hq finish`, which verifies, pushes the task branch, and opens a draft PR/MR without merging.

## Branches

- New task branch: branch from the repo's integration branch.
- If unsure whether integration is `dev` or `main`, inspect branches/config and
  ask before switching.
- Do not rebase a branch that may already be shared.

## Commits

Recommended mechanics:

```bash
git status --short --branch
git diff -- path/to/file
git restore --staged :/
git add path/to/file1 path/to/file2
git commit -m "fix(scope): исправить конкретное поведение" -- path/to/file1 path/to/file2
```

Good commit shape:

- one logical change;
- clear scope;
- no bundled docs/tests/refactor unless they are part of the same logical
  change.


## Managed Agent Workspaces

For parallel or multi-repo agent work, avoid writing in the shared checkout. Use
managed worktrees:

```bash
agent-workspace start <company> <product> <repo> <task-slug>
```

Rules:

- default base branch is `dev` when present, otherwise `main`;
- worktree path uses a short id; agent branch is `agent/<task-slug>-<id>`;
- commit-local mode commits every completed logical change; task finish happens once with `hq finish`;
- never switch branches in a shared checkout when another agent may be active;
- stash is only an emergency tool, not the normal coordination model.

## PR/MR

Before declaring ready:

- self-review diff against target branch;
- run relevant local verification;
- report touched files and residual risk;
- in agent worktrees, use `hq finish` to push and open a draft PR/MR; do not merge remotely unless the user explicitly asked.

Deep reference: `~/dotfiles/agent-profiles/PLATFORM.md` section "Git protocol".
