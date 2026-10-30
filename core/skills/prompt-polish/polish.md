---
target: skill/prompt-polish
rules: [workflow.step0-idea-refine, workflow.step0-detect-category, workflow.step0-prompt-template-file, workflow.polish.scope-paths, workflow.polish.data-flow, workflow.polish.tdd-tests, workflow.polish.branch-proposals, workflow.polish.leak-preaudit, workflow.polish.doc-targets, workflow.polish.plan-destination, workflow.polish.pr-destination, workflow.step0-user-confirms-once]
---
# Prompt polish (step 0)

Turn the request into the block below, print it in chat, and wait for the user to confirm it before `plan` runs. If the project keeps its own prompt template, fill that one; it wins over this one.

## Refine before writing

- Who is this for, and what core problem does it solve?
- Which assumptions hide in the request? Name each one.
- Write an explicit Not Doing list so scope cannot creep. Use the `idea-refine` skill when it is installed.

## Category

- `feat`: new feature, screen slicing, redesign, API integration, new user flow.
- `fix`: functional bug, wrong state or calculation, UI issue that does not crash.
- `hotfix`: critical production crash or blocking exception.

## Block

```text
CATEGORY:         feat | fix | hotfix
GOAL:             <who it is for> - <core problem>
NOT DOING:        <explicit exclusions>
ASSUMPTIONS:      <hidden assumptions surfaced>
TIER:             trivial | small | standard | large - <evidence>
SCOPE:            <exact directories and files>
DATA FLOW:        <layers touched: request -> client -> response DTO -> mapper -> model -> repository -> state holder -> UI>
DATA-LAYER TESTS: <test files and cases to write first, in test/>
DISPOSAL TARGETS: <controllers, focus nodes, subscriptions, timers, tickers, image cache widths>
DARTDOC TARGETS:  <units that will need Dartdoc>
BRANCH:           1) <name>  2) <name>  3) <name>  - base branch? <ask>
PLAN:             {{paths.plans}}/<task-slug>/plan.md (strategy) + code.md (code changes)
PR DESCRIPTION:   {{paths.pr}}/<branch-name>.md
```

Installed packs and workflows may add lines below the block (roster, QA identifiers, analytics question).

## Confirmation

The user confirms scope, Not Doing, roster, tier and the analytics decision once, on this block. Add no further confirmation point.
