---
name: tdd
description: "Implement data-layer logic test first - RED on an assertion, minimal GREEN, refactor only while green - then the UI."
---

Arguments: `<plan stage or unit to implement>`

# tdd

Apply `test-driven-development` and `data-layer-tdd`. The test cases each unit type needs are in `data-layer-tdd`.

## Scope

- Write data-layer logic test first: mappers, repositories, models, and state holders (blocs, cubits, notifiers).
- Implement in this order: mapper, model, repository, state holder, then UI.
- Start the UI only after the data-layer tests pass.
- Skip UI and widget tests unless the user or the plan asks for them. The gap stays visible in the verification map.
- Read every file in full before changing it.

## Implementing

The main session implements. Work the plan lists gets done; anything else does not.

Before writing a line, read in order: the plan (standard and large), its Architecture and Design sections when present, and every file you will change, in full.

- UI from the design spec, not from memory: the listed tokens and components, verbatim copy, no hardcoded server-driven text, identifiers from the plan, bounded image caches in lists, and every resource released.
- Doc comments in the same edit as the code, per AGENTS.md.
- A decision in the plan you cannot follow is raised to the user, never changed silently; the PR notes record it.
- Wire analytics only after the user's decision at step 0.

## Cycle

Run the cycle per plan stage (mapper, state, widget), not per test.

### 1. RED

- Write one test per behavior, named after the behavior, not after the method.
- Write all RED tests of the stage, then run them in one call:

```bash
flutter test test/order_mapper_test.dart --reporter expanded
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

- Aim for at most 6 `flutter test` runs per task. Each run recompiles, which costs 10 to 20 seconds.
- At the end, run every test file you touched in one call:

```bash
flutter test test/order_mapper_test.dart test/order_repository_test.dart --reporter expanded
```

- Never run the whole suite here. Regressions outside the scope are caught by `verify` on the diff.

## Output

```
TDD: <subject>

RED    test/<name>_test.dart:<line>  <test name>
GREEN  lib/<path>/<file>.dart
REFAC  <what was tidied, or none>

flutter test: X/Y passed
```

Paste the real `flutter test` output. "It should pass" is not evidence.
