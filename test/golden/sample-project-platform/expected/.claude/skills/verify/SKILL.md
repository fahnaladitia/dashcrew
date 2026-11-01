---
name: verify
description: "Run the scoped quality gate - analyze, tests, secrets, architecture, QA identifiers, leaks and diff - and report PASS or FAIL backed by real output."
argument-hint: "quick | full | pre-pr | codegen"
---

# verify

Run the phases in order. Stop at a phase marked STOP when it fails.

## Modes

- `quick`: phase 1 only.
- `full` (default): phases 1 to 5.
- `pre-pr`: phases 1 to 6.
- `codegen`: run the project's code generation step first, then `full`.

## Skills

- Apply `verification-loop`.

## Scope: the diff, not the repo

Every phase runs on the files this branch changed.
Run the whole repo only when the user asks or when cutting a release.

Determine the scope once, from the base branch chosen when the branch was created:

```bash
git diff --name-only <base>...HEAD
git status --porcelain | awk '{print $2}'
```

- The analyze list is both outputs combined, source and test files only: tracked changes against the base plus untracked files.
- The test list is every test file in that list, plus every test that imports a changed unit (`grep -rl "<file_name_without_ext>" <test folder>`).

## Phase 1: Analyze (STOP on failure)

```bash
go vet <files and folders from the scope> 2>&1 | tail -30
```

- Analyze runs first. Errors must be zero; an analyze failure stops verify.
- Report each issue as `file:line - message`.
- Warnings are reported but do not block.
- Fix issues by hand, only on lines that belong to the task.

## Phase 2: Tests

Run every test file in scope in one call; each call pays the test runner's startup cost.

```bash
go test <test files from the scope> 2>&1 | tail -40
```

- Report passed and failed counts, and name the scope.
- A scoped run says nothing about the rest of the suite; never claim "all tests green".
- When a file fails, rerun that file alone for the full output.
- When a test fails, check once whether it also fails on the base, without touching this working tree: run that test file alone in a detached worktree of the base (`git worktree add --detach <scratch dir> <base>`, then `go mod download` there; remove it afterwards with `git worktree remove`). Failing there too, it is reported as pre-existing, not as a regression. Never stash, and never check out the base here.
- Every mapper, repository, model and state holder the task touched must have a test. A missing test is FAIL, not a note.
- When the change has no tests at all, trace it in chat instead: files and lines, the logic path before and after, and side effects.
- Hand the user the manual checklist from step 0 in one message, and record each result for the PR Testing section.

## Phase 3: Secret scan

```bash
grep -rn "sk-\|api_key\|apiKey *=\|secret *=\|password *=\|Bearer " <source folders> | head -20
```

DTO field names and constant keys are not findings. Look for literals that hold real values.

## Phase 4: Architecture and leaks

Selected packs add their own boundary checks to this phase.

Check by reading, on the changed files:

- a response DTO used in the UI without a mapper
- debug output left in app code instead of the project logger
- every resource the change opens (subscription, listener, timer, connection, file handle) is closed or cancelled

## Phase 5: QA identifiers

For each screen this task sliced or redesigned, every QA-relevant element carries its identifier as AGENTS.md defines it.
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

Never run a formatter or an auto-fixer to clear analyzer or diff output.

## Output

```
VERIFY: [PASS/FAIL]

Scope:    [N files analyzed, M test files]
Analyze:  [OK / N errors, M warnings]
Tests:    [X/Y passed (scoped)]
Manual:   [N items handed to the user / none]
Secrets:  [OK / N findings]
Arch:     [OK / N violations]
QA ids:   [OK / N elements without identifier / N/A]
Diff:     [N files, M lines out of scope]

PR ready: [YES/NO]
```

- On NO, list every issue with `file:line` and a concrete fix.
- Paste the real command output under the summary. Never conclude PASS without it.
