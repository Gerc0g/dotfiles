# wiki skills stash

WikiPedik curator runtime: skills, SessionStart hooks, and install scripts.

Worker skills (`wiki-context-pack`, `lesson-append`) are NOT here anymore —
they are repo-owned in `~/dotfiles/skills/` and installed into both agent
profiles by `agent-skill install`.

## Contents

### Curator skills (7) — for wiki agents in `~/.codex/skills/`
- `curator/source-ingest/SKILL.md` — external raw → wiki/sources/
- `curator/agent-history-ingest/SKILL.md` — JSONL transcripts → wiki entries
- `curator/inbox-drain/SKILL.md` — _inbox.md → curated files
- `curator/wiki-synthesize/SKILL.md` — cross-repo pattern detection
- `curator/wiki-lint/SKILL.md` — health checks
- `curator/wiki-status/SKILL.md` — delta report
- `curator/autoresearch/SKILL.md` — web research gap-fill

### Hooks
- `hooks/auto-load-codex.sh` — codex SessionStart hot.md injection
- `hooks/auto-load-claude.sh` — claude SessionStart hot.md injection
  - Both also inject the memory-checkpoint reminder because final/stop hooks
    are not enabled until Codex support is verified locally.

### Scripts
- `scripts/wiki-bootstrap-product.sh` — shim over `hq wiki bootstrap`
  (memory skeleton + repo symlinks live in the core)
- `scripts/install-wiki-runtime.sh` — clone/install WikiPedik vault, curator
  skills, hooks, and profile config on a fresh machine

## Activate

```bash
bash ~/dotfiles/skills-stash/wiki/scripts/install-wiki-runtime.sh
```

## Verify

```bash
# Codex (wiki curation) sees curator skills in the model-visible prompt input
CODEX_HOME=~/.codex codex debug prompt-input "smoke" \
  | grep -E "source-ingest|inbox-drain|wiki-status"

# Worker skills come from agent-skill, not the stash
agent-skill doctor
```

## Source attributions

- [Ar9av/obsidian-wiki](https://github.com/Ar9av/obsidian-wiki) — wiki-context-pack, history-ingest pair, wiki-synthesize, wiki-status, wiki-stage-commit (inspiration for inbox-drain)
- [kfchou/wiki-skills](https://github.com/kfchou/wiki-skills) — wiki-ingest, wiki-lint
- [AgriciDaniel/claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian) — autoresearch, hot.md pattern, AGENTS.md bootstrap text
- [Karpathy gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) — the original LLM Wiki pattern spec

License: each fetched SKILL.md retains the original repo's license. Check each
repo's LICENSE file before redistribution.
