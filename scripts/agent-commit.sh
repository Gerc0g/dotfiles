#!/usr/bin/env bash
# Commit one logical agent change with explicit paths only. Does not push.
# Usage: agent-commit.sh "type(scope): русское описание" -- path/to/file [path/to/dir ...]

set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage: agent-commit.sh "type(scope): русское описание" -- path/to/file [path/to/dir ...]

Rules:
  - explicit paths only; never git add . or git add -A
  - commits one logical change
  - does not push; use agent-task-push.sh when the task/branch is complete
  - no force, no rebase, no merge
EOF
}

if [ $# -lt 3 ]; then
  usage
  exit 64
fi

message=$1
shift
if [ "${1:-}" != "--" ]; then
  usage
  exit 64
fi
shift
if [ $# -eq 0 ]; then
  usage
  exit 64
fi

case "$message" in
  feat\(*|fix\(*|refactor\(*|build\(*|ci\(*|chore\(*|docs\(*|style\(*|perf\(*|test\(*) ;;
  feat:*|fix:*|refactor:*|build:*|ci:*|chore:*|docs:*|style:*|perf:*|test:*) ;;
  *)
    echo "error: commit message must use Conventional Commit prefix" >&2
    echo "example: feat(storage): добавить репозиторий daily logs" >&2
    exit 64
    ;;
esac

repo_root=$(git rev-parse --show-toplevel 2>/dev/null) || {
  echo "error: not inside a git repository" >&2
  exit 64
}
cd "$repo_root"

branch=$(git branch --show-current)
if [ -z "$branch" ]; then
  echo "error: detached HEAD; refusing to commit" >&2
  exit 65
fi

# Verify local git identity against company config when repo lives under
# ~/Desktop/Prokectfiles/<company>/...
expected_email=""
case "$repo_root" in
  "$HOME"/Desktop/Prokectfiles/*/*)
    company=${repo_root#"$HOME"/Desktop/Prokectfiles/}
    company=${company%%/*}
    cfg="$HOME/Desktop/Prokectfiles/$company/.company-config"
    if [ -f "$cfg" ]; then
      expected_email=$(awk '/^git_email:/ {print $2; exit}' "$cfg")
    fi
    ;;
esac

if [ -n "$expected_email" ]; then
  current_email=$(git config --get user.email || true)
  if [ "$current_email" != "$expected_email" ]; then
    echo "error: git user.email mismatch for company" >&2
    echo "  expected: $expected_email" >&2
    echo "  current:  ${current_email:-<unset>}" >&2
    echo "fix: git config user.email '$expected_email'" >&2
    exit 65
  fi
fi

# Reset only the index, not the working tree. This prevents accidentally
# committing paths staged by another agent.
git restore --staged :/ >/dev/null 2>&1 || true

paths=()
for path in "$@"; do
  case "$path" in
    .|./|-A|--all|:/)
      echo "error: broad path '$path' is not allowed" >&2
      exit 64
      ;;
  esac
  if [ ! -e "$path" ]; then
    if ! git ls-files --error-unmatch -- "$path" >/dev/null 2>&1; then
      echo "error: path does not exist and is not tracked: $path" >&2
      exit 66
    fi
  fi
  paths+=("$path")
done

if git diff --quiet -- "${paths[@]}" && git diff --cached --quiet -- "${paths[@]}"; then
  untracked=$(git ls-files --others --exclude-standard -- "${paths[@]}")
  if [ -z "$untracked" ]; then
    echo "nothing to commit in selected paths" >&2
    exit 0
  fi
fi

if [ "${AGENT_GIT_VERIFY:-1}" != "0" ]; then
  if [ -f Makefile ] && grep -qE '^verify:' Makefile; then
    make verify
  elif [ -f Makefile ] && grep -qE '^test:' Makefile; then
    make test
  fi
fi

git add -- "${paths[@]}"

if git diff --cached --quiet; then
  echo "nothing staged after explicit add" >&2
  exit 0
fi

printf '\n--- staged diffstat ---\n'
git diff --cached --stat
printf '\n--- committing ---\n'
git commit -m "$message" -- "${paths[@]}"
printf '\n✓ committed locally: %s\n' "$message"
printf 'Next when task is complete: agent-task-push.sh\n'
