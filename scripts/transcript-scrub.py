#!/usr/bin/env python3
"""Scrub high-confidence secrets from agent session transcripts.

Designed to run daily from launchd: session JSONL files live under the home
directory (~/.codex, ~/.claude), which background jobs CAN access
(unlike ~/Desktop, which macOS TCC blocks for daemons).

Safety properties:
  - only well-known token formats are redacted (no heuristic guessing);
  - replacement text is plain alphanumeric+brackets, so JSON stays valid;
  - files touched in the last 24h are skipped (a live session may be
    appending to them);
  - mtime is preserved so delta-tracking tools (ingest manifest) are not
    confused;
  - secrets are never printed — only counts and file paths.

Usage:
    transcript-scrub.py [--dry-run] [--min-age-hours 24]
"""

from __future__ import annotations

import argparse
import os
import re
import sys
import tempfile
import time
from pathlib import Path

ROOTS = [
    Path.home() / ".codex" / "sessions",
    Path.home() / ".claude" / "projects",
    # digests are redacted at creation, but older content may predate a
    # pattern — scrub them too when reachable (Desktop needs a terminal run)
    Path.home() / "Desktop" / "WikiPedik" / "dev" / "10-wiki" / "sources" / "sessions" / "_digests",
]

# (kind, regex, group-to-mask). Group 0 = whole match; named tokens keep a
# recognizable prefix after masking so you can still tell WHICH key leaked.
PATTERNS = [
    ("github", re.compile(r"(?:github_pat|gh[pousr])_[A-Za-z0-9_]{20,}"), 0),
    ("gitlab", re.compile(r"glpat-[A-Za-z0-9_-]{20,}"), 0),
    ("anthropic", re.compile(r"sk-ant-[A-Za-z0-9_-]{20,}"), 0),
    ("openrouter", re.compile(r"sk-or-v1-[A-Za-z0-9]{20,}"), 0),
    # real OpenAI keys are long unbroken base62; the dash-word form (sk-agents-…)
    # is a tmux session name, not a key — require no early dash.
    ("openai", re.compile(r"sk-(?:proj-)?[A-Za-z0-9]{32,}"), 0),
    ("grafana-sa", re.compile(r"glsa_[A-Za-z0-9]{20,}"), 0),
    ("langsmith", re.compile(r"ls(?:v2)?_(?:pt|sk)_[A-Za-z0-9]{20,}"), 0),
    ("aws", re.compile(r"AKIA[0-9A-Z]{16}"), 0),
    ("slack", re.compile(r"xox[bpars]-[A-Za-z0-9-]{10,}"), 0),
    ("npm", re.compile(r"npm_[A-Za-z0-9]{30,}"), 0),
    ("huggingface", re.compile(r"hf_[A-Za-z0-9]{30,}"), 0),
    ("stripe", re.compile(r"[sr]k_live_[A-Za-z0-9]{20,}"), 0),
    ("google", re.compile(r"AIza[0-9A-Za-z_-]{35}"), 0),
    ("sendgrid", re.compile(r"SG\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}"), 0),
    ("telegram", re.compile(r"\b\d{8,10}:AA[A-Za-z0-9_-]{30,}\b"), 0),
    ("jwt", re.compile(r"eyJ[A-Za-z0-9_-]{10,}\.eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}"), 0),
    ("bearer", re.compile(r"(?i)\bbearer\s+([A-Za-z0-9._~+/=-]{25,})"), 1),
    # postgres://user:PASSWORD@host, redis://:PASSWORD@host, etc.
    ("conn-string", re.compile(r"\b[a-z][a-z0-9+]{2,12}://[^:/@\s\"']{1,64}:([^@\s\"']{6,})@"), 1),
    # KEY=value / "api_key": "value" style assignments with secret-ish names
    ("assignment", re.compile(
        r"(?i)\b[\w.]*(?:api[_-]?key|secret|token|passwd|password|private[_-]?key)[\"']?\s*[:=]\s*[\"']([A-Za-z0-9_\-./+=]{12,})[\"']"), 1),
    ("env-var", re.compile(r"\b[A-Z][A-Z0-9_]{2,}(?:KEY|TOKEN|SECRET|PASSWORD|PASSWD)=([A-Za-z0-9_\-./+=]{10,})"), 1),
    ("private-key", re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----.*?-----END [A-Z ]*PRIVATE KEY-----", re.DOTALL), 0),
    ("private-key-open", re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----"), 0),
]

# Values that look like placeholders/already-masked — don't count them.
SKIP_VALUES = re.compile(
    r"(?i)^(x+|\*+|<[^>]+>|your[_-]|example|placeholder|redacted|masked|replace[_-]?me|changeme|\.{3})"
    r"|x{6,}|\*{4,}|…|\$\{?[A-Za-z_]+\}?"  # ${VAR} / $VAR anywhere
)


def mask_value(value: str) -> str:
    keep = 4 if len(value) > 12 else 0
    return value[:keep] + "***MASKED***"


def scrub_text(text: str) -> tuple[str, dict[str, int]]:
    hits: dict[str, int] = {}

    for kind, rx, group in PATTERNS:
        def repl(m: re.Match, kind=kind, group=group) -> str:
            value = m.group(group)
            if SKIP_VALUES.search(value) or "***MASKED***" in value or "[REDACTED" in value:
                return m.group(0)
            hits[kind] = hits.get(kind, 0) + 1
            if group == 0:
                return mask_value(value)
            start, end = m.span(group)
            outer_start = m.span(0)[0]
            whole = m.group(0)
            return whole[: start - outer_start] + mask_value(value) + whole[end - outer_start:]

        text = rx.sub(repl, text)
    return text, hits


def scrub_file(path: Path, dry_run: bool) -> dict[str, int]:
    try:
        raw = path.read_text(encoding="utf-8", errors="surrogateescape")
    except OSError:
        return {}

    cleaned, hits = scrub_text(raw)
    if not hits or dry_run:
        return hits

    stat = path.stat()
    fd, tmp = tempfile.mkstemp(dir=str(path.parent), prefix=".scrub-")
    try:
        with os.fdopen(fd, "w", encoding="utf-8", errors="surrogateescape") as fh:
            fh.write(cleaned)
        os.replace(tmp, path)
        os.utime(path, (stat.st_atime, stat.st_mtime))
    except OSError:
        try:
            os.unlink(tmp)
        except OSError:
            pass
        return {}
    return hits


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--dry-run", action="store_true", help="report, do not rewrite")
    parser.add_argument("--min-age-hours", type=float, default=24.0,
                        help="skip files modified more recently (live sessions)")
    args = parser.parse_args()

    cutoff = time.time() - args.min_age_hours * 3600
    scanned = touched = 0
    totals: dict[str, int] = {}

    for root in ROOTS:
        try:
            if not root.is_dir():
                continue
            files = list(root.rglob("*.jsonl")) + list(root.rglob("*.md"))
        except OSError as exc:
            print(f"skip root (no access): {root} ({exc})", file=sys.stderr)
            continue

        for path in files:
            try:
                if path.stat().st_mtime > cutoff:
                    continue
            except OSError:
                continue
            scanned += 1
            hits = scrub_file(path, args.dry_run)
            if hits:
                touched += 1
                detail = ", ".join(f"{k}×{v}" for k, v in sorted(hits.items()))
                verb = "would scrub" if args.dry_run else "scrubbed"
                print(f"{verb}: {path} [{detail}]")
                for k, v in hits.items():
                    totals[k] = totals.get(k, 0) + v

    summary = ", ".join(f"{k}×{v}" for k, v in sorted(totals.items())) or "ничего"
    print(f"[{time.strftime('%Y-%m-%d %H:%M:%S')}] scanned={scanned} files-with-secrets={touched} redacted: {summary}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
