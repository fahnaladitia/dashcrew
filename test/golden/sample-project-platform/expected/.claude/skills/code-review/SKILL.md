---
name: code-review
description: "Run the read-only reviewer on the current change against the project rules, the success criteria and the active review checklists, then fix by severity."
argument-hint: "[base branch]"
---

# code-review

The main session runs the `reviewer` subagent on the current change and acts on its verdict. The `reviewer` writes nothing.

## Inputs to the reviewer

Pass paths and facts, never copied file content:

- the base branch chosen when the branch was created; never guess it
- the step 0 success criteria, and the plan path when there is one
- the skills on the step 0 `Skills:` line that the skill index marks `review`
- for a re-review: the rework diff only

## What the reviewer does

- Starts reviewing right away, without asking for clarification.
- Covers the committed work on the branch plus the uncommitted working tree:

```bash
git diff --stat <base>...HEAD
git diff <base>...HEAD
git diff --stat
git diff
git status --porcelain
```

- Reads every changed file in full, not only the hunks, and untracked new files too. Context outside the hunk often decides whether the change is right.
- Reviews against AGENTS.md, the priorities below, and the checklist of every active review skill.
- Spec: for each criterion, met, partial or not met, with the `file:line` that proves it and the covering test, or its manual checklist item. A criterion with neither is HIGH.
- Scope: files changed outside the plan or inside its "not doing" list; a branch that tracks its base (reported, never fixed); every `Not in design` item recorded for the PR notes; every open question raised to the user.
- States the bug, shows the fix, stops. No suggestion outside the scope of the review, no compliments.

When a code graph tool is connected, it measures the impact radius before reading:

- list the changed functions with their risk score
- find callers outside the diff that are affected, and read the risky ones
- look up the tests covering each high-risk function; a risky function with no test is HIGH

## After the verdict

- Fix every BLOCKER and HIGH, then run the `reviewer` again on the rework diff only.
- MEDIUM and LOW go to the PR notes; ask the user about them once, as one list.
- A second BLOCK goes to the user as a decision, never into a third round.

## Priorities

Review in this order: correctness, lifecycle and async; architecture boundaries; test coverage; QA identifiers; performance.

**BLOCKER**

- UI or state touched after an `await` once its owner may be gone, without a guard
- a subscription, listener, timer, connection or state holder that is never closed or cancelled
- a response DTO reaching the UI without going through a mapper
- a hardcoded credential
- a forced unwrap or cast of a value not proven non-null
- an untyped value in new code where the language allows a type

**HIGH**

- a new mapper, repository, model or state holder without a test
- business logic in a UI component instead of the state holder
- a QA-relevant element without its identifier, or an identifier built from a dynamic value (list index, hash code, timestamp)
- an identifier removed or renamed without proof that no automation suite uses it

**MEDIUM**

- a large part of the UI re-rendering where a narrower scope would do
- debug output instead of the project logger
- a function over 100 lines
- a magic number without a named constant

Selected packs add stack-specific items right after this list.

## Documentation

- Every unit the documentation rules require must carry a doc comment.
- Doc comments are written in `en` and are at most 3 lines.
- A missing doc comment, one over the line cap, and one that restates the logic are findings.

## Rejected automatically

These are process violations, reported without discussion:

- changed lines unrelated to the task (reformatting, whitespace, quote style)
- comments that explain what the code does
- a doc comment over 3 lines or not in `en`
- an abstraction with a single caller
- error handling for a case that cannot happen

## Finding format

Every finding has a severity, `file:line`, the current code and the fixed code.
A finding without an address is not a finding.

```
[BLOCKER] Order total ignores the discount
<data>/mappers/order_mapper:42

  total = response.subtotal

Fix:

  total = response.subtotal - (response.discount or 0)
```

## Verdict

```
REVIEW: <plan | n files>

BLOCKER  n
HIGH     n
MEDIUM   n
LOW      n

Criteria
  AC-1  met      <file:line>   <test file:line | manual item>
  AC-2  partial  <what is missing>

Verdict: [BLOCK / PASS WITH NOTES / PASS]
```

- LOW is a finding that changes neither behavior nor risk (naming, a small duplication).
- BLOCK when there is any BLOCKER or HIGH.
- PASS WITH NOTES when there are only MEDIUM or LOW findings.
- PASS otherwise.
