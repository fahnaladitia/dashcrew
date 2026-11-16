# P13c: lean execution, reviewer input, a cheaper explorer, quiet passing tests

Tier: standard. Neutral sources (`core/roles`, `core/workflows/code-review`, `core/rules/testing.md`, `platforms/flutter/platform.yaml`) and goldens for every sample; no Go code beyond tests (see Decisions). No profile key, no schema change.

Third of the four token-saving tasks (P13 usage, P13b budget gate, P13c this, P13d path-scoped Claude rules). `dashcrew usage` gives the before and after per role.

## Evidence (expense_tracker, `dashcrew usage --since 60d`)

| Role | Runs | Cache write | Cache read | Output |
|---|---|---|---|---|
| reviewer, session 29d60ea7 | 2 | 313.5k | 9.0M | 56.1k |
| reviewer, session fccf7522 | 3 | 342.2k | 6.4M | 54.2k |
| explorer | 0 | | | |

- All 5 reviewer runs were spawned as `general-purpose` with "You are the read-only `reviewer` in …/reviewer.md (read it)". The role's front matter (read-only tools, model, effort) did not apply, and each run first read the role file.
- The reviewer is told to run five git commands, then "read every changed file in full, not only the hunks". On a 10-file diff that is the whole of each file, every round, re-review included when the rework touches the same files.
- The explorer did not run, so its saving is small today; it matters for `plan` and `audit` sweeps.
- Tests: `test_scoped` is `flutter test --reporter expanded`, one line per test; verify keeps the last 40 lines (`| tail -40`) even when everything passes.

## 1. Reviewer gets the criteria and one diff, not every file

Taken literally, "the main session passes the diff" costs more: the main session would have to read the diff and write it out again as output, the priciest tokens, and the reviewer still reads it. What it passes instead, in `code-review` "Inputs to the reviewer":

- the base branch, the plan path, the review skills (as today);
- the step 0 success criteria written out in the prompt (a few lines), so the reviewer does not dig them out of the plan;
- what `verify` already established (analyze clean, test counts), so the reviewer never re-derives it;
- spawned as the `reviewer` agent type, never a general-purpose agent told to read the role file.

The reviewer, "On a diff" in `core/roles/reviewer/role.md` and the command block in `code-review`:

```bash
git diff --merge-base <base> --stat
git diff --merge-base <base> --function-context
git ls-files --others --exclude-standard
```

- One diff against the merge base covers committed and uncommitted work; `--function-context` shows each changed function whole.
- Untracked files are new: read in full.
- A changed file is opened in full only when the function context does not decide a finding (fields, constructor, imports, registration, a caller outside the hunk). "Read every changed file in full" goes.
- Re-review: the rework diff only (unchanged).

Unchanged on purpose: the reviewer still reads the diff from git, not a summary from the main session, so the review stays independent.

## 2. Explorer on a cheaper model

- New model tier `fast` in `role.yaml` (`high | standard | fast`).
  - Claude Code: `fast` -> `haiku`.
  - Codex: no model name is written (names change too often); the role's `effort` is written as today.
  - Antigravity: `model: inherit` as today; agy has no per-agent model.
- `core/roles/explorer/role.yaml`: `model: fast`, `effort: low`.
- Guard against a weaker model: the explorer output already requires `file:line` per claim and a "Not found" line; the main session reads the lines it acts on.

## 3. Tests: one line when they pass, everything when they fail

- `platforms/flutter/platform.yaml`: `test_scoped: flutter test --reporter failures-only` (checked: Flutter on this machine lists `failures-only`, "a separate line for failing tests, with no output for passing tests").
- `core/rules/testing.md`, one rule: run tests with the quietest output that still prints failures in full; a pass is reported as one line (counts and scope), a failure with the failing test's full output.
- `verify` phase 2 keeps `| tail -40` (a failure longer than that is rerun alone, as it says today).
- A project platform (`platform: other`) keeps its own `test_scoped`; the rule tells the agent what to aim for.

## Code

- `internal/adapters/claude_code.go`: `claudeModels` gains `"fast": "haiku"`.
- `internal/catalog`: refuse a `model` outside `high | standard | fast` with the role's path (today an unknown value is dropped silently).
- Text: `core/roles/reviewer/role.md`, `core/roles/explorer/role.yaml`, `core/workflows/code-review/10-review.md`, `core/rules/testing.md`, `platforms/flutter/platform.yaml`, `docs/catalog-format.md`.

## Tests (test first)

1. `internal/adapters`: explorer is `model: haiku` and `effort: low` for Claude Code; Codex writes `model_reasoning_effort = "low"` and no model.
2. `internal/catalog`: `model: turbo` fails, naming the role.
3. Goldens: reviewer and explorer agent files for every agent, the `code-review` skill, `verify`/`tdd`/testing text in the Flutter samples, locks. Every changed file reviewed.
4. `go vet ./...`, `GOOS=windows go vet ./...`, `go test ./...`.
5. Manual: `flutter test --reporter failures-only` on a scratch package (outside expense_tracker) with one passing and one failing test, to record what a pass and a fail print.

## Decisions (maintainer, 2026-11-16)

1. Reviewer: one diff with `--function-context`, whole files only when needed (section 1 as written).
2. Explorer: keep `model: standard` (sonnet on Claude Code) and lower `effort` to `low`. No `fast` tier: the adapter and catalog changes in section 2 and "Code" are dropped, and so are tests 1 and 2 as written. Instead, test 1: the explorer is `model: sonnet` and `effort: low` for Claude Code, and `model_reasoning_effort = "low"` for Codex.

## Result

- `git diff --merge-base <base>` (git 2.30+) replaces `$(git merge-base <base> HEAD)`: same diff, no command substitution, which Claude Code asks permission for and a background reviewer cannot answer. The guard allows both.
- Explorer: `effort: low` for Claude Code (`model: sonnet` kept) and Codex.
- Flutter `failures-only`, checked on a scratch package: a pass prints `+6: All tests passed!` (one line, where `expanded` printed 8 for 6 tests); a fail prints the failing test's expectation and stack, then `+6 -1: Some tests failed.`
- The `review` wording says "results already in hand from `tdd` or `verify`": in the standard chain `code-review` runs before `verify`.
- The Flutter testing line had to stay short: the largest AGENTS.md is held to 23,788 bytes by `TestLargestOptionOfEveryFamilyStaysUnderTheLimit`.
- Tests updated for the intended change: explorer effort (adapters, sync), the reviewer input phrase (render).
