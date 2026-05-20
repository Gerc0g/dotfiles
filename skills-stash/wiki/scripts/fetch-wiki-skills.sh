#!/usr/bin/env bash
# Fetch community wiki skills sources into ~/dotfiles/skills-stash/wiki/.
# Take-as-is sources go straight in; adapt sources land in *_RAW.md for editing.
#
# Usage:
#   bash ~/dotfiles/skills-stash/wiki/scripts/fetch-wiki-skills.sh
#
# After running:
#   - take-as-is files are ready to symlink into ~/.codex-{new,wiki}/skills/
#   - adapt files (*_RAW.md) need manual editing per notes

set -euo pipefail

STASH="$HOME/dotfiles/skills-stash/wiki"
TMP_CLONES="/tmp/wiki-skills-clones"

mkdir -p "$TMP_CLONES"
mkdir -p "$STASH/worker"
mkdir -p "$STASH/curator"

clone_or_update() {
  local repo_url="$1"
  local target_dir="$2"

  if [ -d "$target_dir/.git" ]; then
    echo "Updating $target_dir..."
    git -C "$target_dir" pull --quiet 2>/dev/null || true
  else
    echo "Cloning $repo_url..."
    git clone --quiet --depth 1 "$repo_url" "$target_dir"
  fi
}

echo "=== Cloning community skill repos ==="
clone_or_update https://github.com/Ar9av/obsidian-wiki        "$TMP_CLONES/obsidian-wiki"
clone_or_update https://github.com/kfchou/wiki-skills         "$TMP_CLONES/wiki-skills"
clone_or_update https://github.com/AgriciDaniel/claude-obsidian "$TMP_CLONES/claude-obsidian"

echo ""
echo "=== Staging worker skills ==="

# wiki-context-pack — TAKE AS IS (slight description tweak after copy)
mkdir -p "$STASH/worker/wiki-context-pack"
if [ -f "$TMP_CLONES/obsidian-wiki/.skills/wiki-context-pack/SKILL.md" ]; then
  cp "$TMP_CLONES/obsidian-wiki/.skills/wiki-context-pack/SKILL.md" \
     "$STASH/worker/wiki-context-pack/SKILL.md"
  echo "✓ worker/wiki-context-pack/SKILL.md (take-as-is)"
else
  echo "⚠ wiki-context-pack/SKILL.md not found in clone — check Ar9av repo layout"
fi

# lesson-append already exists (custom, in repo)
[ -f "$STASH/worker/lesson-append/SKILL.md" ] && echo "✓ worker/lesson-append/SKILL.md (already custom)"

echo ""
echo "=== Staging curator skills ==="

# source-ingest — TAKE AS IS, but rename
mkdir -p "$STASH/curator/source-ingest"
if [ -f "$TMP_CLONES/wiki-skills/skills/wiki-ingest/SKILL.md" ]; then
  cp "$TMP_CLONES/wiki-skills/skills/wiki-ingest/SKILL.md" \
     "$STASH/curator/source-ingest/SKILL.md"
  # Update name field
  sed -i.bak 's/^name: wiki-ingest$/name: source-ingest/' \
     "$STASH/curator/source-ingest/SKILL.md"
  rm -f "$STASH/curator/source-ingest/SKILL.md.bak"
  echo "✓ curator/source-ingest/SKILL.md (take-as-is from wiki-ingest, renamed)"
fi

# agent-history-ingest — ADAPT (merge codex + claude)
mkdir -p "$STASH/curator/agent-history-ingest"
if [ -f "$TMP_CLONES/obsidian-wiki/.skills/codex-history-ingest/SKILL.md" ]; then
  cp "$TMP_CLONES/obsidian-wiki/.skills/codex-history-ingest/SKILL.md" \
     "$STASH/curator/agent-history-ingest/codex-history-ingest_RAW.md"
fi
if [ -f "$TMP_CLONES/obsidian-wiki/.skills/claude-history-ingest/SKILL.md" ]; then
  cp "$TMP_CLONES/obsidian-wiki/.skills/claude-history-ingest/SKILL.md" \
     "$STASH/curator/agent-history-ingest/claude-history-ingest_RAW.md"
fi
cat > "$STASH/curator/agent-history-ingest/ADAPT_NOTES.md" <<'EOF'
# Adapt notes — agent-history-ingest

Merge `codex-history-ingest_RAW.md` and `claude-history-ingest_RAW.md` into a single SKILL.md.

Changes:
- name: `agent-history-ingest`
- description: take a `--source <codex|claude|all>` flag (default: all)
- session paths:
  - codex: `~/.codex-new/sessions/`
  - claude: `~/.claude-new/projects/`
- manifest: single `.manifest.json` per source — `~/.codex-new/.wiki-ingest-manifest.json` etc.
- strip `.env` / Config Resolution Protocol — replace with discovery via `SCHEMA.md` walking up cwd
- output: wiki entries in `10-wiki/sources/sessions/<source>/<date>-<slug>.md`

