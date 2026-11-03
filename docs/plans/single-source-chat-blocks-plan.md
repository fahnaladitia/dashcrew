# One source for the chat block templates

Tier: standard. Neutral sources in `core/` and `platforms/flutter/` across four workflows and skills, plus one role; generated output changes in two places (verify, reviewer) and in the lock digests. Follow-up from `docs/plans/cli-output-readability-plan.md`.

## Problem

Whole-file platform overrides copy the chat blocks: the step 0 block (`prompt-polish`), the `REVIEW` verdict (`code-review` and the `reviewer` role), `VERIFY` and `BUILD FIX` each exist in `core/` and `platforms/flutter/`. A layout change has to be made in up to six places, and a missed copy makes flutter and other platforms print different blocks.

The copies differ only outside the blocks:

| File | Flutter differs in | Block |
|---|---|---|
| `skills/prompt-polish/polish.md` | Lines: Dartdoc, HTTP client, `widget` | identical |
| `workflows/code-review/30-findings.md` | Documentation wording, the finding example | identical |
| `workflows/build-fix/10-build-fix.md` | diagnose steps (Gradle, CocoaPods, build_runner) | identical, with Rules |
| `workflows/verify/30-output.md` | one word: `widgets` for `elements` | the only difference |
| `roles/reviewer/role.md` (core only) | none; it repeats the code-review verdict block | copy |

## Approach

Split each file at a section boundary so the block sits in a core fragment no platform overrides; the platform keeps overriding only the prose that differs. The renderer sorts fragments by `order` and joins them with one blank line after trimming, so a split at a blank line before a `##` heading renders byte for byte the same.

| Source today | After |
|---|---|
| `core/skills/prompt-polish/polish.md` | `10-polish.md` (order 10: intro, refine, block) + `20-lines.md` (order 20: Lines, Confirmation) |
| `platforms/flutter/skills/prompt-polish/polish.md` | `20-lines.md` only |
| `core/workflows/code-review/30-findings.md` | `30-findings.md` (Documentation, Rejected, Finding format) + `35-verdict.md` (Verdict) |
| `platforms/flutter/workflows/code-review/30-findings.md` | Verdict section removed |
| `core/workflows/build-fix/10-build-fix.md` | `10-build-fix.md` (diagnose steps) + `15-rules-output.md` (Rules, Output; before `20-dependencies.md`) |
| `platforms/flutter/workflows/build-fix/10-build-fix.md` | Rules and Output removed |
| `core/workflows/verify/30-output.md` | QA line made platform-neutral (question Q2) |
| `platforms/flutter/workflows/verify/30-output.md` | deleted |
| `core/roles/reviewer/role.md` | per question Q1 |

Rule ids move with the text they state; no id is renamed. Rule ids are not written to generated files.

## Questions

- Q1, reviewer role: a fragment has one target, so the role cannot share the `35-verdict.md` fragment. (a) The role names the block instead of copying it: "Return the `REVIEW` block in the `code-review` workflow's Verdict format". The role already reads that workflow for its priorities and finding format. (b) Keep the role's copy and add a test that it equals the workflow's block.
- Q2, verify QA line: (a) `QA ids     <OK | n missing an identifier | N/A>` for every platform, so the flutter override goes. (b) Keep the flutter override for the one word.

## Generated output

- `prompt-polish`, `code-review`, `build-fix`: byte-identical renders; checked by the golden run showing no change in those files.
- `verify`: the QA line on every platform with Q2 (a).
- `reviewer` agent files on every adapter with Q1 (a).
- `.ai/lock.json` digests, since the catalog files change.

## Tests

1. Test first: `TestEachChatBlockHasOneSource` in `internal/render` walks the shipped catalog fragments and counts the sources whose body holds each block title (`STEP 0  <`, `REVIEW  <`, `VERIFY  <`, `BUILD FIX\n=`, `LEARN\n=`, `SUMMARY\n=`); each must be exactly one. It fails today on step 0, REVIEW, VERIFY and BUILD FIX.
2. Split the files.
3. `DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard`; review `git diff --stat` on the goldens: only verify, reviewer and lock files change. `test/golden/sample-migrate-p3/p3/` stays untouched.
4. `go vet ./...`, `go test ./...`; the AGENTS.md budget is untouched (no agents_md fragment changes).

## Not doing

- New platform variables or schema keys.
- `chain.md` and other whole-file overrides that hold no chat block.
- Fragments with several targets in the catalog format.
