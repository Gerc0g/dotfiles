# wiki skills stash

Staging area for Karpathy LLM Wiki skills before activation.

## Final list (9 skills + 1 hook + 1 script)

### Worker (2) — for project agents in `~/.codex-new/skills/`
- `worker/wiki-context-pack/SKILL.md` — read memory before planning [take-as-is from Ar9av]
- `worker/lesson-append/SKILL.md` — write durable lesson to _inbox.md [new custom, in repo]

### Curator (7) — for wiki agents in `~/.codex-wiki/skills/`
- `curator/source-ingest/SKILL.md` — external raw → wiki/sources/ [take-as-is from kfchou, renamed]
- `curator/agent-history-ingest/SKILL.md` — JSONL → wiki entries [merged from Ar9av's two]
- `curator/inbox-drain/SKILL.md` — _inbox.md → curated files [new custom, in repo]
- `curator/wiki-synthesize/SKILL.md` — cross-repo pattern detection [adapt Ar9av]
- `curator/wiki-lint/SKILL.md` — health checks [take-as-is from kfchou]
- `curator/wiki-status/SKILL.md` — delta report [adapt Ar9av]
- `curator/autoresearch/SKILL.md` — web research gap-fill [adapt AgriciDaniel]

### Hooks
- `hooks/auto-load-codex.sh` — codex SessionStart hot.md injection
- `hooks/auto-load-claude.sh` — claude SessionStart hot.md injection
  - Both also inject the memory-checkpoint reminder because final/stop hooks are not enabled until Codex support is verified locally.

### Shell scripts
- `scripts/wiki-bootstrap-product.sh` — create `20-projects/<co>/<prod>/repos/<repo>/` skeleton + symlinks (NEXT — port from Oracle response §4)
- `scripts/install-wiki-runtime.sh` — clone/install WikiPedik vault, skills, hooks, and profile config on a fresh machine.

## Steps to activate

### 1. Fetch community sources
```bash
bash ~/dotfiles/skills-stash/wiki/scripts/fetch-wiki-skills.sh
```

This clones Ar9av/obsidian-wiki, kfchou/wiki-skills, AgriciDaniel/claude-obsidian into `/tmp/wiki-skills-clones/` and copies SKILL.md files into the right subdirectories. Files needing manual edits land as `SKILL.md_RAW.md` with sibling `ADAPT_NOTES.md`.

### 2. Process adapt sources
Open each `SKILL.md_RAW.md` with sibling `ADAPT_NOTES.md` and apply edits. Save final result as `SKILL.md` next to the raw. Skills to adapt:
- `curator/agent-history-ingest/` (merge codex + claude versions)
- `curator/wiki-synthesize/` (drop taxonomy)
- `curator/wiki-status/` (extract delta section)
- `curator/autoresearch/` (drop DragonScale)

### 3. Activate
Preferred:

```bash
bash ~/dotfiles/skills-stash/wiki/scripts/install-wiki-runtime.sh
```

Manual equivalent:

```bash
# Worker skills (codex + claude profile)
ln -sfn "$HOME/dotfiles/skills-stash/wiki/worker/wiki-context-pack" "$HOME/.codex-new/skills/wiki-context-pack"
ln -sfn "$HOME/dotfiles/skills-stash/wiki/worker/lesson-append"     "$HOME/.codex-new/skills/lesson-append"
ln -sfn "$HOME/dotfiles/skills-stash/wiki/worker/wiki-context-pack" "$HOME/.claude-new/skills/wiki-context-pack"
ln -sfn "$HOME/dotfiles/skills-stash/wiki/worker/lesson-append"     "$HOME/.claude-new/skills/lesson-append"

# Curator skills (wiki profile)
for skill in source-ingest agent-history-ingest inbox-drain wiki-synthesize wiki-lint wiki-status autoresearch; do
  ln -sfn "$HOME/dotfiles/skills-stash/wiki/curator/$skill" "$HOME/.codex-wiki/skills/$skill"
done

# Hooks
mkdir -p ~/.codex-new/hooks ~/.claude-new/hooks
ln -sfn "$HOME/dotfiles/skills-stash/wiki/hooks/auto-load-codex.sh"  "$HOME/.codex-new/hooks/SessionStart.sh"
ln -sfn "$HOME/dotfiles/skills-stash/wiki/hooks/auto-load-codex.sh"  "$HOME/.codex-wiki/hooks/SessionStart.sh"
ln -sfn "$HOME/dotfiles/skills-stash/wiki/hooks/auto-load-claude.sh" "$HOME/.claude-new/hooks/SessionStart.sh"
```

### 4. Verify
```bash
# Codex sees worker skills in the model-visible prompt input
CODEX_HOME=~/.codex-new codex debug prompt-input "smoke" \
  | grep -E "wiki-context-pack|lesson-append"

# Codex (wiki profile) sees curator skills in the model-visible prompt input
CODEX_HOME=~/.codex-wiki codex debug prompt-input "smoke" \
  | grep -E "source-ingest|inbox-drain|wiki-status"
```

### 5. Pilot
Before full rollout — bootstrap `neurodesk/agents` (6 repos, 30 min setup) using `wiki-bootstrap-product.sh` and run 7 sanity checks from `~/dotfiles/docs/wiki-integration-design.md` Tests section.

## Source attributions

- [Ar9av/obsidian-wiki](https://github.com/Ar9av/obsidian-wiki) — wiki-context-pack, history-ingest pair, wiki-synthesize, wiki-status, wiki-stage-commit (inspiration for inbox-drain)
- [kfchou/wiki-skills](https://github.com/kfchou/wiki-skills) — wiki-ingest, wiki-lint
- [AgriciDaniel/claude-obsidian](https://github.com/AgriciDaniel/claude-obsidian) — autoresearch, hot.md pattern, AGENTS.md bootstrap text
- [Karpathy gist](https://gist.github.com/karpathy/442a6bf555914893e9891c11519de94f) — the original LLM Wiki pattern spec

License: each fetched SKILL.md retains original repo's license. Check each repo's LICENSE file before redistribution. New custom skills (`lesson-append`, `inbox-drain`, hooks) — own this work, choose your license.
