---
name: skill-maintainer
description: "Create, update, split, install, validate, debug, or improve Codex/Claude skills in this dotfiles architecture. Use whenever the user wants to make a new skill, modify an existing skill, adapt a skill from another source, fix SKILL.md YAML/frontmatter/loading issues, or decide where a skill should live across daily/setup/wiki agent profiles."
---

# Skill Maintainer

Use this skill as the single entrypoint for creating and maintaining reusable
Codex/Claude skills in this dotfiles repo.

It combines the general skill-writing guide with the local platform rule:
skills are authored in dotfiles first, then installed into runtime profiles by
symlink.

## Source Of Truth

- Do not create reusable skills directly in `~/.codex-new/skills` or
  `~/.claude-new/skills`.
- Do not create reusable skills directly in `~/.codex-setup/skills`,
  `~/.claude-setup/skills`, or `~/.codex-wiki/skills`.
- Create or edit source skills in `~/dotfiles/skills/<skill-name>/SKILL.md`.
- Runtime profile directories should contain symlinks to dotfiles skills.
- Install and validate with `agent-skill install` and `agent-skill doctor`.

## What Skills Are

Skills are modular folders that extend an agent with specialized workflows,
tool usage, or domain knowledge. Treat them as compact onboarding guides for
repeatable work.

A skill can provide:

- specialized workflows for a domain or recurring task;
- tool integrations and deterministic scripts;
- domain-specific references, schemas, or policies;
- bundled assets or templates used during output generation.

Keep the skill focused. Do not turn it into a general README.

## Anatomy

```text
skill-name/
├── SKILL.md          # required
├── scripts/          # optional deterministic code
├── references/       # optional docs loaded only when needed
└── assets/           # optional files/templates used as output material
```

Do not add extra docs like `README.md`, `INSTALLATION_GUIDE.md`,
`CHANGELOG.md`, or quick references unless the user explicitly asks. The
skill folder should contain only what helps an agent perform the skill.

## Frontmatter

Every `SKILL.md` must start with valid YAML frontmatter:

```md
---
name: skill-name
description: "Use when the task involves explicit trigger words and workflow scope."
---
```

Rules:

- `name` must match the folder name.
- Always quote `description`.
- Put trigger logic in `description`, not only in the body.
- Make the description concrete and slightly pushy enough to trigger when
  useful.
- Avoid broad "use always" descriptions.
- If the description contains colon-like text such as `key: value`, quoting is
  mandatory or the skill may fail to load.

Good description shape:

```yaml
description: "Use when the user asks to <task>, mentions <trigger words>, or needs <workflow outcome>."
```

## Progressive Disclosure

Skills are loaded in layers:

1. Metadata: `name` and `description` are always visible.
2. `SKILL.md` body: loaded only after the skill triggers.
3. Bundled resources: loaded only when the body tells the agent to read or run
   them.

Keep `SKILL.md` lean. If it approaches ~500 lines or contains variant-specific
details, split details into `references/` and link them directly from
`SKILL.md`.

Use:

- `scripts/` for deterministic operations that are fragile or repeated;
- `references/` for schemas, policies, detailed examples, and provider-specific
  variants;
- `assets/` for templates, icons, fonts, fixtures, or other output resources.

Avoid deep reference chains. Reference files should be discoverable directly
from `SKILL.md`.

## Degrees Of Freedom

Choose how strict the skill should be:

- High freedom: natural-language guidance when several approaches are valid.
- Medium freedom: pseudocode/checklists when a preferred pattern exists.
- Low freedom: scripts and exact steps when consistency or safety matters.

Prefer the lowest complexity that keeps the workflow reliable.

## Writing Workflow

1. Capture intent:
   - What should the skill help the agent do?
   - When should it trigger?
   - What output should it produce?
   - What files, commands, or tools does it need?
   - What should it never do?
2. Decide the target:
   - Daily worker skill: useful during normal repo work.
   - Setup skill: useful for onboarding, analysis, template generation, or
     platform maintenance.
   - Wiki curator skill: useful only inside WikiPedik curator flows.
3. Create or edit the source:

   ```bash
   agent-skill new <name>
   ```

   Then edit `~/dotfiles/skills/<name>/SKILL.md`.

4. Keep the body procedural:
   - Start with when to use the skill.
   - List prerequisites.
   - Give the workflow.
   - Add rules and failure modes.
   - Point to specific references/scripts only when needed.
5. Install and validate:

   ```bash
   agent-skill install
   agent-skill doctor
   ```

6. Verify the runtime prompt if the profile matters:

   ```bash
   CODEX_HOME=~/.codex-new codex debug prompt-input smoke | rg '<skill-name>'
   CODEX_HOME=~/.codex-setup codex debug prompt-input smoke | rg '<skill-name>'
   CODEX_HOME=~/.codex-wiki codex debug prompt-input smoke | rg '<skill-name>'
   ```

## Local Profile Rules

This platform intentionally separates runtime profiles:

- Daily: `~/.codex-new`, `~/.claude-new`
  - normal development;
  - worker memory skills;
  - no setup-only analysis skills.
- Setup: `~/.codex-setup`, `~/.claude-setup`
  - onboarding and context generation;
  - product/repo analysis;
  - AGENTS.md generation.
- Wiki curator: `~/.codex-wiki`
  - WikiPedik curation and synthesis.

`skill-maintainer` is universal and should be installed everywhere. Most
skills are setup-only unless the user explicitly says they should be daily or
wiki skills.

If adding a new profile target, update `scripts/agent-skill.sh`,
`bootstrap.sh`, and docs together.

## Quality Bar

Before reporting done:

- `SKILL.md` has valid quoted YAML frontmatter.
- `name` equals folder name.
- Description states concrete triggers.
- Body is short enough and does not duplicate reference files.
- Scripts are executable if the skill depends on them.
- References are linked from `SKILL.md`.
- `agent-skill doctor` passes.
- For Codex, `codex debug prompt-input` confirms the skill appears in the
  intended profile and does not leak into unintended profiles.

## Evaluation

For simple procedural skills, a manual smoke test is enough:

- trigger phrase appears in prompt;
- the skill appears in the relevant profile;
- one small realistic task follows the workflow.

For fragile or important skills, add test prompts and evaluate whether the
skill changes behavior in the intended direction. Prefer raw evidence:
transcripts, output files, diffs, and command logs.

Do not claim a skill is production-ready just because the YAML parses.

## Workflow

1. Create/update `~/dotfiles/skills/<name>/SKILL.md`.
2. Run `agent-skill install`.
3. Run `agent-skill doctor`.
4. Report installed targets and any validation errors.

For a new skill, prefer:

```bash
agent-skill new <name>
```

Then edit the generated template.
