---
name: agent-history-ingest
description: Ingest local Codex and Claude session history into WikiPedik source notes. Use when the user asks to process agent history, import Codex/Claude sessions, or mine prior sessions for durable project knowledge.
---

# agent-history-ingest

Curator-side skill. Reads local agent transcripts and writes distilled source notes into WikiPedik. It does not edit project repos and does not write curated lessons directly.

## Scope

Sources — ALWAYS prefer digests over raw transcripts:
- Digests: `~/Desktop/WikiPedik/dev/10-wiki/sources/sessions/_digests/{codex,claude}/*.md` — compact extracts (user prompts + assistant text + errors, secrets redacted) produced by `python3 ~/dotfiles/scripts/agent-session-digest.py`. Frontmatter `project_dir` and `branch` give the repo scope.
- Raw fallback only when a digest is missing or clearly lost critical detail: Codex `~/.codex-new/sessions/`, Claude `~/.claude-new/projects/` (files run to many MB — never read whole, sample with grep).

If digests are missing for the requested period, run the digester first:

```bash
python3 ~/dotfiles/scripts/agent-session-digest.py --source all --since YYYY-MM-DD --until YYYY-MM-DD
```

Target:
- `~/Desktop/WikiPedik/dev/10-wiki/sources/sessions/<source>/<YYYY-MM-DD>-<slug>.md`

Manifest (inside the vault — curator sandbox cannot write outside it):
- `~/Desktop/WikiPedik/dev/10-wiki/sources/sessions/.ingest-manifest.json`
- One JSON object: `{"<digest or session path>": {"mtime": ..., "size": ..., "note": "<output note path>"}}`.

## Arguments

Accept an optional source selector in the user's request:
- `--source codex`
- `--source claude`
- `--source all` (default)

## Workflow

1. Build an inventory of candidate session files for the selected source.
2. Compare path, mtime, and size against the source manifest.
3. Process only new or modified files unless the user explicitly asks for a full re-ingest.
4. Extract durable signal only:
   - decisions made;
   - root causes found;
   - failed approaches;
   - recurring project constraints;
   - commands or workflows worth remembering.

   **Derivability test (hard filter).** Before listing anything as a candidate
   capture, ask: could a new developer re-derive this from the repo itself —
   code, `.claude/skills/*`, prompts, `AGENTS.md`, `docs/design.md`, ADRs?
   - YES → it is spec, not a lesson. Do not capture it (do not even list it
     as a weak candidate). Example: "the KDL assistant keeps a narrow
     commercial scope" is written in its skill file — skip.
   - NO → capture. Lessons live in the *divergence* from spec: where observed
     behavior contradicted the spec, what failed, what surprised, what the
     spec is silent about.
5. Redact secrets, tokens, raw `.env` values, personal identifiers, and long raw logs.
6. Write one source note per meaningful session or cluster of related sessions.
7. Update the manifest with processed files and output notes.

## Output Note Template

```markdown
---
source: codex|claude
session_id: <id or filename>
date: YYYY-MM-DD
scope: <company/product/repo if inferable>
status: source
---

# <short session title>

## Durable signal

- ...

## Project context

- ...

## Candidate captures

- ...

## Links

- source file: `<path>`
- related repo memory: `<20-projects/... if known>`
```

## Boundaries

- Do not write to `20-projects/<co>/<prod>/repos/*/lessons.md` or other curated memory pages.
- Do not write to `_inbox.md`; worker captures use `lesson-append`.
- Do not ingest whole transcripts verbatim.
- Do not cross company boundaries when inferring project scope.
- If unsure whether a transcript contains sensitive material, summarize more aggressively and mark the note `sensitive-review-needed`.

