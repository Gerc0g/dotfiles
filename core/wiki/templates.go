package wiki

// The bootstrap page templates, ported verbatim from the zsh heredocs.
// Placeholders {co}, {prod}, {repo}, {repoPath} are substituted by render().

const tplRootIndex = `# Project memory index

Company-scoped developer memory lives here. One git repo per company (recommended).

## Companies

<!-- auto-managed list — entries added by wiki-bootstrap-product -->

## Privacy rule

- Never read across company directories unless explicitly instructed by the user.
- Root index must not contain sensitive product details.
- Each company directory has its own ` + "`privacy.md`" + ` with rules.
`

const tplRootReadme = `# WikiPedik project memory

This directory follows the Karpathy LLM Wiki pattern. See ` + "`~/dotfiles/docs/wiki-integration-design.md`" + ` for the full design.

Structure:

    20-projects/
      <company>/         ← privacy firewall (one git repo per company)
        index.md
        log.md
        health.md
        privacy.md
        shared/          ← cross-product patterns within company
        <product>/
          index.md
          log.md
          shared/        ← cross-repo patterns within product
          repos/
            <repo>/
              _inbox.md
              lessons.md
              gotchas.md
              debugging-stories.md
              decisions-not-adr.md
              open-questions.md
              links.md
              index.md
`

const tplLessonEntry = `# Lesson template

## <YYYY-MM-DD> | <short title>

**Scope:** <co>/<prod>/<repo>
**Tags:** #tag-a #tag-b
**Status:** candidate | validated | deprecated
**Last verified:** YYYY-MM-DD
**Source:** <commit/path/session>

**Problem:**

**Root cause:**

**Fix:**

**Failed approach (if any):**

**Reusable rule:**

**Links:**
- canonical:
- code:
- related wiki:
`

const tplDebuggingStory = `# Debugging story template

## <YYYY-MM-DD> story | <title>

**Symptom:**

**Hypothesis 1:** ... (tried, result)
**Hypothesis 2:** ... (tried, result)

**Real root cause:**

**Fix:**

**Lesson:**
`

const tplOpenQuestion = `# Open question template

## <YYYY-MM-DD> question | <title>

**Question:**

**Why it matters:**

**What we know:**

**What we don't know:**

**Next step:**
`

const tplCompanyIndex = `# {co} project memory

Scope: {co} only.

## Products

<!-- auto-managed list — entries added by wiki-bootstrap-product -->

## Shared company memory

- [[shared/patterns]]
- [[shared/gotchas]]
- [[shared/glossary]]
- [[shared/cross-product-lessons]]
- [[shared/decisions-not-adr]]
`

const tplCompanyPage = `# {co}

Company-level memory namespace.

Use this page as the human-readable graph node for the company. The canonical agent index is [[20-projects/{co}/index|{co} index]].

## Navigation

- [[20-projects/{co}/index|Company index]]
- [[20-projects/{co}/shared/patterns|Shared patterns]]
- [[20-projects/{co}/shared/gotchas|Shared gotchas]]
- [[20-projects/{co}/shared/glossary|Glossary]]
- [[20-projects/{co}/shared/cross-product-lessons|Cross-product lessons]]
- [[20-projects/{co}/shared/decisions-not-adr|Decisions not ADR]]
`

const tplCompanyLog = `# {co} memory log

Append-only chronological log of curator events (drains, syncs, lints, promotions).
`

const tplCompanyHealth = `# {co} wiki health

Last lint: never

Findings will be appended here by ` + "`wiki-lint`" + `.
`

const tplCompanyPrivacy = `# {co} privacy boundary

This directory is company-scoped. Do not copy, summarize, or expose its contents into another company namespace.

Do not store:
- Customer-identifying data
- Production secrets / tokens / credentials
- Raw production logs (sanitized summaries OK)
- Cross-company information

Workers (project agents) must stay within their company's namespace. Curator must refuse cross-company synthesis.
`

const tplCompanyPatterns = `# {co} shared patterns

Cross-product patterns within {co}. Promote here from product/shared/patterns.md when a pattern recurs in 2+ products.
`

const tplCompanyGotchas = `# {co} shared gotchas

Cross-product gotchas within {co}.
`

const tplCompanyGlossary = `# {co} glossary

Domain-specific terms used across products in {co}.
`

const tplCompanyCrossLessons = `# {co} cross-product lessons

Lessons that apply across multiple products in {co}.
`

const tplCompanyDecisions = `# {co} decisions not yet promoted to ADR

Informal company-wide decisions that haven't crystallized into ADRs yet.
`

const tplCompanyPlaybookDebug = `# {co} debugging playbook

Cross-product debugging procedures.
`

const tplCompanyPlaybookRelease = `# {co} release playbook

Cross-product release procedures.
`

const tplCompanyPlaybookMigration = `# {co} migration playbook

Cross-product migration procedures.
`

const tplProductIndex = `# {co}/{prod} memory index

Purpose:
- Cross-repo memory for the {prod} product.
- Start here before debugging/designing recurring {prod}-product problems.

## Repos

<!-- auto-managed list — entries added by wiki-bootstrap-product -->

## Shared product memory

- [[shared/patterns]]
- [[shared/gotchas]]
- [[shared/interfaces]]
- [[shared/integration-points]]
- [[shared/debugging-playbook]]
`

