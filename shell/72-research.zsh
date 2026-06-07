# WikiPedik research-vault helpers

_research_vault_root() {
  print -r -- "$HOME/Desktop/WikiPedik/research"
}

_research_obsidian_root() {
  print -r -- "$HOME/Desktop/WikiPedik"
}

_research_usage() {
  cat <<'EOF'
Usage:
  research status
  research lint
  research quality
  research graph-profile <root|research|study|all> [--force]

Notes:
  status/lint/quality are read-only.
  graph-profile edits ~/Desktop/WikiPedik/.obsidian/graph.json and refuses
  while Obsidian is running unless --force is passed.
EOF
}

research-status() {
  local root="$(_research_vault_root)"
  if [ ! -d "$root" ]; then
    echo "Research vault not found: ${root/#$HOME/~}"
    return 1
  fi

  python3 - "$root" <<'PY'
from pathlib import Path
import collections, re, sys

root = Path(sys.argv[1])
link_re = re.compile(r"\[\[([^\]|#]+)(?:#[^\]|]+)?(?:\|[^\]]+)?\]\]")

def frontmatter(text):
    if not text.startswith("---\n"):
        return {}
    end = text.find("\n---", 4)
    if end == -1:
        return {}
    data = {}
    for line in text[4:end].splitlines():
        if ":" in line and not line.startswith(" "):
            key, value = line.split(":", 1)
            data[key.strip()] = value.strip().strip('"')
    return data

def body_text(text):
    if not text.startswith("---\n"):
        return text
    end = text.find("\n---", 4)
    if end == -1:
        return text
    return text[end + 4:]

files = [
    p for p in root.rglob("*.md")
    if ".obsidian" not in p.parts and "99-archive" not in p.parts
]
by_layer = collections.Counter()
by_type = collections.Counter()
by_domain = collections.Counter()
outgoing = []

for path in files:
    rel = path.relative_to(root).as_posix()
    layer = rel.split("/", 1)[0]
    by_layer[layer] += 1
    text = path.read_text(errors="ignore")
    fm = frontmatter(text)
    by_type[fm.get("type", "none")] += 1
    by_domain[fm.get("domain", "none")] += 1
    outgoing.append((len(link_re.findall(body_text(text))), rel))

print("Research vault:", str(root).replace(str(Path.home()), "~"))
print("")
print("Files by layer:")
for key, value in sorted(by_layer.items()):
    print(f"  {key:12} {value}")
print("")
print("Files by type:")
for key, value in by_type.most_common():
    print(f"  {key:16} {value}")
print("")
print("Files by domain:")
for key, value in by_domain.most_common():
    print(f"  {key:16} {value}")
print("")
print("Largest outgoing-link pages:")
for count, rel in sorted(outgoing, reverse=True)[:12]:
    print(f"  {count:3} {rel}")
PY
}

research-lint() {
  local root="$(_research_vault_root)"
  if [ ! -d "$root" ]; then
    echo "Research vault not found: ${root/#$HOME/~}"
    return 1
  fi

  python3 - "$root" <<'PY'
from pathlib import Path
import re, sys

root = Path(sys.argv[1])
link_re = re.compile(r"\[\[([^\]|#]+)(?:#[^\]|]+)?(?:\|[^\]]+)?\]\]")
skip_names = {"AGENTS.md", "llm-wiki.md", "log.md", "open-questions.md"}
issues = []

def frontmatter(text):
    if not text.startswith("---\n"):
        return {}
    end = text.find("\n---", 4)
    if end == -1:
        return {"__invalid__": "missing closing frontmatter"}
    data = {}
    for lineno, line in enumerate(text[4:end].splitlines(), 2):
        if not line.strip() or line.startswith(" ") or line.startswith("-"):
            continue
        if ":" not in line:
            issues.append(("frontmatter", f"line {lineno}: no key/value", None))
            continue
        key, value = line.split(":", 1)
        data[key.strip()] = value.strip().strip('"')
    return data

def body_text(text):
    if not text.startswith("---\n"):
        return text
    end = text.find("\n---", 4)
    if end == -1:
        return text
    return text[end + 4:]

for path in sorted(root.rglob("*.md")):
    if ".obsidian" in path.parts or "99-archive" in path.parts:
        continue
    rel = path.relative_to(root).as_posix()
    text = path.read_text(errors="ignore")
    body = body_text(text)
    fm = frontmatter(text)
    links = link_re.findall(body)
    words = len(re.findall(r"\S+", body))
    typ = fm.get("type", "")
    study_type = fm.get("study_type", "")

    if path.name not in skip_names and path.name != "index.md":
        if not fm:
            issues.append(("missing-frontmatter", "no YAML frontmatter", rel))
        elif "__invalid__" in fm:
            issues.append(("invalid-frontmatter", fm["__invalid__"], rel))
        else:
            for key in ("type", "status"):
                if not fm.get(key):
                    issues.append(("missing-field", f"missing `{key}`", rel))
            if rel.startswith(("10-wiki/", "15-study/", "20-outputs/")) and not fm.get("domain"):
                issues.append(("missing-field", "missing `domain`", rel))

    if typ == "source" and len(links) > 7:
        issues.append(("link-density", f"source has {len(links)} wikilinks; target <= 7", rel))
    if typ == "concept" and len(links) > 10:
        issues.append(("link-density", f"concept has {len(links)} wikilinks; target <= 10", rel))
    if typ == "concept" and words > 1200:
        issues.append(("page-size", f"concept card has {words} words; split deep study", rel))
    if typ == "source" and "## Before Reading" not in body:
        issues.append(("source-ux", "source note missing `## Before Reading`", rel))
    if study_type == "concept-deep" and not fm.get("concept_card"):
        issues.append(("deep-study", "concept-deep missing `concept_card`", rel))

print("Research lint:", str(root).replace(str(Path.home()), "~"))
if not issues:
    print("  OK: no structural issues detected")
    raise SystemExit(0)

for kind, msg, rel in issues[:120]:
    target = f" {rel}" if rel else ""
    print(f"  [{kind}]{target}: {msg}")
if len(issues) > 120:
    print(f"  ... {len(issues) - 120} more")
raise SystemExit(1)
PY
}

