---
target: workflow/code-review
order: 10
rules:
  - review.runs-reviewer
  - review.reviewer-inputs
  - review.diff-against-rules
  - review.start-without-clarification
  - review.read-whole-files
  - review.impact-radius-graph
  - review.state-bug-fix-stop
  - review.no-out-of-scope-suggestions
  - review.no-compliments
  - review.spec-axis
  - review.scope-checks
  - review.after-verdict
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
