#!/usr/bin/env python3
"""Deterministically rebuild WikiPedik hot.md from curated repo memory pages.

hot.md is what SessionStart hooks inject into every agent session. The curator
LLM is supposed to refresh it during inbox-drain, but that step has been
skipped by the model before — this script is the guaranteed fallback.

Usage:
    wiki-hot-refresh.py <company>[/<product>[/<repo>]] [--dry-run]
"""

from __future__ import annotations

import argparse
import re
import sys
from datetime import datetime
from pathlib import Path

VAULT_ROOT = Path.home() / "Desktop" / "WikiPedik" / "dev" / "20-projects"

LESSONS_LIMIT = 3
GOTCHAS_LIMIT = 5
QUESTIONS_LIMIT = 5


def slugify(title: str) -> str:
    slug = re.sub(r"[^\w\s-]", "", title.lower(), flags=re.UNICODE)
    return re.sub(r"[\s_]+", "-", slug).strip("-")


def entry_titles(page: Path) -> list[str]:
    """Return '## ' entry titles in file order (oldest first)."""
    if not page.is_file():
        return []
    titles: list[str] = []
    for line in page.read_text(encoding="utf-8").splitlines():
        if line.startswith("## ") and not line.startswith("## ["):
            titles.append(line[3:].strip())
        elif line.startswith("## ["):
            # inbox-style heading: "## [date] capture | title"
            part = line.split("|", 1)
            titles.append(part[1].strip() if len(part) == 2 else line[3:].strip())
    return titles


def section(label: str, page_name: str, titles: list[str], limit: int) -> list[str]:
    lines = [f"## {label}", ""]
    if not titles:
        lines.append(f"- No entries in {page_name} yet.")
    else:
        for title in titles[-limit:][::-1]:
            lines.append(f"- {page_name}#{slugify(title)}: {title}")
    lines.append("")
    return lines


def build_hot(repo_mem: Path, repo: str) -> str:
    lessons = entry_titles(repo_mem / "lessons.md")
    gotchas = entry_titles(repo_mem / "gotchas.md")
    questions = entry_titles(repo_mem / "open-questions.md")

    stamp = datetime.now().strftime("%Y-%m-%d %H:%M")
    lines = [
        f"# {repo} hot context",
        "",
        "Auto-refreshed by curator. Loaded by SessionStart hook on every codex/claude session in this repo.",
        "",
    ]
    lines += section("Fresh Lessons", "lessons.md", lessons, LESSONS_LIMIT)
    lines += section("Critical Gotchas", "gotchas.md", gotchas, GOTCHAS_LIMIT)
    lines += section("Open Questions", "open-questions.md", questions, QUESTIONS_LIMIT)
    lines.append(f"<!-- last refreshed: {stamp} (wiki-hot-refresh) -->")
    return "\n".join(lines) + "\n"


def repo_dirs_for_scope(scope: str) -> list[Path]:
    parts = [p for p in scope.split("/") if p]
    if not parts or len(parts) > 3:
        raise ValueError(f"bad scope: {scope!r} (want co[/prod[/repo]])")

    if len(parts) == 3:
        return [VAULT_ROOT / parts[0] / parts[1] / "repos" / parts[2]]

    root = VAULT_ROOT / parts[0]
    if len(parts) == 2:
        root = root / parts[1]
    if not root.is_dir():
        raise FileNotFoundError(f"scope not found: {root}")
    return sorted(p.parent for p in root.glob("**/repos/*/index.md"))


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("scope", help="company[/product[/repo]]")
    parser.add_argument("--dry-run", action="store_true", help="print targets without writing")
    args = parser.parse_args()

    try:
        repo_dirs = repo_dirs_for_scope(args.scope)
    except (ValueError, FileNotFoundError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 1

    if not repo_dirs:
        print(f"no repo memory dirs under scope {args.scope}", file=sys.stderr)
        return 1

    for repo_mem in repo_dirs:
        if not repo_mem.is_dir():
            print(f"skip (missing): {repo_mem}", file=sys.stderr)
            continue
        repo = repo_mem.name
        content = build_hot(repo_mem, repo)
        target = repo_mem / "hot.md"
        if args.dry_run:
            print(f"would refresh: {target}")
            continue
        target.write_text(content, encoding="utf-8")
        print(f"refreshed: {target}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