research-quality() {
  local root="$(_research_vault_root)"
  if [ ! -d "$root" ]; then
    echo "Research vault not found: ${root/#$HOME/~}"
    return 1
  fi

  python3 - "$root" <<'PY'
from pathlib import Path
import re, sys

root = Path(sys.argv[1])
warnings = []

CODE_RE = re.compile(r"```.*?```", re.S)
LATIN_RE = re.compile(r"[A-Za-z]")
CYR_RE = re.compile(r"[А-Яа-яЁё]")

def frontmatter(text):
    if not text.startswith("---\n"):
        return {}
    end = text.find("\n---", 4)
    if end == -1:
        return {}
    data = {}
    for line in text[4:end].splitlines():
        if ":" in line and not line.startswith((" ", "-")):
            key, value = line.split(":", 1)
            data[key.strip()] = value.strip().strip('"')
    return data

def body_text(text):
    if not text.startswith("---\n"):
        return text
    end = text.find("\n---", 4)
    if end == -1:
        return text
    return text[end + 4:]

def has_heading(body, heading):
    return re.search(rf"^##\s+{re.escape(heading)}\s*$", body, re.M) is not None

def language_ratio(body):
    body = CODE_RE.sub("", body)
    latin = len(LATIN_RE.findall(body))
    cyr = len(CYR_RE.findall(body))
    total = latin + cyr
    return (latin / total) if total else 0.0

checks = {
    "concept": [
        "Short Definition", "Why It Matters", "Minimum Prerequisites",
        "Current Understanding", "Common Confusions", "Sources",
    ],
    "source": [
        "TL;DR", "Bibliographic Metadata", "Before Reading", "Core Concepts",
        "Core Claims", "Method / Evidence", "Key Details",
        "Contradictions / Tensions", "Open Questions",
    ],
    "concept-deep": [
        "If You Forgot Everything", "Intuition", "Formal Definition",
        "Step-By-Step Examples", "How To Use It", "Common Mistakes",
        "Where It Appears In Sources", "Practice Questions",
        "Retrieval Prompts", "Next Concepts",
    ],
    "article-breakdown": [
        "One-Sentence Takeaway", "Why This Matters", "Simple Explanation",
        "Section-By-Section Notes", "Key Terms", "Examples / Mental Models",
        "What To Remember", "Open Questions",
    ],
}

for path in sorted(root.rglob("*.md")):
    if ".obsidian" in path.parts or "99-archive" in path.parts:
        continue
    rel = path.relative_to(root).as_posix()
    text = path.read_text(errors="ignore")
    fm = frontmatter(text)
    body = body_text(text)
    typ = fm.get("type", "")
    study_type = fm.get("study_type", "")
    status = fm.get("status", "")
    words = len(re.findall(r"\S+", body))

    if path.name == "index.md" or typ == "study-index":
        continue

    key = study_type if typ == "study-note" else typ
    required = checks.get(key)
    if required:
        missing = [h for h in required if not has_heading(body, h)]
        if missing:
            warnings.append((rel, "missing-sections", ", ".join(missing)))

    if rel.startswith("15-study/"):
        ratio = language_ratio(body)
        if ratio > 0.65:
            warnings.append((rel, "language", f"learner-facing page looks too English-heavy: latin ratio {ratio:.2f}"))

    if status == "mature" and words < 1000 and key in {"concept-deep", "article-breakdown"}:
        warnings.append((rel, "maturity", f"mature {key} is short: {words} words"))
    if key == "concept-deep" and words < 800:
        warnings.append((rel, "depth", f"concept-deep is probably too shallow: {words} words"))
    if key == "article-breakdown" and words < 1200:
        warnings.append((rel, "depth", f"article-breakdown is probably too shallow: {words} words"))

print("Research quality:", str(root).replace(str(Path.home()), "~"))
if not warnings:
    print("  OK: no quality warnings detected")
    raise SystemExit(0)

for rel, kind, msg in warnings[:160]:
    print(f"  [{kind}] {rel}: {msg}")
if len(warnings) > 160:
    print(f"  ... {len(warnings) - 160} more")
raise SystemExit(1)
PY
}

