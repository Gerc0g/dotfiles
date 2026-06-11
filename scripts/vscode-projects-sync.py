#!/usr/bin/env python3
"""Sync dotfiles workspaces into VS Code Project Manager projects.json."""

from __future__ import annotations

import argparse
import json
import shutil
import sys
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Any


DEFAULT_WORK_ROOT = Path.home() / "Desktop" / "Prokectfiles"
DEFAULT_PROJECT_FILE = (
    Path.home()
    / "Library"
    / "Application Support"
    / "Code"
    / "User"
    / "globalStorage"
    / "alefragnani.project-manager"
    / "projects.json"
)
MANAGED_TAG = "dotfiles"


@dataclass(frozen=True)
class ProjectEntry:
    name: str
    root_path: Path
    tags: tuple[str, ...]

    def to_json(self, existing: dict[str, Any] | None = None) -> dict[str, Any]:
        enabled = True
        profile = ""
        paths: list[str] = []

        if existing:
            enabled = bool(existing.get("enabled", True))
            profile = str(existing.get("profile", ""))
            existing_paths = existing.get("paths", [])
            if isinstance(existing_paths, list):
                paths = [str(item) for item in existing_paths]

        return {
            "name": self.name,
            "rootPath": str(self.root_path),
            "paths": paths,
            "tags": list(self.tags),
            "enabled": enabled,
            "profile": profile,
        }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Sync ~/Desktop/Prokectfiles products/repos with VS Code Project Manager."
    )
    parser.add_argument(
        "--root",
        default=str(DEFAULT_WORK_ROOT),
        help=f"workspace root, default: {DEFAULT_WORK_ROOT}",
    )
    parser.add_argument(
        "--project-file",
        default=str(DEFAULT_PROJECT_FILE),
        help=f"Project Manager projects.json, default: {DEFAULT_PROJECT_FILE}",
    )
    parser.add_argument(
        "--repos-only",
        action="store_true",
        help="sync repository entries only",
    )
    parser.add_argument(
        "--products-only",
        action="store_true",
        help="sync product folder entries only",
    )
    parser.add_argument(
        "--no-worktrees",
        action="store_true",
        help="do not include active agent worktrees",
    )
    parser.add_argument(
        "--no-prune",
        action="store_true",
        help="keep stale dotfiles-managed entries",
    )
    parser.add_argument(
        "--preserve-external",
        action="store_true",
        help="keep projects that are not managed by dotfiles",
    )
    parser.add_argument(
        "--no-backup",
        action="store_true",
        help="do not create projects.json backup before writing",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="show planned changes without writing",
    )
    parser.add_argument(
        "--check",
        action="store_true",
        help="exit 1 if projects.json is not already synced",
    )
    return parser.parse_args()


def load_projects(path: Path) -> list[dict[str, Any]]:
    if not path.exists():
        return []

    with path.open("r", encoding="utf-8") as file:
        data = json.load(file)

    if not isinstance(data, list):
        raise ValueError(f"{path} must contain a JSON array")

    projects: list[dict[str, Any]] = []
    for item in data:
        if isinstance(item, dict):
            projects.append(item)
    return projects


def is_visible_dir(path: Path) -> bool:
    return path.is_dir() and not path.name.startswith(".")


def is_repo_dir(path: Path) -> bool:
    git_marker = path / ".git"
    return git_marker.exists()


def read_metadata(path: Path) -> dict[str, str]:
    values: dict[str, str] = {}
    try:
        lines = path.read_text(encoding="utf-8").splitlines()
    except OSError:
        return values

    for line in lines:
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key] = value
    return values


def discover_entries(root: Path, include_products: bool, include_repos: bool) -> list[ProjectEntry]:
    if not root.exists():
        raise FileNotFoundError(f"workspace root not found: {root}")

    entries: list[ProjectEntry] = []
    for company_dir in sorted(root.iterdir(), key=lambda item: item.name.lower()):
        if not is_visible_dir(company_dir):
            continue

        company = company_dir.name
        for product_dir in sorted(company_dir.iterdir(), key=lambda item: item.name.lower()):
            if not is_visible_dir(product_dir):
                continue

            product = product_dir.name
            if include_products:
                entries.append(
                    ProjectEntry(
                        name=f"Product: {company}/{product}",
                        root_path=product_dir,
                        tags=(MANAGED_TAG, "product", f"company:{company}", f"product:{product}"),
                    )
                )

            if not include_repos:
                continue

            for repo_dir in sorted(product_dir.iterdir(), key=lambda item: item.name.lower()):
                if not is_visible_dir(repo_dir) or not is_repo_dir(repo_dir):
                    continue

                repo = repo_dir.name
                entries.append(
                    ProjectEntry(
                        name=repo,
                        root_path=repo_dir,
                        tags=(
                            MANAGED_TAG,
                            "repo",
                            f"company:{company}",
                            f"product:{product}",
                            f"repo:{repo}",
                        ),
                    )
                )

    return entries


