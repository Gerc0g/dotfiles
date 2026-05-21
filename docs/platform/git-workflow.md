# Git Workflow

Reference for commits, branches, PRs/MRs, merge, release, push policy, and
reviewing staged/unstaged changes.

## Rules

- Read first: `git status --short --branch`, then only relevant diffs/logs.
- Preserve user/parallel-agent work. Never revert unrelated changes.
- Use explicit paths for staging and commits. Never use `git add .` or
  `git add -A`.
- Commit after each logical change when the user asks to commit.
- Conventional Commit prefix/scope in English; description in Russian by
  default.
- Do not push, force-push, rebase pushed commits, merge PRs/MRs, or delete
  branches without explicit user ask.

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

## PR/MR

Before declaring ready:

- self-review diff against target branch;
- run relevant local verification;
- report touched files and residual risk;
- do not push or open/merge remotely unless the user explicitly asked.

Deep reference: `~/dotfiles/agent-profiles/PLATFORM.md` section "Git protocol".
