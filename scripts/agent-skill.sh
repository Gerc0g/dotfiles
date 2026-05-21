#!/usr/bin/env bash
# Manage repo-owned Codex/Claude skills.

set -euo pipefail

DOTFILES="${DOTFILES:-$HOME/dotfiles}"
SKILLS_SRC="$DOTFILES/skills"
CODEX_SETUP_SKILLS="${CODEX_SETUP_SKILLS:-$HOME/.codex-setup/skills}"
CLAUDE_SETUP_SKILLS="${CLAUDE_SETUP_SKILLS:-$HOME/.claude-setup/skills}"
CODEX_DAILY_SKILLS="${CODEX_DAILY_SKILLS:-$HOME/.codex-new/skills}"
CLAUDE_DAILY_SKILLS="${CLAUDE_DAILY_SKILLS:-$HOME/.claude-new/skills}"
CODEX_WIKI_SKILLS="${CODEX_WIKI_SKILLS:-$HOME/.codex-wiki/skills}"
UNIVERSAL_SKILLS="${UNIVERSAL_SKILLS:-skill-maintainer}"

usage() {
  cat <<'EOF'
Usage: agent-skill <command> [args]

Commands:
  list              List repo-owned skills and runtime install status
  new <name>        Create ~/dotfiles/skills/<name>/SKILL.md template
  install           Symlink repo-owned skills into intended runtime profiles
  doctor            Validate SKILL.md frontmatter, runtime links, and isolation
EOF
}

skill_dirs() {
  find "$SKILLS_SRC" -mindepth 2 -maxdepth 2 -name SKILL.md -print 2>/dev/null \
    | sed 's#/SKILL.md$##' \
    | sort
}

valid_name() {
  case "$1" in
    *[!a-z0-9-]* | "" | -* | *- | *--*) return 1 ;;
    *) return 0 ;;
  esac
}

skill_name_from_dir() {
  basename "$1"
}

is_universal_skill() {
  local name=$1 item
  for item in $UNIVERSAL_SKILLS; do
    [ "$name" = "$item" ] && return 0
  done
  return 1
}

status_for() {
  local target=$1
  local src=$2
  if [ -L "$target" ]; then
    local actual
    actual="$(readlink "$target")"
    if [ "$actual" = "$src" ]; then
      printf "linked"
    else
      printf "linked-other:%s" "$actual"
    fi
  elif [ -e "$target" ]; then
    printf "exists-not-link"
  else
    printf "missing"
  fi
}

cmd_list() {
  local dir name
  for dir in $(skill_dirs); do
    name="$(skill_name_from_dir "$dir")"
    local scope="setup"
    is_universal_skill "$name" && scope="universal"
    printf "%-22s scope=%-9s setup-codex=%s setup-claude=%s daily-codex=%s daily-claude=%s wiki-codex=%s\n" \
      "$name" \
      "$scope" \
      "$(status_for "$CODEX_SETUP_SKILLS/$name" "$dir")" \
      "$(status_for "$CLAUDE_SETUP_SKILLS/$name" "$dir")" \
      "$(status_for "$CODEX_DAILY_SKILLS/$name" "$dir")" \
      "$(status_for "$CLAUDE_DAILY_SKILLS/$name" "$dir")" \
      "$(status_for "$CODEX_WIKI_SKILLS/$name" "$dir")"
  done
}

cmd_new() {
  local name="${1:-}"
  if ! valid_name "$name"; then
    echo "error: skill name must be lowercase kebab-case: [a-z0-9-]" >&2
    return 2
  fi

  local dir="$SKILLS_SRC/$name"
  local file="$dir/SKILL.md"
  if [ -e "$file" ]; then
    echo "error: skill already exists: $file" >&2
    return 1
  fi

  mkdir -p "$dir"
  cat > "$file" <<EOF
---
name: $name
description: "Use when the task involves <specific trigger words and workflow scope>."
---

# ${name}

Use this skill when <when to use>.

## Workflow

1. <step>
2. <step>
3. <step>

## Rules

- <rule>
EOF
  echo "created $file"
}

