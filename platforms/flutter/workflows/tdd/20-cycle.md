---
target: workflow/tdd
order: 20
rules:
  - tdd.red-fails-on-assertion
  - tdd.green-minimal
  - tdd.refactor-only-green
  - tdd.one-test-one-behavior
---
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
