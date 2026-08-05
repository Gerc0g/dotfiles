#!/usr/bin/env bash
# Manage repo-owned Codex/Claude skills.
#
# There are two runtime profiles and they are the tools' own defaults, so every
# repo-owned skill goes into both. The earlier split — setup / daily / wiki
# profiles, with a "universal" subset allowed to cross over — existed to keep
# five profiles apart. With two, it was only a way to forget a skill somewhere.

set -euo pipefail

DOTFILES="${DOTFILES:-$HOME/dotfiles}"
SKILLS_SRC="$DOTFILES/skills"
CLAUDE_SKILLS="${CLAUDE_SKILLS:-$HOME/.claude/skills}"
CODEX_SKILLS="${CODEX_SKILLS:-$HOME/.codex/skills}"

usage() {
  cat <<'EOF'
Usage: agent-skill <command> [args]

Commands:
  list              List repo-owned skills and their install status
  new <name>        Create ~/dotfiles/skills/<name>/SKILL.md template
  install           Symlink every repo-owned skill into both agent profiles
  doctor            Validate SKILL.md frontmatter, links, and orphans
EOF
}

targets() {
  printf "%s\n%s\n" "$CLAUDE_SKILLS" "$CODEX_SKILLS"
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
    printf "%-22s claude=%s codex=%s\n" \
      "$name" \
      "$(status_for "$CLAUDE_SKILLS/$name" "$dir")" \
      "$(status_for "$CODEX_SKILLS/$name" "$dir")"
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
EOF
  echo "created $file"
}

cmd_install() {
  local target dir name
  while IFS= read -r target; do
    mkdir -p "$target"
  done < <(targets)

  for dir in $(skill_dirs); do
    name="$(skill_name_from_dir "$dir")"
    while IFS= read -r target; do
      ln -sfn "$dir" "$target/$name"
    done < <(targets)
    echo "linked $name"
  done
}

validate_skill() {
  local file=$1
  local dir name frontmatter
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

# report_orphans catches what the previous doctor could not see: a link left in
# a profile after its source was deleted from the repo. Iterating over the repo
# alone can never find those, which is how a dangling skill link survived for
# weeks while doctor reported ok.
report_orphans() {
  local target entry failed=0
  while IFS= read -r target; do
    [ -d "$target" ] || continue
    for entry in "$target"/*; do
      [ -L "$entry" ] || continue
      if [ ! -e "$entry" ]; then
        echo "error: dangling skill link: $entry -> $(readlink "$entry")" >&2
        failed=1
      fi
    done
  done < <(targets)
  return "$failed"
}

cmd_doctor() {
  local failed=0 dir name file target

  for dir in $(skill_dirs); do
    name="$(skill_name_from_dir "$dir")"
    file="$dir/SKILL.md"

    validate_skill "$file" || failed=1

    while IFS= read -r target; do
      if [ "$(status_for "$target/$name" "$dir")" != "linked" ]; then
        echo "error: not linked into $target: $name" >&2
        failed=1
      fi
    done < <(targets)
  done

  report_orphans || failed=1

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
