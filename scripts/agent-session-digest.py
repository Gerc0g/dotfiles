#!/usr/bin/env python3
"""Condense Codex/Claude JSONL session transcripts into small markdown digests.

Raw transcripts run to many megabytes; the agent-history-ingest curator skill
cannot read them economically. This deterministic pre-extractor keeps only the
conversational signal (user prompts, assistant text, error markers) so the
curator mines digests instead of raw JSONL.

Usage:
    agent-session-digest.py --source claude --since 2026-06-01 --until 2026-06-11
    agent-session-digest.py --source codex --since 2026-06-01
    agent-session-digest.py --source all --project clodi4 --dry-run
"""

from __future__ import annotations

import argparse
import json
import re
import sys
from datetime import datetime
from pathlib import Path

CLAUDE_ROOT = Path.home() / ".claude" / "projects"
CODEX_ROOT = Path.home() / ".codex" / "sessions"
OUT_ROOT = (
    Path.home()
    / "Desktop"
    / "WikiPedik"
    / "dev"
    / "10-wiki"
    / "sources"
    / "sessions"
    / "_digests"
)

USER_SNIPPET = 600
ASSISTANT_SNIPPET = 1200
FIRST_PROMPT_LIMIT = 2000
MAX_TURNS = 120
SECRET_RE = re.compile(
    r"(BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_-]{20,}"
    r"|sk-[A-Za-z0-9]{20,}|AKIA[0-9A-Z]{16}|xox[bap]-[A-Za-z0-9-]{10,})"
)


def clean(text: str, limit: int) -> str:
    text = SECRET_RE.sub("[REDACTED]", text).strip()
    text = re.sub(r"\n{3,}", "\n\n", text)
    if len(text) > limit:
        text = text[:limit].rstrip() + " …[truncated]"
    return text


def texts_from_content(content) -> list[str]:
    """Pull human-readable text out of a message content field."""
    if isinstance(content, str):
        return [content] if content.strip() else []
    out: list[str] = []
    if isinstance(content, list):
        for item in content:
            if not isinstance(item, dict):
                continue
            kind = item.get("type", "")
            if kind in ("text", "input_text", "output_text"):
                value = item.get("text", "")
                if isinstance(value, str) and value.strip():
                    out.append(value)
    return out


def iter_jsonl(path: Path):
    try:
        with path.open("r", encoding="utf-8") as fh:
            for line in fh:
                line = line.strip()
                if not line:
                    continue
                try:
                    yield json.loads(line)
                except json.JSONDecodeError:
                    continue
    except OSError:
        return


def parse_claude(path: Path) -> dict | None:
    turns: list[tuple[str, str]] = []
    errors: list[str] = []
    meta = {"cwd": "", "branch": "", "started": ""}

    for rec in iter_jsonl(path):
        if not isinstance(rec, dict):
            continue
        if rec.get("isSidechain"):
            continue
        if not meta["cwd"] and isinstance(rec.get("cwd"), str):
            meta["cwd"] = rec["cwd"]
        if not meta["branch"] and isinstance(rec.get("gitBranch"), str):
            meta["branch"] = rec["gitBranch"]
        if not meta["started"] and isinstance(rec.get("timestamp"), str):
            meta["started"] = rec["timestamp"][:16]

        message = rec.get("message")
        if not isinstance(message, dict):
            continue
        role = message.get("role", "")
        if role not in ("user", "assistant"):
            continue

        if role == "user" and isinstance(message.get("content"), list):
            for item in message["content"]:
                if isinstance(item, dict) and item.get("type") == "tool_result" and item.get("is_error"):
                    snippet = item.get("content", "")
                    if isinstance(snippet, list):
                        snippet = " ".join(texts_from_content(snippet))
                    if isinstance(snippet, str) and snippet.strip():
                        errors.append(clean(snippet, 200))

        for text in texts_from_content(message.get("content")):
            if role == "user" and text.startswith(("<local-command", "<command-name>", "<task-notification>")):
                continue
            turns.append((role, text))

    if not turns:
        return None
    return {"meta": meta, "turns": turns, "errors": errors}


