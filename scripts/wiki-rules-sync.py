#!/usr/bin/env python3
"""Materialize WikiPedik rules into repos as path-scoped Claude Code rules.

A "rule" is a binding lesson promoted by the curator. Source of truth lives in
the vault:

    20-projects/<co>/<prod>/repos/<repo>/rules/<slug>.md   (repo rule)
    20-projects/<co>/<prod>/shared/rules/<slug>.md         (product rule)

This script symlinks them into the matching checkouts as
`.claude/rules/wiki-<slug>.md` (repo rules) / `.claude/rules/wiki-product-<slug>.md`
(product rules, into every repo of the product), including active agent
worktrees. Claude Code loads them lazily via the `paths:` frontmatter, so the
always-on context does not grow. Codex sees a one-line digest via hot.md
(built by wiki-hot-refresh.py).

Rules without a `paths:` frontmatter are skipped with a warning — a rule that
would load unconditionally is context bloat by definition.

Usage:
    wiki-rules-sync.py [<co>[/<prod>[/<repo>]]] [--dry-run]
"""

from __future__ import annotations

import argparse
import re
import sys
from pathlib import Path

VAULT_ROOT = Path.home() / "Desktop" / "WikiPedik" / "dev" / "20-projects"
PROJECTS_ROOT = Path.home() / "Desktop" / "Prokectfiles"
WORKTREES_ROOT = PROJECTS_ROOT / ".worktrees"
EXCLUDE_LINE = ".claude/rules/wiki-*"


def has_paths_frontmatter(rule: Path) -> bool:
    try:
        text = rule.read_text(encoding="utf-8")
    except OSError:
        return False
    match = re.match(r"^---\n(.*?)\n---", text, re.DOTALL)
    return bool(match and re.search(r"^paths\s*:", match.group(1), re.MULTILINE))


def ensure_exclude(repo_dir: Path) -> None:
    git_dir = repo_dir / ".git"
    if git_dir.is_file():  # worktree: excludes shared via common dir
        return
    exclude = git_dir / "info" / "exclude"
    try:
        exclude.parent.mkdir(parents=True, exist_ok=True)
        existing = exclude.read_text(encoding="utf-8") if exclude.exists() else ""
        if EXCLUDE_LINE not in existing.splitlines():
            with exclude.open("a", encoding="utf-8") as fh:
                fh.write(EXCLUDE_LINE + "\n")
    except OSError:
        pass


def checkout_dirs(co: str, prod: str, repo: str) -> list[Path]:
    """Main checkout + its active agent worktrees."""
    dirs = []
    main = PROJECTS_ROOT / co / prod / repo
    if (main / ".git").exists():
        dirs.append(main)
    wt_root = WORKTREES_ROOT / co / prod / repo
    try:
        if wt_root.is_dir():
            for wt in sorted(wt_root.iterdir()):
                if (wt / ".git").exists():
                    dirs.append(wt)
    except OSError as exc:  # macOS TCC can flake on Desktop subdirs
        print(f"⚠ skip worktrees of {repo}: {exc}", file=sys.stderr)
    return dirs


def desired_links(co: str, prod: str, repo: str) -> dict[str, Path]:
    """link name -> vault rule path for one repo."""
    links: dict[str, Path] = {}
    repo_rules = VAULT_ROOT / co / prod / "repos" / repo / "rules"
    prod_rules = VAULT_ROOT / co / prod / "shared" / "rules"

    for src_dir, prefix in ((repo_rules, "wiki-"), (prod_rules, "wiki-product-")):
        if not src_dir.is_dir():
            continue
        for rule in sorted(src_dir.glob("*.md")):
            if not has_paths_frontmatter(rule):
                print(f"⚠ skip (no paths: frontmatter): {rule}", file=sys.stderr)
                continue
            links[f"{prefix}{rule.name}"] = rule
    return links


def sync_repo(co: str, prod: str, repo: str, dry_run: bool) -> tuple[int, int]:
    links = desired_links(co, prod, repo)
    linked = pruned = 0

    for checkout in checkout_dirs(co, prod, repo):
        rules_dir = checkout / ".claude" / "rules"

        for name, target in links.items():
            dest = rules_dir / name
            if dest.is_symlink() and dest.resolve() == target.resolve():
                continue
            if dry_run:
                print(f"would link: {dest} -> {target}")
            else:
                rules_dir.mkdir(parents=True, exist_ok=True)
                if dest.is_symlink() or dest.exists():
                    dest.unlink()
                dest.symlink_to(target)
            linked += 1

        if rules_dir.is_dir():
            for stale in rules_dir.glob("wiki-*"):
                if not stale.is_symlink() or stale.name in links:
                    continue
                # only manage links we created: absolute symlinks into the vault
                if not str(stale.readlink()).startswith(str(VAULT_ROOT)):
                    continue
                if dry_run:
                    print(f"would prune: {stale}")
                else:
                    stale.unlink()
                pruned += 1

        if links and not dry_run:
            ensure_exclude(checkout)

    return linked, pruned


def scope_repos(scope: str) -> list[tuple[str, str, str]]:
    parts = [p for p in scope.split("/") if p] if scope else []
    repos: list[tuple[str, str, str]] = []

    companies = [parts[0]] if parts else [d.name for d in PROJECTS_ROOT.iterdir() if d.is_dir() and not d.name.startswith(".")]
    for co in companies:
        co_dir = PROJECTS_ROOT / co
        if not co_dir.is_dir():
            continue
        products = [parts[1]] if len(parts) > 1 else [d.name for d in co_dir.iterdir() if d.is_dir() and not d.name.startswith(".")]
        for prod in products:
            prod_dir = co_dir / prod
            if not prod_dir.is_dir():
                continue
            names = [parts[2]] if len(parts) > 2 else [d.name for d in prod_dir.iterdir() if (d / ".git").exists()]
            for repo in names:
                repos.append((co, prod, repo))
    return repos


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("scope", nargs="?", default="", help="co[/prod[/repo]], default: everything")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    total_linked = total_pruned = 0
    for co, prod, repo in scope_repos(args.scope):
        linked, pruned = sync_repo(co, prod, repo, args.dry_run)
        total_linked += linked
        total_pruned += pruned

    verb = "would change" if args.dry_run else "changed"
    print(f"rules-sync: {verb} {total_linked} link(s), pruned {total_pruned}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
