#!/usr/bin/env zsh
# wiki-bootstrap-product — create WikiPedik project-memory skeleton + symlinks for a product.
#
# Idempotent — safe to re-run.
#
# Usage:
#   wiki-bootstrap-product                    # interactive picker
#   wiki-bootstrap-product <co>               # all products in company
#   wiki-bootstrap-product <co> <prod>        # one product, all its repos
#   wiki-bootstrap-product <co> <prod> <repo> # single repo
#
# What it does (per product):
#   1. Ensures ~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/ skeleton exists
#      (company-level shared files + product-level shared files + per-repo memory dirs)
#   2. For each repo in ~/Desktop/Prokectfiles/<co>/<prod>/*/:
#        - creates docs/knowledge        → repos/<repo>/        (repo memory)
#        - creates docs/product-knowledge → <prod>/             (product memory)
#        - creates docs/company-knowledge → <co>/               (company memory)
#        - adds those paths to .git/info/exclude (local-only)
#   3. Updates product index.md with repo entries
#
# Privacy: stays inside company namespace. Refuses to symlink across companies.

set -euo pipefail

# ─── Config ───
PROJECTS_ROOT="$HOME/Desktop/Prokectfiles"
VAULT_ROOT="$HOME/Desktop/WikiPedik/dev/20-projects"

# ─── Helpers ───

safe_link() {
  local target="$1"
  local link="$2"

  if [[ -e "$link" && ! -L "$link" ]]; then
    echo "  ⚠ $link exists and is not a symlink. Move it manually first." >&2
    return 1
  fi

  ln -sfn "$target" "$link"
}

ensure_file() {
  local path="$1"
  local content="$2"

  if [[ ! -f "$path" ]]; then
    printf '%s\n' "$content" > "$path"
  fi
}

write_managed_file() {
  local path="$1"
  local content="$2"

  printf '%s\n' "$content" > "$path"
}

ensure_dir() {
  local d="$1"
  [[ -d "$d" ]] || mkdir -p "$d"
}

ensure_exclude_line() {
  local repo_path="$1"
  local line="$2"
  local exclude_file="$repo_path/.git/info/exclude"

  mkdir -p "$(dirname "$exclude_file")"
  touch "$exclude_file"

  grep -qxF "$line" "$exclude_file" 2>/dev/null || echo "$line" >> "$exclude_file"
}

