---
name: ralph
description: "Use when the user says Ralph, ralph-loop, ральф, asks to iteratively fill epics/features, or wants a bounded self-checking planning loop with acceptance criteria and verification."
---

# Ralph

Use this skill when the user wants an iterative planning loop for product,
engineering, documentation, or implementation work. Typical triggers:

- `ralph`
- `ralph-loop`
- `ральф`
- "прогони через ральфа"
- "итеративно распиши фичи"
- "заполни эпики с проверкой"

This is a Codex adaptation of the Ralph loop idea. Codex does not use Claude
Code stop hooks, so do not claim an automatic hidden infinite loop exists.
Instead, run a bounded visible loop inside the chat: inspect, improve, check,
report, and continue until the agreed promise is true or the iteration budget is
exhausted.

## Inputs

If the user did not provide enough context, ask for only the minimum:

- target file/folder or topic;
- desired output shape;
- max iterations if the task is large;
- completion promise if the user wants one.

Use sensible defaults when safe:

- max iterations: 3 for planning/docs, 5 for code work;
- completion promise: `RALPH_DONE`;
- output shape for feature work: epic -> feature -> plan -> acceptance -> checks.

## Workflow

1. Restate the current objective and completion promise in one short paragraph.
2. Build or read the current work artifact.
3. Run one iteration:
   - inspect current state;
   - identify gaps;
   - patch or write the next improvement;
   - run the smallest relevant check;
   - summarize what improved and what remains.
4. Decide:
   - if completion criteria are true, output the promise exactly once;
   - if blocked, explain the blocker and the last useful partial result;
   - otherwise continue until max iterations.
5. For planning work, every iteration should improve one of:
   - scope clarity;
   - feature breakdown;
   - acceptance criteria;
   - verification checks;
   - dependency/risk sequencing.

## Feature Planning Template

For each epic, produce features in this shape:

```markdown
### FNN: Feature Name

Plan:

1. First implementation/design step.
2. Second implementation/design step.
3. Verification or rollout step.

Acceptance:

- Observable outcome that proves the feature works.
- Boundary or safety condition.
- Failure mode is explicit.

Checks:

- Smallest relevant test/check.
- Manual review check if automation is not yet possible.
```

## Self-Check Rubric

Before declaring completion, verify:

- every epic has at least one concrete feature;
- each feature has plan, acceptance, and checks;
- V0 work is separated from later/V1/V2 work;
- dependencies are obvious or documented;
- risky/staging/production actions are gated explicitly;
- artifacts and reports are named;
- no raw secrets or production data were introduced.

## Rules

- Do not output the completion promise unless the objective is actually true.
- Keep loops bounded; never spin indefinitely.
- Prefer editing files over only describing changes when the user asked to
  produce an artifact.
- Run the smallest relevant check after edits.
- If the task needs human judgment, pause with clear options instead of
  pretending the loop can decide.
- Do not add Claude-specific hook instructions to Codex docs unless the user
  explicitly asks for Claude Code setup.

## Rules

- <rule>
