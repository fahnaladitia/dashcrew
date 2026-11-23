---
target: workflow/verify
order: 20
rules:
  - verify.analyze-first-errors-zero
  - verify.warnings-nonblocking
  - verify.format-check-read-only
---
## Phase 1: Analyze (STOP on failure)

```bash
flutter analyze <files and folders from the scope> 2>&1 | tail -30
```

- Analyze runs first. Errors must be zero; an analyze failure stops verify.
- Report each issue as `file:line - message`.
- Warnings are reported but do not block.
- Fix issues by hand, only on lines that belong to the task.

Then check formatting without writing, on the Dart files in scope except generated ones:

```bash
dart format --output=none --set-exit-if-changed <Dart files from the scope, without *.g.dart and *.freezed.dart>
```

- It writes nothing: it lists each file whose layout differs and exits 1. Verify does not pass while a task line fails it.
- When the guard blocks this check, the installed dashcrew is older than the check: report it and do not count formatting as checked.
