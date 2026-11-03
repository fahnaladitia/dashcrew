---
name: prompt-polish
description: "Turn a raw request into the confirmed step 0 block before any plan or code."
when_to_use: "At the start of every task."
---

# Prompt polish (step 0)

Turn the request into the step 0 block below, print it in chat, and wait for the user to confirm it. If the project keeps its own prompt template, fill that one; it wins over this one.

## Refine before writing

- Who is this for, and what core problem does it solve?
- Which assumptions hide in the request? Name each one; when the request reads two ways, name both.
- Keep scope tight: what is left out goes on the `Not doing` row. Use the `idea-refine` skill when it is installed.

## Block

```text
STEP 0  <feat | fix | hotfix>: <polished request, one sentence>
========================================================================
Scope      <directories and files>
Not doing  <what stays out | none>
Tier       <trivial | small | standard | large>  (<evidence>)

-- Assumptions ---------------------------------------------------------
A1    <assumption>

-- Questions -----------------------------------------------------------
Q1    <question>

-- Skills --------------------------------------------------------------
NAME                 WHY
<name>               <why>

-- Success criteria ----------------------------------------------------
ID    CRITERION                                 TEST
<id>  <criterion>                               <test type>

-- Branch --------------------------------------------------------------
1     <name>
2     <name>
3     <name>
base  <ask>
========================================================================
Reply "ok", or answer by id (Q1 ...), pick a branch and a base, edit any line.
```

An empty list prints `none` under its rule. Columns line up with spaces; a long value wraps under its own column.

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
  - long-form documentation writers that break the Dartdoc line cap
  - mass lint or format auto-fix skills
  - setup skills for an HTTP client, router or analytics product the project does not use
- `Success criteria`: when the task has none, apply `acceptance-criteria`. Map each criterion to the test type that proves it: `unit`, `widget`, `integration` or `manual`.
- `Branch`: 3 names per the branching rules, and the base question.

## Confirmation

Right after the block, ask once and wait. With the agent's question tool, ask through it: each `Q` by its id, the branch and base, then the confirmation; the `Reply` line is then left out. Without one, the `Reply` line asks. The user confirms the whole block once. Add no further confirmation point before the plan.
