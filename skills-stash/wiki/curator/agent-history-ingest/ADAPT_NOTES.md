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
