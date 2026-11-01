---
target: skill/prompt-polish
rules: [workflow.step0-idea-refine, workflow.step0-detect-category, workflow.step0-prompt-template-file, workflow.polish.scope-paths, workflow.polish.branch-proposals, workflow.step0-user-confirms-once, step0.questions-once, step0.skills-from-index, step0.skill-exclusions, step0.criteria-mapped]
---
# Prompt polish (step 0)

Turn the request into the step 0 block in AGENTS.md, print it in chat, and wait for the user to confirm it. If the project keeps its own prompt template, fill that one; it wins over this one.

## Refine before writing

- Who is this for, and what core problem does it solve?
- Which assumptions hide in the request? Name each one; when the request reads two ways, name both.
- Keep scope tight: what is left out goes into the TASK line as "not doing". Use the `idea-refine` skill when it is installed.

## Lines

- `TASK`: the category, then the request in one or two sentences with the exact directories and files in scope.
  - `feat`: new feature, screen slicing, redesign, API integration, new user flow.
  - `fix`: functional bug, wrong state or calculation, UI issue that does not crash.
  - `hotfix`: critical production crash or blocking exception.
- `Tier`: per the size tier table in AGENTS.md, with evidence.
- `Questions`: every question at once, including the ones packs add (analytics, identifiers). Never ask a second round later.
- `Skills`: from the skill index in AGENTS.md, each with the reason it applies. No skill that conflicts with the project's choices:
  - architecture skills that impose a state-management pattern the project does not use
  - test skills that require a mocking library the project does not use
  - long-form documentation writers that break the doc comment line cap
  - mass lint or format auto-fix skills
  - setup skills for a client, router or analytics product the project does not use
- `Success criteria`: when the task has none, apply `acceptance-criteria`. Map each criterion to the test type that proves it, or `manual`.
- `Branch`: 3 names per the branching rules, and the base question.

## Confirmation

The user confirms the whole block once. Add no further confirmation point before the plan.
