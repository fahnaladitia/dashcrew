---
target: workflow/verify
order: 20
rules:
  - verify.analyze-first-errors-zero
  - verify.warnings-nonblocking
  - verify.test-evidence
  - verify.trace-when-no-tests
  - verify.leak-audit
  - verify.diff-review
---
## Phase 1: Analyze (STOP on failure)

```bash
flutter analyze <files and folders from the scope> 2>&1 | tail -30
```

- Analyze runs first. Errors must be zero; an analyze failure stops verify.
- Report each issue as `file:line - message`.
- Warnings are reported but do not block.
- Fix issues by hand, only on lines that belong to the task.

## Phase 2: Tests

Run every test file in scope in one call; each call pays a 10 to 20 second compile.

```bash
flutter test test/<a>_test.dart test/<b>_test.dart --reporter expanded 2>&1 | tail -40
```

- Report passed and failed counts, and name the scope.
- A scoped run says nothing about the rest of the suite; never claim "all tests green".
- When a file fails, rerun that file alone for the full output.
- Every mapper, repository, model and state holder the task touched must have a test. A missing test is FAIL, not a note.
- When the change has no tests at all, trace it in chat instead: files and lines, the logic path before and after, and side effects.

## Phase 3: Secret scan

```bash
grep -rn "sk-\|api_key\|apiKey *=\|secret *=\|password *=\|Bearer " lib/ --include="*.dart" | grep -v "\.g\.dart" | head -20
```

DTO field names and constant keys are not findings. Look for literals that hold real values.

## Phase 4: Architecture and leaks

```bash
grep -rn "print(" lib/ --include="*.dart" | grep -v "debugPrint" | head -20
```

Selected packs add their own boundary checks to this phase.

Check by reading, on the changed files:

- a response DTO used in the UI without a mapper
- `BuildContext` used after an `await` without a `mounted` guard
- every controller, focus node, timer, animation ticker and stream subscription is disposed or cancelled
- network images in lists and grids set a cache bound (`cacheWidth`, `cacheHeight` or `memCacheWidth`)

## Phase 5: QA identifiers

For each screen this task sliced or redesigned, every QA-relevant widget carries its identifier as AGENTS.md defines it.
Write N/A when no screen was sliced or redesigned.

## Phase 6: Diff review

```bash
git diff --stat <base>...HEAD
git diff --stat
```

Read each changed file. Look for:

- lines changed outside the task (indentation, spacing, style); revert them
- missing error handling
- edge cases not handled