def discover_worktree_entries(root: Path) -> list[ProjectEntry]:
    worktrees_root = root / ".worktrees"
    if not worktrees_root.exists():
        return []

    entries: list[ProjectEntry] = []
    for meta_path in sorted(worktrees_root.rglob(".agent-workspace")):
        wt_dir = meta_path.parent
        if not is_repo_dir(wt_dir):
            continue

        meta = read_metadata(meta_path)
        company = meta.get("company") or "unknown"
        product = meta.get("product") or "unknown"
        repo = meta.get("repo") or wt_dir.parent.name
        wt_id = meta.get("id") or wt_dir.name
        task = meta.get("task") or "work"
        state = meta.get("cleanup_state") or "unknown"
        branch = meta.get("branch") or ""

        tags = [
            MANAGED_TAG,
            "worktree",
            f"company:{company}",
            f"product:{product}",
            f"repo:{repo}",
            f"task:{task}",
            f"state:{state}",
            f"id:{wt_id}",
        ]
        if branch:
            tags.append(f"branch:{branch}")

        entries.append(
            ProjectEntry(
                name=f"{repo} @ {wt_id} · {task}",
                root_path=wt_dir,
                tags=tuple(tags),
            )
        )

    return entries


def is_managed(project: dict[str, Any]) -> bool:
    tags = project.get("tags", [])
    return isinstance(tags, list) and MANAGED_TAG in tags


def project_root(project: dict[str, Any]) -> str:
    return str(project.get("rootPath") or project.get("fullPath") or "")


def sync_projects(
    existing: list[dict[str, Any]],
    desired_entries: list[ProjectEntry],
    *,
    prune: bool,
    preserve_external: bool,
) -> tuple[list[dict[str, Any]], dict[str, int]]:
    desired_by_root = {str(entry.root_path): entry for entry in desired_entries}
    result: list[dict[str, Any]] = []
    stats = {
        "preserved": 0,
        "adopted": 0,
        "updated": 0,
        "added": 0,
        "pruned": 0,
        "external_pruned": 0,
    }

    for project in existing:
        root = project_root(project)
        desired = desired_by_root.pop(root, None)

        if desired:
            result.append(desired.to_json(project))
            if is_managed(project):
                stats["updated"] += 1
            else:
                stats["adopted"] += 1
            continue

        if prune and is_managed(project):
            stats["pruned"] += 1
            continue

        if not preserve_external and not is_managed(project):
            stats["external_pruned"] += 1
            continue

        result.append(project)
        stats["preserved"] += 1

    for entry in sorted(desired_by_root.values(), key=lambda item: item.name.lower()):
        result.append(entry.to_json())
        stats["added"] += 1

    return result, stats


def write_projects(path: Path, projects: list[dict[str, Any]], backup: bool) -> Path | None:
    path.parent.mkdir(parents=True, exist_ok=True)

    backup_path = None
    if backup and path.exists():
        timestamp = datetime.now().strftime("%Y%m%d-%H%M%S")
        backup_path = path.with_name(f"{path.name}.bak.{timestamp}")
        shutil.copy2(path, backup_path)

    with path.open("w", encoding="utf-8") as file:
        json.dump(projects, file, ensure_ascii=False, indent=4)
        file.write("\n")

    return backup_path


def main() -> int:
    args = parse_args()
    if args.repos_only and args.products_only:
        print("error: --repos-only and --products-only are mutually exclusive", file=sys.stderr)
        return 2

    root = Path(args.root).expanduser()
    project_file = Path(args.project_file).expanduser()
    include_products = not args.repos_only
    include_repos = not args.products_only
    include_worktrees = not args.no_worktrees

    existing = load_projects(project_file)
    desired = discover_entries(root, include_products, include_repos)
    if include_worktrees:
        desired.extend(discover_worktree_entries(root))
    synced, stats = sync_projects(
        existing,
        desired,
        prune=not args.no_prune,
        preserve_external=args.preserve_external,
    )

    changed = synced != existing
    print(f"Project Manager file: {project_file}")
    print(f"Workspace root: {root}")
    print(f"Discovered entries: {len(desired)}")
    print(
        "Changes: "
        f"add={stats['added']} "
        f"adopt={stats['adopted']} "
        f"update={stats['updated']} "
        f"prune={stats['pruned']} "
        f"external_prune={stats['external_pruned']} "
        f"preserve={stats['preserved']}"
    )

    if args.check:
        if changed:
            print("not synced")
            return 1
        print("already synced")
        return 0

    if args.dry_run:
        print("dry-run: no files written")
        return 0

    if not changed:
        print("already synced")
        return 0

    backup_path = write_projects(project_file, synced, backup=not args.no_backup)
    if backup_path:
        print(f"Backup: {backup_path}")
    print("synced")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
