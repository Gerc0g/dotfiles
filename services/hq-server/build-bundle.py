#!/usr/bin/env python3
"""Materialize a trusted local HQ release and write a checksummed manifest.

This packs already-built payloads. It does not download tools, copy homes,
activate credentials, or invoke the application build pipeline.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import tempfile


def safe_name(name: str) -> bool:
    value = name.lower()
    return value not in {".git", ".happier", ".codex", "auth.json", "access.key", "credentials.json"} and not value.startswith(".env")


def copy_component(source: Path, destination: Path, boundary: Path, parents=frozenset()):
    resolved = source.resolve(strict=True)
    if not resolved.is_relative_to(boundary):
        raise ValueError(f"Source symlink escapes its component: {source.name}")
    if not safe_name(source.name):
        raise ValueError(f"Credential or state path is not a release artifact: {source.name}")
    if resolved.is_dir():
        if resolved in parents:
            raise ValueError("Cyclic source symlink")
        destination.mkdir(mode=0o755)
        for child in sorted(source.iterdir()):
            copy_component(child, destination / child.name, boundary, parents | {resolved})
    elif resolved.is_file():
        shutil.copyfile(source, destination)
        destination.chmod(0o755 if source.stat().st_mode & 0o111 else 0o644)
    else:
        raise ValueError("Only regular files and directories belong in a release")


def build(args):
    output = Path(args.output).absolute()
    if output.exists():
        raise ValueError("Output already exists; use a new release directory")
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]*", args.version):
        raise ValueError("Invalid release version")
    output.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix=".hq-bundle-", dir=output.parent))
    try:
        for key in ("hq", "cli", "relay", "ui", "skills", "templates", "codex"):
            source = Path(getattr(args, key)).resolve(strict=True)
            copy_component(source, stage / key, source if source.is_dir() else source.parent)
        files = {}
        for item in sorted(stage.rglob("*")):
            if item.is_file():
                files[item.relative_to(stage).as_posix()] = {
                    "sha256": hashlib.sha256(item.read_bytes()).hexdigest(),
                    "mode": item.stat().st_mode & 0o777,
                }
        for required in ("hq", "cli/happier", "relay/happier-server", "codex/bin/codex", "ui/index.html"):
            if required not in files:
                raise ValueError(f"Required release artifact is missing: {required}")
            if required != "ui/index.html" and files[required]["mode"] != 0o755:
                raise ValueError(f"Release entrypoint is not executable: {required}")
        if not any(name.startswith("skills/") and name.endswith("/SKILL.md") for name in files):
            raise ValueError("Release lacks skills")
        for name in ("company", "product", "repo"):
            if f"templates/AGENTS.md.{name}.tmpl" not in files:
                raise ValueError(f"Required onboarding template is missing: {name}")
        engine = "libquery_engine-linux-arm64-openssl-3.0.x.so.node" if args.platform == "linux-arm64" else "libquery_engine-debian-openssl-3.0.x.so.node"
        for required in ("relay/node_modules/@prisma/client/package.json", "relay/node_modules/.prisma/client/index.js", f"relay/node_modules/.prisma/client/{engine}", "relay/prisma/sqlite/migrations/migration_lock.toml"):
            if required not in files:
                raise ValueError(f"Relay runtime closure is missing: {required}")
        if not any(name.startswith("relay/prisma/sqlite/migrations/") and name.endswith("/migration.sql") for name in files):
            raise ValueError("Relay runtime closure lacks SQLite migrations")
        manifest = {"schemaVersion": 1, "version": args.version, "platform": args.platform, "files": files}
        if args.codex_image:
            if not re.fullmatch(r"[A-Za-z0-9./:_-]+@sha256:[a-f0-9]{64}", args.codex_image):
                raise ValueError("Codex image must be pinned by digest")
            manifest["codexImage"] = args.codex_image
        (stage / "manifest.json").write_text(json.dumps(manifest, indent=2, sort_keys=True) + "\n")
        stage.chmod(0o755)
        stage.rename(output)
    finally:
        if stage.exists():
            shutil.rmtree(stage)
    return output


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ("hq", "cli", "relay", "ui", "skills", "templates", "codex", "version", "output"):
        parser.add_argument(f"--{key}", required=True)
    parser.add_argument("--platform", required=True, choices=("linux-amd64", "linux-arm64"))
    parser.add_argument("--codex-image")
    print(build(parser.parse_args()))
