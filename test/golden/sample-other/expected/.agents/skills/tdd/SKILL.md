---
name: tdd
description: "Implement data-layer logic test first - RED on an assertion, minimal GREEN, refactor only while green - then the UI."
---

Arguments: `<plan stage or unit to implement>`

# tdd

Apply `test-driven-development` and `data-layer-tdd`. The test cases each unit type needs are in `data-layer-tdd`.

## Scope

- Write data-layer logic test first: mappers, repositories, models, and state holders.
- Implement in this order: mapper, model, repository, state holder, then UI.
- Start the UI only after the data-layer tests pass.
- Skip UI tests unless the user or the plan asks for them. The gap stays visible in the verification map.
- Read every file in full before changing it.

## Implementing

The main session implements. Work the plan lists gets done; anything else does not.

Before writing a line, read in order: the plan (standard and large), its Architecture and Design sections when present, and every file you will change, in full.

- UI from the design spec, not from memory: the listed tokens and components, verbatim copy, no hardcoded server-driven text, identifiers from the plan, bounded image caches in lists, and every resource released.
- Doc comments in the same edit as the code, per AGENTS.md.
- A decision in the plan you cannot follow is raised to the user, never changed silently; the PR notes record it.
- Wire analytics only after the user's decision at step 0.

## Cycle

Run the cycle per plan stage (mapper, state, UI), not per test.

### 1. RED

- Write one test per behavior, named after the behavior, not after the method.
- Write all RED tests of the stage, then run them in one call:

```bash
./scripts/test.sh <the stage's test files>
```

- A valid RED fails on an assertion.
- A compile error or a missing import is not RED. Fix the structure first, then run again.

### 2. GREEN

- Write the simplest code that makes the tests pass, and nothing more.
- Run the stage's tests again in one call.

### 3. REFACTOR

- Refactor only while the tests are green.
- Touch only lines that belong to the task; do not tidy the file's style.

### Run budget

- Aim for at most 6 test runs per task. Each run pays the test runner's startup cost.
- At the end, run every test file you touched in one call:

```bash
./scripts/test.sh <every test file you touched>
```

- Never run the whole suite here. Regressions outside the scope are caught by `verify` on the diff.

## Output

```
TDD: <subject>

RED    <test file>:<line>  <test name>
GREEN  <source file>
REFAC  <what was tidied, or none>

tests: X/Y passed
```

Paste the real test output. "It should pass" is not evidence.
