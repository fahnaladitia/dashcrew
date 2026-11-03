---
target: skill/prompt-polish
order: 20
rules: [step0.release, step0.spec, workflow.step0-detect-category, workflow.polish.scope-paths, workflow.polish.branch-proposals, workflow.step0-user-confirms-once, step0.questions-once, step0.skills-from-index, step0.skill-exclusions, step0.criteria-mapped]
---
## Lines

- `STEP 0`: the category, then the request in one sentence. `Scope` names the exact directories and files, plus what a pack asks step 0 to list (such as QA identifiers).
  - `feat`: new feature, screen slicing, redesign, API integration, new user flow.
  - `fix`: functional bug, wrong state or calculation, UI issue that does not crash.
  - `hotfix`: critical production crash or blocking exception.
- `Tier`: per the size tier table in AGENTS.md, with evidence.
- `Assumptions` and `Questions`: numbered `A1`, `Q1`, so the user answers by id.
- `Questions`: every question at once, including the ones packs add (analytics, identifiers). Never ask a second round later.
- `Skills`: from the skill index in AGENTS.md, each with the reason it applies. No skill that conflicts with the project's choices:
  - architecture skills that impose a state-management pattern the project does not use
  - test skills that require a mocking library the project does not use
  - long-form documentation writers that break the doc comment line cap
  - mass lint or format auto-fix skills
  - setup skills for a client, router or analytics product the project does not use
- `Success criteria`: when the task has none, apply `acceptance-criteria`. Map each criterion to the test type that proves it: `unit`, `ui`, `integration` or `manual`.
- `Branch`: 3 names per the branching rules, and the base question.
{{#has_release_lines}}
- `Release`: the version the task ships in, or `none`; ask it with the base. A fix on an older line is ported later with `port`.
{{/has_release_lines}}
{{#has_knowledge}}
- `Spec`: the spec the task builds, `specs/<release>/<feature>@rev<n>` with the current `rev` from `.ai/knowledge/specs/`, or `none`. With a spec, the success criteria are the ACs this task covers, by ID (`AC-2  <criterion>  <test type>`); never renumber or add one.
{{/has_knowledge}}

## Confirmation

Right after the block, ask once and wait. With the agent's question tool, ask through it: each `Q` by its id, the branch and base{{#has_release_lines}} and the release{{/has_release_lines}}, then the confirmation; the `Reply` line is then left out. Without one, the `Reply` line asks. The user confirms the whole block once. Add no further confirmation point before the plan.