pick_from_list() {
  # $1 = prompt, $2... = options
  local prompt="$1"; shift
  local opts=("$@")
  local n=${#opts[@]}
  [[ $n -eq 0 ]] && return 1
  [[ $n -eq 1 ]] && { echo "${opts[1]}"; return 0; }

  echo "$prompt" >&2
  local i=1
  for o in "${opts[@]}"; do
    echo "  $i) $o" >&2
    i=$((i+1))
  done
  echo -n "Choose 1-$n: " >&2
  local choice
  read choice
  [[ -z "$choice" ]] && return 1
  [[ "$choice" -lt 1 || "$choice" -gt $n ]] && return 1
  echo "${opts[$choice]}"
}

list_companies() {
  for d in "$PROJECTS_ROOT"/*/; do
    [[ -f "$d/.company-config" ]] && basename "$d"
  done
}

list_products() {
  local co=$1
  for d in "$PROJECTS_ROOT/$co"/*/; do
    [[ -f "$d/.product-config" ]] && basename "$d"
  done
}

list_repos() {
  local co=$1 prod=$2
  for d in "$PROJECTS_ROOT/$co/$prod"/*/; do
    [[ -d "$d/.git" ]] && basename "$d"
  done
}

# ─── Skeleton creators ───

bootstrap_root() {
  ensure_dir "$VAULT_ROOT"
  ensure_dir "$VAULT_ROOT/_templates"

  ensure_file "$VAULT_ROOT/index.md" "# Project memory index

Company-scoped developer memory lives here. One git repo per company (recommended).

## Companies

<!-- auto-managed list — entries added by wiki-bootstrap-product -->

## Privacy rule

- Never read across company directories unless explicitly instructed by the user.
- Root index must not contain sensitive product details.
- Each company directory has its own \`privacy.md\` with rules.
"

  ensure_file "$VAULT_ROOT/README.md" "# WikiPedik project memory

This directory follows the Karpathy LLM Wiki pattern. See \`~/dotfiles/docs/wiki-integration-design.md\` for the full design.

Structure:

    20-projects/
      <company>/         ← privacy firewall (one git repo per company)
        index.md
        log.md
        health.md
        privacy.md
        shared/          ← cross-product patterns within company
        <product>/
          index.md
          log.md
          shared/        ← cross-repo patterns within product
          repos/
            <repo>/
              _inbox.md
              lessons.md
              gotchas.md
              debugging-stories.md
              decisions-not-adr.md
              open-questions.md
              links.md
              index.md
"

  ensure_file "$VAULT_ROOT/_templates/lesson-entry.md" "# Lesson template

## <YYYY-MM-DD> | <short title>

**Scope:** <co>/<prod>/<repo>
**Tags:** #tag-a #tag-b
**Status:** candidate | validated | deprecated
**Last verified:** YYYY-MM-DD
**Source:** <commit/path/session>

**Problem:**

**Root cause:**

**Fix:**

**Failed approach (if any):**

**Reusable rule:**

**Links:**
- canonical:
- code:
- related wiki:
"

  ensure_file "$VAULT_ROOT/_templates/debugging-story.md" "# Debugging story template

## <YYYY-MM-DD> story | <title>

**Symptom:**

**Hypothesis 1:** ... (tried, result)
**Hypothesis 2:** ... (tried, result)

**Real root cause:**

**Fix:**

**Lesson:**
"

  ensure_file "$VAULT_ROOT/_templates/open-question.md" "# Open question template

## <YYYY-MM-DD> question | <title>

**Question:**

**Why it matters:**

**What we know:**

**What we don't know:**

**Next step:**
"
}

bootstrap_company() {
  local co=$1
  local CO_DIR="$VAULT_ROOT/$co"

  ensure_dir "$CO_DIR"
  ensure_dir "$CO_DIR/shared"
  ensure_dir "$CO_DIR/shared/playbooks"

  ensure_file "$CO_DIR/index.md" "# $co project memory

Scope: $co only.

## Products

<!-- auto-managed list — entries added by wiki-bootstrap-product -->

## Shared company memory

- [[shared/patterns]]
- [[shared/gotchas]]
- [[shared/glossary]]
- [[shared/cross-product-lessons]]
- [[shared/decisions-not-adr]]
"

  write_managed_file "$CO_DIR/$co.md" "# $co

Company-level memory namespace.

Use this page as the human-readable graph node for the company. The canonical agent index is [[20-projects/$co/index|$co index]].

## Navigation

- [[20-projects/$co/index|Company index]]
- [[20-projects/$co/shared/patterns|Shared patterns]]
- [[20-projects/$co/shared/gotchas|Shared gotchas]]
- [[20-projects/$co/shared/glossary|Glossary]]
- [[20-projects/$co/shared/cross-product-lessons|Cross-product lessons]]
- [[20-projects/$co/shared/decisions-not-adr|Decisions not ADR]]
"

  ensure_file "$CO_DIR/log.md" "# $co memory log

Append-only chronological log of curator events (drains, syncs, lints, promotions).
"

  ensure_file "$CO_DIR/health.md" "# $co wiki health

Last lint: never

Findings will be appended here by \`wiki-lint\`.
"

  ensure_file "$CO_DIR/privacy.md" "# $co privacy boundary

This directory is company-scoped. Do not copy, summarize, or expose its contents into another company namespace.

Do not store:
- Customer-identifying data
- Production secrets / tokens / credentials
- Raw production logs (sanitized summaries OK)
- Cross-company information

Workers (project agents) must stay within their company's namespace. Curator must refuse cross-company synthesis.
"

  ensure_file "$CO_DIR/shared/patterns.md" "# $co shared patterns

Cross-product patterns within $co. Promote here from product/shared/patterns.md when a pattern recurs in 2+ products.
"

  ensure_file "$CO_DIR/shared/gotchas.md" "# $co shared gotchas

Cross-product gotchas within $co.
"

  ensure_file "$CO_DIR/shared/glossary.md" "# $co glossary

Domain-specific terms used across products in $co.
"

  ensure_file "$CO_DIR/shared/cross-product-lessons.md" "# $co cross-product lessons

Lessons that apply across multiple products in $co.
"

  ensure_file "$CO_DIR/shared/decisions-not-adr.md" "# $co decisions not yet promoted to ADR

Informal company-wide decisions that haven't crystallized into ADRs yet.
"

  ensure_file "$CO_DIR/shared/playbooks/debugging.md" "# $co debugging playbook

Cross-product debugging procedures.
"

  ensure_file "$CO_DIR/shared/playbooks/release.md" "# $co release playbook

Cross-product release procedures.
"

  ensure_file "$CO_DIR/shared/playbooks/migration.md" "# $co migration playbook

Cross-product migration procedures.
"

  # Register in root index. Link to a unique company overview page for readable graph labels.
  if grep -qF -- "- $co: [[$co/index]]" "$VAULT_ROOT/index.md"; then
    sed -i.bak "s#- $co: \[\[$co/index\]\]#- $co: [[$co/$co]]#" "$VAULT_ROOT/index.md" 2>/dev/null && rm -f "$VAULT_ROOT/index.md.bak"
  elif ! grep -qF -- "- $co" "$VAULT_ROOT/index.md"; then
    echo "- $co: [[$co/$co]]" >> "$VAULT_ROOT/index.md"
  fi

  echo "  ✓ company skeleton: $CO_DIR"
}

bootstrap_product() {
  local co=$1 prod=$2
  local CO_DIR="$VAULT_ROOT/$co"
  local PROD_DIR="$CO_DIR/$prod"

  ensure_dir "$PROD_DIR"
  ensure_dir "$PROD_DIR/shared"
  ensure_dir "$PROD_DIR/repos"

  ensure_file "$PROD_DIR/index.md" "# $co/$prod memory index

Purpose:
- Cross-repo memory for the $prod product.
- Start here before debugging/designing recurring $prod-product problems.

## Repos

<!-- auto-managed list — entries added by wiki-bootstrap-product -->

## Shared product memory

- [[shared/patterns]]
- [[shared/gotchas]]
- [[shared/interfaces]]
- [[shared/integration-points]]
- [[shared/debugging-playbook]]
"

  write_managed_file "$PROD_DIR/$prod.md" "# $prod

Product-level memory namespace for \`$co/$prod\`.

Use this page as the human-readable graph node for the product. The canonical agent index is [[20-projects/$co/$prod/index|$co/$prod index]].

## Navigation

- [[20-projects/$co/$co|Company hub]]
- [[20-projects/$co/$prod/index|Product index]]
- [[20-projects/$co/$prod/shared/patterns|Shared patterns]]
- [[20-projects/$co/$prod/shared/gotchas|Shared gotchas]]
- [[20-projects/$co/$prod/shared/interfaces|Interfaces]]
- [[20-projects/$co/$prod/shared/integration-points|Integration points]]
- [[20-projects/$co/$prod/shared/debugging-playbook|Debugging playbook]]
- [[20-projects/$co/$prod/open-questions|Product open questions]]
"

  ensure_file "$PROD_DIR/log.md" "# $co/$prod memory log

Append-only chronological log.
"

  ensure_file "$PROD_DIR/health.md" "# $co/$prod wiki health

Last lint: never
"

  ensure_file "$PROD_DIR/open-questions.md" "# $co/$prod open questions

Product-level uncertainties. Reviewed before sprint planning.
"

  ensure_file "$PROD_DIR/shared/patterns.md" "# $co/$prod shared patterns

Patterns recurring in 2+ repos of $prod. Promoted here by curator via wiki-synthesize.
"

  ensure_file "$PROD_DIR/shared/gotchas.md" "# $co/$prod shared gotchas

Counter-intuitive things that bite across $prod repos.
"

  ensure_file "$PROD_DIR/shared/interfaces.md" "# $co/$prod interfaces

Stable contracts between repos in $prod.
"

  ensure_file "$PROD_DIR/shared/integration-points.md" "# $co/$prod integration points

Where $prod repos talk to each other and to external systems.
"

  ensure_file "$PROD_DIR/shared/decisions-not-adr.md" "# $co/$prod decisions not yet promoted to ADR

Product-level informal decisions.
"

  ensure_file "$PROD_DIR/shared/debugging-playbook.md" "# $co/$prod debugging playbook

Step-by-step procedures for known $prod debugging scenarios.
"

  ensure_file "$PROD_DIR/_synthesis-candidates.md" "# $co/$prod synthesis candidates

Queue of lessons flagged by inbox-drain as candidates for cross-repo pattern synthesis. Processed by wiki-synthesize.
"

  # Register in company index. Link to a unique product overview page for readable graph labels.
  if grep -qF -- "- $prod: [[$prod/index]]" "$CO_DIR/index.md"; then
    sed -i.bak "s#- $prod: \[\[$prod/index\]\]#- $prod: [[$prod/$prod]]#" "$CO_DIR/index.md" 2>/dev/null && rm -f "$CO_DIR/index.md.bak"
  elif ! grep -qF -- "- $prod" "$CO_DIR/index.md"; then
    sed -i.bak "/^## Products/a\\
- $prod: [[$prod/$prod]]" "$CO_DIR/index.md" 2>/dev/null && rm -f "$CO_DIR/index.md.bak"
  fi

  echo "  ✓ product skeleton: $PROD_DIR"
}

bootstrap_repo() {
  local co=$1 prod=$2 repo=$3

  local REPO_PATH="$PROJECTS_ROOT/$co/$prod/$repo"
  local CO_DIR="$VAULT_ROOT/$co"
  local PROD_DIR="$CO_DIR/$prod"
  local REPO_MEM="$PROD_DIR/repos/$repo"

  if [[ ! -d "$REPO_PATH/.git" ]]; then
    echo "  ⚠ $REPO_PATH is not a git repo — skipping" >&2
    return 1
  fi

  ensure_dir "$REPO_MEM"

  ensure_file "$REPO_MEM/index.md" "# $co/$prod/$repo memory index

Canonical repo docs (read FIRST for facts):
- repo design: \`$REPO_PATH/docs/design.md\`
- repo AGENTS: \`$REPO_PATH/AGENTS.md\`
- ADRs: \`$REPO_PATH/docs/adr/\`

## Recent memory

<!-- auto-managed by inbox-drain -->

## Memory pages

- [[lessons]]
- [[debugging-stories]]
- [[gotchas]]
- [[decisions-not-adr]]
- [[open-questions]]
- [[links]]
"

  write_managed_file "$REPO_MEM/$repo.md" "# $repo

Scope: \`$co/$prod/$repo\`.

Use this page as the human-readable graph node for the repo. Agent-facing memory stays in the stable files below.

## Memory

- [[20-projects/$co/$prod/repos/$repo/hot|Hot context]]
- [[20-projects/$co/$prod/repos/$repo/lessons|Lessons]]
- [[20-projects/$co/$prod/repos/$repo/gotchas|Gotchas]]
- [[20-projects/$co/$prod/repos/$repo/debugging-stories|Debugging stories]]
- [[20-projects/$co/$prod/repos/$repo/decisions-not-adr|Decisions not ADR]]
- [[20-projects/$co/$prod/repos/$repo/open-questions|Open questions]]
- [[20-projects/$co/$prod/repos/$repo/links|Links]]
- [[20-projects/$co/$prod/repos/$repo/_inbox|Inbox]]

## Parent context

- Company: [[20-projects/$co/$co|$co]]
- Product: [[20-projects/$co/$prod/$prod|$co/$prod]]
"

  ensure_file "$REPO_MEM/_inbox.md" "# $repo memory inbox

Append-only raw captures by Worker agent. Drained into curated files by Curator (wiki-sync command).

Entries below this line. Marked \`Status: drained\` after curation; \`Status: rejected\` if low-signal.

---
"

  ensure_file "$REPO_MEM/hot.md" "# $repo hot context

Auto-refreshed by curator. Loaded by SessionStart hook on every codex/claude session in this repo.

Keep this file small (~50 lines). Include only:
- 2-3 most recent durable lessons
- Currently active open questions
- Critical gotchas relevant right now

<!-- last refreshed: never -->
"

  ensure_file "$REPO_MEM/lessons.md" "# $repo lessons

Durable validated lessons. Promoted from _inbox.md by Curator.

Each entry format: see \`../../../_templates/lesson-entry.md\`.
"

  ensure_file "$REPO_MEM/debugging-stories.md" "# $repo debugging stories

Narratives of complex debug sessions — symptoms, hypotheses tried, real root cause, fix, lesson.

Each entry format: see \`../../../_templates/debugging-story.md\`.
"

  ensure_file "$REPO_MEM/gotchas.md" "# $repo gotchas

Counter-intuitive things that bite in this codebase. Failed approaches worth not repeating.
"

  ensure_file "$REPO_MEM/decisions-not-adr.md" "# $repo decisions not yet promoted to ADR

Informal decisions in this repo. If a decision becomes binding → promote to docs/adr/ in the actual repo.
"

  ensure_file "$REPO_MEM/open-questions.md" "# $repo open questions

Unresolved questions specific to this repo.

Each entry format: see \`../../../_templates/open-question.md\`.
"

  ensure_file "$REPO_MEM/links.md" "# $repo memory links

Canonical sources to consult:
- design: \`$REPO_PATH/docs/design.md\`
- AGENTS: \`$REPO_PATH/AGENTS.md\`
- ADRs: \`$REPO_PATH/docs/adr/\`
- README: \`$REPO_PATH/README.md\`

External:
"

  # Create 3 symlinks in repo
  ensure_dir "$REPO_PATH/docs"

  safe_link "$REPO_MEM" "$REPO_PATH/docs/knowledge" || return 1
  safe_link "$PROD_DIR" "$REPO_PATH/docs/product-knowledge" || return 1
  safe_link "$CO_DIR"   "$REPO_PATH/docs/company-knowledge" || return 1

  # Mark links as local-only (don't commit)
  ensure_exclude_line "$REPO_PATH" "docs/knowledge"
  ensure_exclude_line "$REPO_PATH" "docs/product-knowledge"
  ensure_exclude_line "$REPO_PATH" "docs/company-knowledge"

  # Register in product index. Link to the unique repo overview page so Obsidian graph
  # shows repo names instead of many indistinguishable index nodes.
  if grep -qF -- "- $repo: [[repos/$repo/index]]" "$PROD_DIR/index.md"; then
    sed -i.bak "s#- $repo: \[\[repos/$repo/index\]\]#- $repo: [[repos/$repo/$repo]]#" "$PROD_DIR/index.md" 2>/dev/null && rm -f "$PROD_DIR/index.md.bak"
  elif ! grep -qF -- "- $repo:" "$PROD_DIR/index.md"; then
    sed -i.bak "/^## Repos/a\\
- $repo: [[repos/$repo/$repo]]" "$PROD_DIR/index.md" 2>/dev/null && rm -f "$PROD_DIR/index.md.bak"
  fi

  echo "  ✓ repo: $repo ($REPO_PATH)"
}

# ─── Sanity checks ───

sanity_check_repo() {
  local co=$1 prod=$2 repo=$3
  local REPO_PATH="$PROJECTS_ROOT/$co/$prod/$repo"
  local CO_DIR="$VAULT_ROOT/$co"

  echo ""
  echo "─── Sanity check: $repo ───"

  for link in docs/knowledge docs/product-knowledge docs/company-knowledge; do
    if [[ -L "$REPO_PATH/$link" ]]; then
      echo "  ✓ $link → $(readlink "$REPO_PATH/$link")"
    else
      echo "  ✗ $link missing or not a symlink"
    fi
  done

  # Verify company-knowledge points inside this company only
  local company_link_target=$(readlink "$REPO_PATH/docs/company-knowledge" 2>/dev/null || echo "")
  if [[ "$company_link_target" == "$CO_DIR" ]]; then
    echo "  ✓ privacy: stays in $co namespace"
  else
    echo "  ✗ privacy: company-knowledge points outside expected company"
  fi

  # Verify excluded from git
  (cd "$REPO_PATH" && git status --short 2>/dev/null | grep -E "docs/(knowledge|product-knowledge|company-knowledge)" >/dev/null) \
    && echo "  ✗ symlinks NOT excluded from git" \
    || echo "  ✓ symlinks excluded from git"
}

# ─── Main ───

main() {
  bootstrap_root

  local co prod repo

  case $# in
    0)
      # Interactive
      local companies=($(list_companies))
      [[ ${#companies[@]} -eq 0 ]] && { echo "⚠ No companies in $PROJECTS_ROOT"; exit 1; }
      co=$(pick_from_list "Companies:" "${companies[@]}") || exit 1

      local products=($(list_products "$co"))
      [[ ${#products[@]} -eq 0 ]] && { echo "⚠ No products in $co"; exit 1; }
      prod=$(pick_from_list "Products in $co:" "${products[@]}") || exit 1
      ;;
    1)
      co=$1
      ;;
    2)
      co=$1; prod=$2
      ;;
    3)
      co=$1; prod=$2; repo=$3
      ;;
    *)
      echo "Usage: wiki-bootstrap-product [<co> [<prod> [<repo>]]]"
      exit 1
      ;;
  esac

  [[ ! -d "$PROJECTS_ROOT/$co" ]] && { echo "⚠ Company not found: $PROJECTS_ROOT/$co"; exit 1; }

  echo ""
  echo "→ Bootstrapping company: $co"
  bootstrap_company "$co"

  if [[ -z "${prod:-}" ]]; then
    # All products in company
    for p in $(list_products "$co"); do
      echo ""
      echo "→ Bootstrapping product: $co/$p"
      bootstrap_product "$co" "$p"
      for r in $(list_repos "$co" "$p"); do
        bootstrap_repo "$co" "$p" "$r" || continue
      done
    done
  else
    echo ""
    echo "→ Bootstrapping product: $co/$prod"
    bootstrap_product "$co" "$prod"

    if [[ -z "${repo:-}" ]]; then
      # All repos in product
      for r in $(list_repos "$co" "$prod"); do
        bootstrap_repo "$co" "$prod" "$r" || continue
      done

      echo ""
      echo "─── Sanity checks ───"
      for r in $(list_repos "$co" "$prod"); do
        sanity_check_repo "$co" "$prod" "$r"
      done
    else
      bootstrap_repo "$co" "$prod" "$repo"
      sanity_check_repo "$co" "$prod" "$repo"
    fi
  fi

  echo ""
  echo "═══════════════════════════════════════════════════════════════════"
  echo " ✅ wiki-bootstrap-product complete"
  echo "═══════════════════════════════════════════════════════════════════"
  echo ""
  echo "Next steps:"
  echo "  1. Open in Obsidian: $VAULT_ROOT"
  echo "  2. Visit one repo to verify symlinks:"
  echo "       cd $PROJECTS_ROOT/$co/${prod:-<prod>}/${repo:-<repo>}"
  echo "       ls -la docs/knowledge docs/product-knowledge docs/company-knowledge"
  echo "  3. Worker agents will start writing to _inbox.md on durable lessons"
  echo "  4. Curator (wikipedik) processes inbox: 'wiki sync $co/${prod:-<prod>}'"
}

main "$@"
