---
name: onboard-agents-md
description: "Interactively complete company, product or repo AGENTS.md through HQ entity cards. Use after new-company/new-project or for onboard, fill todo, complete agents.md, заонбордить, заполни AGENTS.md, доделай контекст компании, вопросы по проекту. Preserve unknowns and distinguish saved content from human-confirmed onboarding."
---

# Onboard AGENTS.md

Use one guided flow for every entity: collect available facts, save supported
content, then ask for missing knowledge and human review. HQ owns the document
revision and onboarding state; the agent does not invent completion. Existing
design/architecture docs are useful evidence, not prerequisites for onboarding.

## Resolve the target first

1. Use the explicit scope from the HQ task when supplied. Otherwise resolve
   it with `hq ctx --plain`; parse the output as data, not shell code. Build
   the scope from its company/product/repo fields.
2. Read `hq entity show <scope> --json`. Use the returned `kind`,
   `document.path`, `document.content`, `document.revision`, and
   `onboarding.items`. State the document being edited. Check that a supplied
   canonical cwd matches this card on the selected machine; do not substitute
   a laptop path or a similarly named entity when the target is unavailable.
   An optional item focus must match a returned non-automatic item. It narrows
   this same guided flow to that item's sections; it is not a separate mode or
   permission to confirm other items.
3. Read the matching reference:

   | Marker | Level | Questions |
   |---|---|---|
   | `.company-config` | Company | [company](questions/company.md) |
   | `.product-config` | Product | [product](questions/product.md) |
   | `.git` directory **or file** | Repo / worktree | [repo](questions/repo.md) |

   Markers help interpret the context; do not reconstruct workspace paths from
   cwd or assume every repo has a `.git/` directory. A worktree resolves to its
   owning repo card. The card's document is canonical; inspect its checkout
   when deriving shared repo facts, not unmerged worktree changes.
4. If the entity command or registered scope is unavailable, report the exact
   gap. Do not fall back to creating directories, overwriting context, or
   fabricating state. Do not rerun company/product/repo creation as a context
   repair mechanism.

## Missing or legacy context

An explicit request to create/fill this entity's context authorizes the bounded
AGENTS.md changes below; do not add a second approval round for those same
edits. A read-only review does not authorize them.

- If the document is absent, start from the matching source template, render
  only coordinates supplied by HQ, and leave unknown facts as TODO. Do not
  execute generators or setup commands to obtain a template.
- For an existing document, preserve its rules and custom prose. Append only
  truly missing required sections for the selected item(s), using the template
  headings and known facts/TODOs. Never regenerate the whole document.
- Recognize supported legacy headings: `Local development` → `Local dev`,
  `Stack defaults` → `Stack overrides`, `Secrets & sensitive data` → `Secrets`,
  and `Conventions` → `Non-obvious patterns`. Do not add a duplicate canonical
  section when an existing alias already supplies it.
- A focused request repairs/fills that item's sections, not the entire file.
  If existing rules conflict with the proposed content, show that concrete
  conflict instead of silently replacing the user's policy.

Save through HQ with the current document revision. Creating a scaffold does
not establish facts or confirm the generated defaults.

## Walkthrough

1. Read the whole returned document and identify unresolved template fields,
   including angle-bracket placeholders. Search only `document.path` with
   `rg -n 'TODO|TBD|<[^>]+>'` to locate candidates, not to determine status.
2. **Facts first:** inspect available evidence within the selected scope.
   Company: HQ settings and existing company context. Product: registered
   children, available READMEs/manifests and architecture docs. Repo: sources
   in [repo questions](questions/repo.md). Prepare supported facts without asking
   the user to repeat what the files establish. Do not initiate deep analysis
   or create design docs/ADRs as an onboarding prerequisite.
3. Update only the intended passage in the current document string. Preserve
   other prose, sections, comments, and prior human edits. Replace a template
   comment only when its specific question was resolved; never delete HTML
   comment blocks globally.
4. Save the edit through HQ and use the returned revision for the next edit.
   Show the changed passage briefly. Unknown facts remain `TODO` with a short
   explanation; do not turn missing evidence into a default decision.
5. **Then questions:** show the factual changes and remaining missing/review
   items. Ask one question at a time, using available session answers first.
   For a new/empty repository or product, ask its intended purpose and unresolved
   choices. Separate plans from current implementation; absence of code does
   not justify inventing a stack, commands, deployment, infra or architecture.
6. Record human confirmation only for an exact backend item whose **whole
   current content** the user reviewed or explicitly supplied. A prior answer
   authorizing that content is sufficient; do not ask again solely to satisfy
   the tool. An answer covering one field does not confirm other unreviewed
   sections in the same item. Auto-filled content is never human confirmation.
7. Re-read the card at the end. Report its `onboarding.status`, completed/total,
   and the remaining `missing` or `review` items. Health findings are a separate
   report. A zero TODO count or finished agent session does not mean onboarding
   is complete. Launching this task never confirms its future generated text.

## Save and confirmation protocol

Use `hq entity update <scope> --json-base64 <payload>`, where the payload is
base64 of UTF-8 JSON. Use a JSON serializer and base64 encoder; never interpolate
document text into shell syntax. Each request contains the latest revision and
**exactly one edit**. For a document change:

```json
{"revision":"<document.revision>","content":"<complete updated document>"}
```

For a human-confirmed item, make a separate request after saving the document:

```json
{"revision":"<latest document.revision>","confirm":{"id":"<returned item id>","confirmed":true}}
```

- Use IDs returned by `onboarding.items`; never make up checklist IDs.
- Never confirm items with `automatic:true`. HQ computes them.
- `HQ_ENTITY_REVISION_CONFLICT` means reload the card, preserve concurrent
  edits, and reapply the intended change. Do not overwrite with the stale draft.
- HQ may invalidate confirmation after the corresponding section changes.
- Do not edit `.hq/entity.json` or force a status to `complete` yourself.
- This skill changes canonical AGENTS.md and the backend-managed state/history.
  Config files, source code, Makefiles, env files, infrastructure, and credentials
  are outside this workflow unless separately authorized.

## User controls

- `skip`: leave the field unresolved and continue.
- `back`: revisit the previous answer using the current card revision.
- `done` / `pause`: stop questions and retain saved progress. Neither control
  confirms remaining items or implies completion.

Do not auto-commit. If useful, show a scoped diff from the owning Git checkout;
company/product folders are not necessarily repositories. Follow existing
session authorization and the platform Git workflow for any commit request.

## References

- Templates: `~/dotfiles/templates/AGENTS.md.{company,product,repo}.tmpl`
- Creation: `hq onboard company`, `hq onboard product`, `hq onboard repo`
- Card/state contract: `hq entity show`, `hq entity update`
- Platform rules: `~/dotfiles/agent-profiles/PLATFORM.md`
