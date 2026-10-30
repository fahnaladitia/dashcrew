---
target: workflow/code-review
order: 10
rules:
  - review.diff-against-rules
  - review.start-without-clarification
  - review.read-whole-files
  - review.impact-radius-graph
  - review.state-bug-fix-stop
  - review.no-out-of-scope-suggestions
  - review.no-compliments
---
# /code-review

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