def parse_codex(path: Path) -> dict | None:
    turns: list[tuple[str, str]] = []
    meta = {"cwd": "", "branch": "", "started": ""}

    for rec in iter_jsonl(path):
        if not isinstance(rec, dict):
            continue
        if not meta["started"] and isinstance(rec.get("timestamp"), str):
            meta["started"] = rec["timestamp"][:16]
        payload = rec.get("payload")
        if not isinstance(payload, dict):
            continue
        if not meta["cwd"]:
            cwd = payload.get("cwd")
            if isinstance(cwd, str):
                meta["cwd"] = cwd
        role = payload.get("role", "")
        if payload.get("type") == "message" and role in ("user", "assistant"):
            for text in texts_from_content(payload.get("content")):
                if role == "user" and text.startswith("<ENVIRONMENT_CONTEXT>"):
                    continue
                turns.append((role, text))

    if not turns:
        return None
    return {"meta": meta, "turns": turns, "errors": []}


def render(source: str, path: Path, parsed: dict) -> str:
    meta = parsed["meta"]
    turns = parsed["turns"][:MAX_TURNS]
    users = sum(1 for role, _ in parsed["turns"] if role == "user")
    assistants = len(parsed["turns"]) - users

    lines = [
        "---",
        f"source: {source}",
        f"session_file: {path}",
        f"project_dir: {meta['cwd']}",
        f"branch: {meta['branch']}",
        f"started: {meta['started']}",
        f"turns: {users} user / {assistants} assistant",
        "status: digest",
        "---",
        "",
        f"# Session digest: {path.stem}",
        "",
    ]

    first_user = next((t for role, t in turns if role == "user"), "")
    if first_user:
        lines += ["## Task (first user message)", "", clean(first_user, FIRST_PROMPT_LIMIT), ""]

    lines += ["## Conversation signal", ""]
    for role, text in turns:
        limit = USER_SNIPPET if role == "user" else ASSISTANT_SNIPPET
        marker = "U" if role == "user" else "A"
        lines.append(f"**[{marker}]** {clean(text, limit)}")
        lines.append("")

    if parsed["errors"]:
        lines += ["## Tool errors (sample)", ""]
        for err in parsed["errors"][:10]:
            lines.append(f"- {err}")
        lines.append("")

    return "\n".join(lines)


def discover(source: str, since: str, until: str, project: str) -> list[tuple[str, Path]]:
    found: list[tuple[str, Path]] = []
    lo = since or "0000-00-00"
    hi = until or "9999-99-99"

    if source in ("claude", "all"):
        for path in sorted(CLAUDE_ROOT.glob("*/*.jsonl")):
            day = datetime.fromtimestamp(path.stat().st_mtime).strftime("%Y-%m-%d")
            if not (lo <= day <= hi):
                continue
            if project and project not in str(path.parent.name):
                continue
            found.append(("claude", path))

    if source in ("codex", "all"):
        for path in sorted(CODEX_ROOT.glob("*/*/*/*.jsonl")):
            day = "-".join(path.parts[-4:-1])
            if not (lo <= day <= hi):
                continue
            if project:
                continue  # codex sessions are not project-keyed by path
            found.append(("codex", path))

    return found


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", choices=["claude", "codex", "all"], default="all")
    parser.add_argument("--since", default="", help="YYYY-MM-DD inclusive")
    parser.add_argument("--until", default="", help="YYYY-MM-DD inclusive")
    parser.add_argument("--project", default="", help="substring filter on claude project dir")
    parser.add_argument("--out", default=str(OUT_ROOT), help=f"digest output root, default {OUT_ROOT}")
    parser.add_argument("--dry-run", action="store_true")
    args = parser.parse_args()

    out_root = Path(args.out).expanduser()
    sessions = discover(args.source, args.since, args.until, args.project)
    if not sessions:
        print("no sessions matched", file=sys.stderr)
        return 1

    written = skipped = 0
    for source, path in sessions:
        parsed = parse_claude(path) if source == "claude" else parse_codex(path)
        if parsed is None:
            skipped += 1
            continue
        day = parsed["meta"]["started"][:10] or datetime.fromtimestamp(path.stat().st_mtime).strftime("%Y-%m-%d")
        dest = out_root / source / f"{day}-{path.stem[:48]}.md"
        if args.dry_run:
            print(f"would write: {dest}")
            written += 1
            continue
        dest.parent.mkdir(parents=True, exist_ok=True)
        dest.write_text(render(source, path, parsed), encoding="utf-8")
        written += 1

    print(f"digests: {written} written, {skipped} skipped (no signal), out: {out_root}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
