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

Run the cycle per plan stage (mapper, state, UI), not per test.

### 1. RED

- Write one test per behavior, named after the behavior, not after the method.
- Write all RED tests of the stage, then run them in one call:

```bash
{{platform.commands.test_scoped}} <the stage's test files>
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
{{platform.commands.test_scoped}} <every test file you touched>
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
