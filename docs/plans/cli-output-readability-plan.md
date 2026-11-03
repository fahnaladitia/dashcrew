# Readable chat blocks (step 0, review, verify, summary)

Tier: standard. About 25 neutral sources in `core/` and `platforms/flutter/`, all of them change generated output, plus the goldens and README.

## Problem

The blocks the agent prints in the terminal are hard to scan:

- Step 0 packs lists into one line: `Skills`, `Success criteria`, `Questions` and the three branch names each sit on a single 18-column-indented line. With two or three items they wrap at the terminal edge back to column 0.
- Nothing separates one part of a block from the next.
- Questions and assumptions have no ids, so the user cannot answer "Q2: ...".
- The step 0 block never says what the user should reply.
- Placeholders differ per block: `<x>` in step 0 and review, `[A/B]` in verify.
- Review puts the verdict last.
- The end-of-task summary has no layout.
- Comparisons (options, before and after) come as prose or bullets.

## Layout (confirmed: variant A)

```text
STEP 0  feat: Add a status filter (all, paid, unpaid) to the order list.
========================================================================
Scope      lib/features/orders/ (order_list_page, order_list_cubit,
           order_repository), test/features/orders/
Not doing  keeping the filter across app restarts
Tier       standard  (4 files across ui, state and data)

-- Assumptions ---------------------------------------------------------
A1    the orders API accepts ?status=paid|unpaid

-- Questions -----------------------------------------------------------
Q1    default filter: "all", or the last one used?

-- Skills --------------------------------------------------------------
NAME                 WHY
bloc                 state lives in OrderListCubit

-- Success criteria ----------------------------------------------------
ID    CRITERION                                 TEST
AC-1  picking "paid" shows only paid orders     widget

-- Branch --------------------------------------------------------------
1     feat/order-status-filter
2     feat/orders-filter-by-status
3     feat/order-list-filter
base  develop or main?
========================================================================
Reply "ok", or answer by id (Q1 ...), pick a branch and a base, edit any line.
```

- ASCII only (`format.no-em-dash`): `=` frames the block, `-- Name ---` opens a section, both 72 columns wide.
- Single values are `Label  value` rows; lists are rows with an id; tables have an uppercase header row. Columns are aligned with spaces, never `|` or box characters, so a cell copies clean.
- A command sits alone on its line, never wrapped.
- An empty list prints `none` under its section rule.
- A comparison is a table in the same layout: one row per item, one column per point compared.

## Always ask

Added after confirmation: a question or open question is always asked, with the agent's question tool when it has one, and the agent waits. One sentence in `principles.md` (AGENTS.md) and the `prompt-polish` confirmation step.

## Where the templates live

The heaviest AGENTS.md is 23782 bytes against a 23788 budget and the 24000 Antigravity limit, so full templates cannot go there.

- AGENTS.md (`core/rules/output.md`) gets one short "Chat blocks" rule with the layout above and the comparison rule.
- AGENTS.md (`chain.md`) drops the step 0 block and points to `prompt-polish`, which is already mandatory at step 0 and renders for every agent. That pays for the new rule.
- The summary bullet in `chain.md` becomes a `SUMMARY` block named field by field, in the chat block layout.
- Full templates: step 0 in `skill/prompt-polish`; `REVIEW` in `workflow/code-review` and `role/reviewer`; `VERIFY` in `workflow/verify`; `LEARN` and `BUILD FIX` reshaped the same way.

## References

Step 0 labels become sections, so references follow:

| Before | After |
|---|---|
| step 0 `Skills:` line | step 0 `Skills` list |
| step 0 `Questions:` line | step 0 `Questions` list |
| step 0 `Tier:` line | step 0 `Tier` row |
| step 0 `Release:` / `Spec:` | step 0 `Release` / `Spec` |
| step 0 `TASK` line | step 0 `Scope` row |

The plan's own `Questions:` line is a plan field and stays. README sections "Choosing skills" and line 174 show the new block; the changelog entry at line 523 is history and stays.

## Not doing

- No change to CLI command output (`dashcrew` usage, `doctor`, `check`).
- No rule id renamed: `format.chat-blocks`, `format.compare-as-table` and `principle.always-ask` are added; `workflow.step0-release` and `workflow.step0-spec` leave `chain.md`, since `prompt-polish` states them (`step0.release`, `step0.spec`). Tiers stay; a trivial task keeps its one-line summary.
- No colors or ANSI codes.

## Tests

1. `internal/render/catalog_render_test.go`: move the step 0 `Release` and `Spec` assertions from `agents_md` to `skill/prompt-polish`, update the reviewer `Spec` texts, add assertions for the chat block rule and the `SUMMARY` block in `agents_md`, the reply line in `skill/prompt-polish`, `REVIEW  <` in `workflow/code-review`, `VERIFY  <` in `workflow/verify`; they fail first.
2. Edit the sources.
3. `DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard`, review `git diff --stat test/golden cmd/dashcrew/testdata internal/guard/testdata`; `test/golden/sample-migrate-p3/p3/` stays untouched.
4. `go vet ./...` and `go test ./...`; the AGENTS.md budget test passes without raising the budget.

## Review rounds

- Round 1 (10 findings) and round 2 (8 findings) fixed, except the two decisions below.
- The full `SUMMARY` template lives in `skill/pr-description` (`core/skills/pr-description/summary.md`, order 400); AGENTS.md names it and its fields. Tracker and analytics packs add their row (`Tracker`, `Analytics`) with an order 410 fragment, so a row renders only with its pack. The doc comment row is `Docs` on every platform.
- Follow-up task, not here: the step 0 and `REVIEW` templates are copied in `core/` and `platforms/flutter/` (whole-file overrides, the existing pattern). Collapsing them into one source with a platform variable for the UI test type is its own change; `TestChatBlocksShareOneLayout` guards the `widget` / `ui` difference meanwhile.
- `internal/guard/testdata/corpus_outcomes.txt` comes back reordered (same lines) on every golden rewrite; it was restored to HEAD each time. The unstable order is a separate issue.