research-graph-profile() {
  local profile="${1:-}"
  local force=0
  local root graph search

  shift 2>/dev/null || true
  while [ $# -gt 0 ]; do
    case "$1" in
      --force) force=1; shift ;;
      *) echo "Unknown option: $1"; return 1 ;;
    esac
  done

  case "$profile" in
    root)
      search="(path:dev/20-projects OR path:research/10-wiki OR path:research/20-outputs) -path:research/00-inbox -path:research/01-raw -path:research/99-archive -file:index -file:log -file:open-questions -file:hot -file:_inbox -file:links -file:lessons -file:gotchas -file:debugging-stories -file:decisions-not-adr -file:health -file:_synthesis-candidates"
      ;;
    research)
      search="(path:research/10-wiki OR path:research/20-outputs) -path:research/99-archive -file:index -file:log -file:open-questions"
      ;;
    study)
      search="path:research/15-study -path:research/99-archive -file:index"
      ;;
    all)
      search="(path:dev/20-projects OR path:research) -path:research/00-inbox -path:research/01-raw -path:research/99-archive -file:log -file:hot -file:_inbox"
      ;;
    *)
      echo "Usage: research graph-profile <root|research|study|all> [--force]"
      return 1
      ;;
  esac

  if [ "$force" -ne 1 ] && pgrep -x Obsidian >/dev/null 2>&1; then
    echo "Obsidian is running and can overwrite graph.json."
    echo "Close Obsidian or re-run: research graph-profile $profile --force"
    return 1
  fi

  root="$(_research_obsidian_root)"
  graph="$root/.obsidian/graph.json"
  if [ ! -f "$graph" ]; then
    echo "Graph config not found: ${graph/#$HOME/~}"
    return 1
  fi

  GRAPH_SEARCH="$search" python3 - "$graph" <<'PY'
from pathlib import Path
import json, os, sys

path = Path(sys.argv[1])
data = json.loads(path.read_text())
data["search"] = os.environ["GRAPH_SEARCH"]
data["hideUnresolved"] = True
data["showOrphans"] = False
data["showAttachments"] = False
data["showTags"] = False
data["collapse-filter"] = False
data["collapse-color-groups"] = False
data["nodeSizeMultiplier"] = 1.1
data["lineSizeMultiplier"] = 0.8
data["centerStrength"] = 0.45
data["repelStrength"] = 12
data["linkStrength"] = 0.8
data["linkDistance"] = 220
data["colorGroups"] = [
    {"query": "path:dev/20-projects/chimera", "color": {"a": 1, "rgb": 10040115}},
    {"query": "path:dev/20-projects/neurodesk", "color": {"a": 1, "rgb": 14725458}},
    {"query": "path:research/10-wiki/ml", "color": {"a": 1, "rgb": 6737322}},
    {"query": "path:research/10-wiki/quant", "color": {"a": 1, "rgb": 16755285}},
    {"query": "path:research/15-study", "color": {"a": 1, "rgb": 16759620}},
    {"query": "path:research/20-outputs", "color": {"a": 1, "rgb": 7916262}},
]
path.write_text(json.dumps(data, indent=2) + "\n")
PY
  echo "Applied research graph profile: $profile"
  echo "Graph config: ${graph/#$HOME/~}"
}

research() {
  local cmd="${1:-}"
  shift 2>/dev/null || true

  case "$cmd" in
    status) research-status "$@" ;;
    lint) research-lint "$@" ;;
    quality) research-quality "$@" ;;
    graph-profile) research-graph-profile "$@" ;;
    --help|-h|help|"") _research_usage ;;
    *)
      echo "Unknown research command: $cmd"
      _research_usage
      return 1
      ;;
  esac
}
