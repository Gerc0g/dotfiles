---
name: autoresearch
description: "Run a bounded web research loop and file results into WikiPedik research notes. Use when the user asks to research a topic and save it to the wiki."
---

# autoresearch

Curator-side research skill. Uses web search/fetch when available, synthesizes findings, and writes structured research notes.

## Scope

In an HQ server Research task, the current workspace is the isolated research
root. Write notes to `<topic-slug>/` there and append to `log.md` only if it
already exists. Never switch to a company directory or the workstation vault.
Use the available web search/fetch tools subject to the selected network policy.

Workstation target directory:
- `~/Desktop/WikiPedik/dev/10-wiki/research/<topic-slug>/`

Optional program config:
- `references/program.md` inside this skill directory, if present.

## Workflow

1. Confirm or infer the topic.
2. Run up to 3 rounds:
   - broad search;
   - gap-fill search;
   - contradiction/source-quality check.
3. Prefer primary sources, official docs, papers, standards, and direct product docs.
4. Create one round note per round:
   - `round-1.md`
   - `round-2.md`
   - `round-3.md` when needed.
5. Create `summary.md` with key findings, confidence, open questions, and source links.
6. Append a short event to the scope's research `log.md` if it exists.

## Round Note Template

```markdown
# <topic> — round N

## Question

## Sources Checked

## Findings

## Contradictions / Uncertainty

## Next Search Angles
```

## Boundaries

- Do not store paywalled article text or long copyrighted excerpts.
- Do not store secrets, credentials, or private customer data.
- Do not write into `20-projects/<co>/...` unless the research is explicitly tied to that company/product.
- If web tools are unavailable, stop and report what could not be fetched.