cmd_install() {
  mkdir -p "$CODEX_SETUP_SKILLS" "$CLAUDE_SETUP_SKILLS"
  mkdir -p "$CODEX_DAILY_SKILLS" "$CLAUDE_DAILY_SKILLS"
  mkdir -p "$CODEX_WIKI_SKILLS"

  local dir name
  for dir in $(skill_dirs); do
    name="$(skill_name_from_dir "$dir")"
    ln -sfn "$dir" "$CODEX_SETUP_SKILLS/$name"
    ln -sfn "$dir" "$CLAUDE_SETUP_SKILLS/$name"

    if is_universal_skill "$name"; then
      ln -sfn "$dir" "$CODEX_DAILY_SKILLS/$name"
      ln -sfn "$dir" "$CLAUDE_DAILY_SKILLS/$name"
      ln -sfn "$dir" "$CODEX_WIKI_SKILLS/$name"
      echo "linked universal $name"
    else
      if [ -L "$CODEX_DAILY_SKILLS/$name" ] && [ "$(readlink "$CODEX_DAILY_SKILLS/$name")" = "$dir" ]; then
        rm -f "$CODEX_DAILY_SKILLS/$name"
      fi

      if [ -L "$CLAUDE_DAILY_SKILLS/$name" ] && [ "$(readlink "$CLAUDE_DAILY_SKILLS/$name")" = "$dir" ]; then
        rm -f "$CLAUDE_DAILY_SKILLS/$name"
      fi

      if [ -L "$CODEX_WIKI_SKILLS/$name" ] && [ "$(readlink "$CODEX_WIKI_SKILLS/$name")" = "$dir" ]; then
        rm -f "$CODEX_WIKI_SKILLS/$name"
      fi

      echo "linked setup $name"
    fi
  done
}

daily_isolated() {
  local target=$1
  local src=$2

  if [ -L "$target" ] && [ "$(readlink "$target")" = "$src" ]; then
    return 1
  fi

  return 0
}

validate_skill() {
  local file=$1
  local dir name frontmatter line2
  dir="$(dirname "$file")"
  name="$(basename "$dir")"

  if [ "$(sed -n '1p' "$file")" != "---" ]; then
    echo "error: $file: missing opening frontmatter" >&2
    return 1
  fi

  if ! awk 'NR > 1 && $0 == "---" { found=1; exit } END { exit found ? 0 : 1 }' "$file"; then
    echo "error: $file: missing closing frontmatter" >&2
    return 1
  fi

  frontmatter="$(awk 'NR == 1 { next } $0 == "---" { exit } { print }' "$file")"

  if ! printf "%s\n" "$frontmatter" | grep -qE "^name:[[:space:]]*$name$"; then
    echo "error: $file: name must match directory ($name)" >&2
    return 1
  fi

  if ! printf "%s\n" "$frontmatter" | grep -qE '^description:[[:space:]]+".+"[[:space:]]*$'; then
    echo "error: $file: description must be quoted" >&2
    return 1
  fi
}

cmd_doctor() {
  local failed=0 dir name file

  for dir in $(skill_dirs); do
    name="$(skill_name_from_dir "$dir")"
    file="$dir/SKILL.md"

    validate_skill "$file" || failed=1

    if [ "$(status_for "$CODEX_SETUP_SKILLS/$name" "$dir")" != "linked" ]; then
      echo "error: setup codex skill not linked: $name" >&2
      failed=1
    fi

    if [ "$(status_for "$CLAUDE_SETUP_SKILLS/$name" "$dir")" != "linked" ]; then
      echo "error: setup claude skill not linked: $name" >&2
      failed=1
    fi

    if is_universal_skill "$name"; then
      if [ "$(status_for "$CODEX_DAILY_SKILLS/$name" "$dir")" != "linked" ]; then
        echo "error: universal codex daily skill not linked: $name" >&2
        failed=1
      fi

      if [ "$(status_for "$CLAUDE_DAILY_SKILLS/$name" "$dir")" != "linked" ]; then
        echo "error: universal claude daily skill not linked: $name" >&2
        failed=1
      fi

      if [ "$(status_for "$CODEX_WIKI_SKILLS/$name" "$dir")" != "linked" ]; then
        echo "error: universal codex wiki skill not linked: $name" >&2
        failed=1
      fi
    else
      if ! daily_isolated "$CODEX_DAILY_SKILLS/$name" "$dir"; then
        echo "error: repo-owned setup skill leaked into daily codex profile: $name" >&2
        failed=1
      fi

      if ! daily_isolated "$CLAUDE_DAILY_SKILLS/$name" "$dir"; then
        echo "error: repo-owned setup skill leaked into daily claude profile: $name" >&2
        failed=1
      fi

      if ! daily_isolated "$CODEX_WIKI_SKILLS/$name" "$dir"; then
        echo "error: repo-owned setup skill leaked into codex wiki profile: $name" >&2
        failed=1
      fi
    fi
  done

  if [ "$failed" -eq 0 ]; then
    echo "agent-skill doctor: ok"
  fi

  return "$failed"
}

main() {
  local cmd="${1:-}"
  shift || true

  case "$cmd" in
    list) cmd_list "$@" ;;
    new) cmd_new "$@" ;;
    install) cmd_install "$@" ;;
    doctor) cmd_doctor "$@" ;;
    -h|--help|help|"") usage ;;
    *) echo "error: unknown command: $cmd" >&2; usage >&2; return 2 ;;
  esac
}

main "$@"