const tplProductPage = `# {prod}

Product-level memory namespace for ` + "`{co}/{prod}`" + `.

Use this page as the human-readable graph node for the product. The canonical agent index is [[20-projects/{co}/{prod}/index|{co}/{prod} index]].

## Navigation

- [[20-projects/{co}/{co}|Company hub]]
- [[20-projects/{co}/{prod}/index|Product index]]
- [[20-projects/{co}/{prod}/shared/patterns|Shared patterns]]
- [[20-projects/{co}/{prod}/shared/gotchas|Shared gotchas]]
- [[20-projects/{co}/{prod}/shared/interfaces|Interfaces]]
- [[20-projects/{co}/{prod}/shared/integration-points|Integration points]]
- [[20-projects/{co}/{prod}/shared/debugging-playbook|Debugging playbook]]
- [[20-projects/{co}/{prod}/open-questions|Product open questions]]
`

const tplProductLog = `# {co}/{prod} memory log

Append-only chronological log.
`

const tplProductHealth = `# {co}/{prod} wiki health

Last lint: never
`

const tplProductOpenQuestions = `# {co}/{prod} open questions

Product-level uncertainties. Reviewed before sprint planning.
`

const tplProductPatterns = `# {co}/{prod} shared patterns

Patterns recurring in 2+ repos of {prod}. Promoted here by curator via wiki-synthesize.
`

const tplProductGotchas = `# {co}/{prod} shared gotchas

Counter-intuitive things that bite across {prod} repos.
`

const tplProductInterfaces = `# {co}/{prod} interfaces

Stable contracts between repos in {prod}.
`

const tplProductIntegrations = `# {co}/{prod} integration points

Where {prod} repos talk to each other and to external systems.
`

const tplProductDecisions = `# {co}/{prod} decisions not yet promoted to ADR

Product-level informal decisions.
`

const tplProductPlaybook = `# {co}/{prod} debugging playbook

Step-by-step procedures for known {prod} debugging scenarios.
`

const tplProductSynthesis = `# {co}/{prod} synthesis candidates

Queue of lessons flagged by inbox-drain as candidates for cross-repo pattern synthesis. Processed by wiki-synthesize.
`

const tplRepoIndex = `# {co}/{prod}/{repo} memory index

Canonical repo docs (read FIRST for facts):
- repo design: ` + "`{repoPath}/docs/design.md`" + `
- repo AGENTS: ` + "`{repoPath}/AGENTS.md`" + `
- ADRs: ` + "`{repoPath}/docs/adr/`" + `

## Recent memory

<!-- auto-managed by inbox-drain -->

## Memory pages

- [[lessons]]
- [[debugging-stories]]
- [[gotchas]]
- [[decisions-not-adr]]
- [[open-questions]]
- [[links]]
`

const tplRepoPage = `# {repo}

Scope: ` + "`{co}/{prod}/{repo}`" + `.

Use this page as the human-readable graph node for the repo. Agent-facing memory stays in the stable files below.

## Memory

- [[20-projects/{co}/{prod}/repos/{repo}/hot|Hot context]]
- [[20-projects/{co}/{prod}/repos/{repo}/lessons|Lessons]]
- [[20-projects/{co}/{prod}/repos/{repo}/gotchas|Gotchas]]
- [[20-projects/{co}/{prod}/repos/{repo}/debugging-stories|Debugging stories]]
- [[20-projects/{co}/{prod}/repos/{repo}/decisions-not-adr|Decisions not ADR]]
- [[20-projects/{co}/{prod}/repos/{repo}/open-questions|Open questions]]
- [[20-projects/{co}/{prod}/repos/{repo}/links|Links]]
- [[20-projects/{co}/{prod}/repos/{repo}/_inbox|Inbox]]

## Parent context

- Company: [[20-projects/{co}/{co}|{co}]]
- Product: [[20-projects/{co}/{prod}/{prod}|{co}/{prod}]]
`

const tplRepoInbox = `# {repo} memory inbox

Append-only raw captures by Worker agent. Drained into curated files by Curator (wiki-sync command).

Entries below this line. Marked ` + "`Status: drained`" + ` after curation; ` + "`Status: rejected`" + ` if low-signal.

---
`

const tplRepoHot = `# {repo} hot context

Auto-refreshed by curator. Loaded by SessionStart hook on every codex/claude session in this repo.

Keep this file small (~50 lines). Include only:
- 2-3 most recent durable lessons
- Currently active open questions
- Critical gotchas relevant right now

<!-- last refreshed: never -->
`

const tplRepoLessons = `# {repo} lessons

Durable validated lessons. Promoted from _inbox.md by Curator.

Each entry format: see ` + "`../../../_templates/lesson-entry.md`" + `.
`

const tplRepoDebugging = `# {repo} debugging stories

Narratives of complex debug sessions — symptoms, hypotheses tried, real root cause, fix, lesson.

Each entry format: see ` + "`../../../_templates/debugging-story.md`" + `.
`

const tplRepoGotchas = `# {repo} gotchas

Counter-intuitive things that bite in this codebase. Failed approaches worth not repeating.
`

const tplRepoDecisions = `# {repo} decisions not yet promoted to ADR

Informal decisions in this repo. If a decision becomes binding → promote to docs/adr/ in the actual repo.
`

const tplRepoOpenQuestions = `# {repo} open questions

Unresolved questions specific to this repo.

Each entry format: see ` + "`../../../_templates/open-question.md`" + `.
`

const tplRepoLinks = `# {repo} memory links

Canonical sources to consult:
- design: ` + "`{repoPath}/docs/design.md`" + `
- AGENTS: ` + "`{repoPath}/AGENTS.md`" + `
- ADRs: ` + "`{repoPath}/docs/adr/`" + `
- README: ` + "`{repoPath}/README.md`" + `

External:
`
