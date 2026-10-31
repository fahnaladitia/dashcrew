---
target: skill/verification-loop
order: 10
rules:
  - verify.when-to-run
  - verify.evidence-not-claims
---
# Verification loop

Show evidence, never a claim. "It should work" is not evidence.

## When

- after finishing a feature or a large change
- before opening a PR
- after a refactor
- at phase boundaries in `orchestrate`

## How

Run the phases of the `verify` workflow on the diff, never on the whole repo:

1. Analyze the changed files. Errors must be zero; stop and fix on failure.
2. Run the covering test files in one call. Every touched mapper, repository, model and state holder has a test, or the gate fails.
3. Scan for secrets in literals.
4. Check architecture boundaries and leaks on the changed files.
5. Check QA identifiers on sliced or redesigned screens.
6. Review the diff for out-of-scope lines, missing error handling and unhandled edge cases.

## Evidence

- Behind every PASS and every "fixed", paste the real command output.
- When no test covers the change, trace it instead: files and lines, the logic path before and after, and side effects.
- A summary with no output behind it is a claim, not a verification.

## Why not on every edit

A full analyze and a full test run take minutes on a real app. Running them on every edit makes the session unusable.
Cheap pattern checks belong in hooks or guards; this loop runs at the points you choose.