Target length: ≤ 250 lines (current sources are ~250+ each)
EOF
echo "✓ curator/agent-history-ingest/{RAW + ADAPT_NOTES.md} (needs manual merge)"

# inbox-drain — already custom in repo
[ -f "$STASH/curator/inbox-drain/SKILL.md" ] && echo "✓ curator/inbox-drain/SKILL.md (already custom)"

# wiki-synthesize — ADAPT (drop taxonomy)
mkdir -p "$STASH/curator/wiki-synthesize"
if [ -f "$TMP_CLONES/obsidian-wiki/.skills/wiki-synthesize/SKILL.md" ]; then
  cp "$TMP_CLONES/obsidian-wiki/.skills/wiki-synthesize/SKILL.md" \
     "$STASH/curator/wiki-synthesize/SKILL.md_RAW.md"
fi
cat > "$STASH/curator/wiki-synthesize/ADAPT_NOTES.md" <<'EOF'
# Adapt notes — wiki-synthesize

Changes from SKILL.md_RAW.md:
- Drop dependency on `_meta/taxonomy.md` (we don't have it)
- Target patterns by tag co-occurrence across repos in same product/company instead
- Output: `~/Desktop/WikiPedik/dev/20-projects/<co>/<prod>/shared/patterns/<pattern-slug>.md`
- Input: `_synthesis-candidates.md` (queued by inbox-drain) + all `repos/*/lessons.md`, `gotchas.md`
- Add backlinks from constituent repo lessons to the new pattern page
- Update `<co>/<prod>/index.md` "Patterns" section after creating pattern
EOF
echo "✓ curator/wiki-synthesize/{RAW + ADAPT_NOTES.md} (needs adapt)"

# wiki-lint — TAKE AS IS
mkdir -p "$STASH/curator/wiki-lint"
if [ -f "$TMP_CLONES/wiki-skills/skills/wiki-lint/SKILL.md" ]; then
  cp "$TMP_CLONES/wiki-skills/skills/wiki-lint/SKILL.md" \
     "$STASH/curator/wiki-lint/SKILL.md"
  echo "✓ curator/wiki-lint/SKILL.md (take-as-is)"
fi

# wiki-status — ADAPT (delta section only)
mkdir -p "$STASH/curator/wiki-status"
if [ -f "$TMP_CLONES/obsidian-wiki/.skills/wiki-status/SKILL.md" ]; then
  cp "$TMP_CLONES/obsidian-wiki/.skills/wiki-status/SKILL.md" \
     "$STASH/curator/wiki-status/SKILL.md_RAW.md"
fi
cat > "$STASH/curator/wiki-status/ADAPT_NOTES.md" <<'EOF'
# Adapt notes — wiki-status

Changes from SKILL.md_RAW.md:
- KEEP: delta tracking section (what's ingested vs pending, recent additions)
- DROP: "insights mode" (hub pages, orphan-adjacent) — that's wiki-lint territory
- Target length: ≤ 150 lines (current is 461)
- Output to terminal, not file
- Optionally append summary to `<co>/<prod>/health.md`
EOF
echo "✓ curator/wiki-status/{RAW + ADAPT_NOTES.md} (needs trim)"

# autoresearch — ADAPT (drop DragonScale)
mkdir -p "$STASH/curator/autoresearch"
if [ -f "$TMP_CLONES/claude-obsidian/skills/autoresearch/SKILL.md" ]; then
  cp "$TMP_CLONES/claude-obsidian/skills/autoresearch/SKILL.md" \
     "$STASH/curator/autoresearch/SKILL.md_RAW.md"
fi
# Also copy references/program.md if present
if [ -d "$TMP_CLONES/claude-obsidian/skills/autoresearch/references" ]; then
  cp -r "$TMP_CLONES/claude-obsidian/skills/autoresearch/references" \
        "$STASH/curator/autoresearch/references_RAW"
fi
cat > "$STASH/curator/autoresearch/ADAPT_NOTES.md" <<'EOF'
# Adapt notes — autoresearch

Changes from SKILL.md_RAW.md:
- DROP DragonScale Memory boundary-first selection mode (not relevant to our setup)
- KEEP: 3-round research loop, `program.md` config externalization
- KEEP: WebSearch + WebFetch usage
- Adapt `references/program.md` to our research interests
- Target output: `10-wiki/research/<topic>/<round-N>.md`
EOF
echo "✓ curator/autoresearch/{RAW + ADAPT_NOTES.md} (needs adapt)"

echo ""
echo "=== Done ==="
echo ""
echo "Staged files:"
find "$STASH" -type f \( -name "SKILL.md" -o -name "*_RAW.md" -o -name "ADAPT_NOTES.md" \) | sort
echo ""
echo "Next steps:"
echo "  1. Review SKILL.md files (take-as-is — ready for activation)"
echo "  2. Process *_RAW.md files per ADAPT_NOTES.md instructions (manual or via codex)"
echo "  3. Activate: symlink finalized skills to ~/.codex-{new,wiki}/skills/"
echo ""
echo "Cleanup clones (optional):"
echo "  rm -rf $TMP_CLONES"
