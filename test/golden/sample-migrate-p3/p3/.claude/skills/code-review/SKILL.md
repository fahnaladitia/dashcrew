---
name: code-review
description: "Review the current change against architecture, lifecycle, security and project rules, stating each bug with its fix and ending in a verdict."
argument-hint: "[base branch]"
---

# code-review

Runs the `flutter-code-reviewer` role on the current change.
Review the change against clean architecture, security and the project rules in AGENTS.md.

## Style

- Start reviewing right away; do not ask for clarification first.
- State the bug, show the fix, stop.
- Make no suggestion outside the scope of the review.
- No compliments, before or after the findings.

## Steps

The change is the committed work on the branch plus the uncommitted working tree:

```bash
git diff --stat <base>...HEAD
git diff <base>...HEAD
git diff --stat
git diff
git status --porcelain
```

- Read every changed file in full, not only the hunks. Context outside the hunk often decides whether the change is right.
- Read untracked new files in full too.

When a code graph tool is connected, measure the impact radius before reading:

- list the changed functions with their risk score
- find callers outside the diff that are affected, and read the risky ones
- look up the tests covering each high-risk function; a risky function with no test is HIGH

## Priorities

Review in this order: correctness, lifecycle and async; architecture boundaries; test coverage; QA identifiers; performance.

**BLOCKER**

- `BuildContext` used after an `await` without a `mounted` guard
- a controller, focus node, stream subscription or state holder that is never disposed
- a response DTO reaching the UI without going through a mapper
- a hardcoded credential
- `!` on a value not proven non-null
- `dynamic` in new code

**HIGH**

- a new mapper, repository, model or state holder without a test
- business logic in a widget instead of the state holder
- a QA-relevant widget without its identifier, or an identifier built from a dynamic value (list index, hashCode, timestamp)

**MEDIUM**

- a missing `const` constructor
- a large subtree rebuilding where a narrower rebuild scope would do
- `print(` instead of the project logger
- a widget `build` method over 100 lines
- a magic number without a named constant

Selected packs add stack-specific items right after this list.

## Documentation

- Every unit the documentation rules require must carry Dartdoc.
- Dartdoc is written in `en` and is at most 3 lines.
- Missing Dartdoc, Dartdoc over the line cap, and Dartdoc that restates the logic are findings.

## Rejected automatically

These are process violations, reported without discussion:

- changed lines unrelated to the task (reformatting, whitespace, quote style)
- comments that explain what the code does
- Dartdoc over 3 lines or not in `en`
- an abstraction with a single caller
- error handling for a case that cannot happen

## Finding format

Every finding has a severity, `file:line`, the current code and the fixed code.
A finding without an address is not a finding.

```
[BLOCKER] BuildContext used across an async gap
lib/ui/orders/order_page.dart:88

  await repository.submit();
  Navigator.of(context).pop();

Fix:

  await repository.submit();
  if (!context.mounted) return;
  Navigator.of(context).pop();
```

## Verdict

```
REVIEW: <n> files

BLOCKER  n
HIGH     n
MEDIUM   n

Verdict: [BLOCK / PASS WITH NOTES / PASS]
```

- BLOCK when there is any BLOCKER or HIGH.
- PASS WITH NOTES when there are only MEDIUM findings.
- PASS otherwise.
