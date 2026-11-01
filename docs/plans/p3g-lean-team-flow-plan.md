# P3g plan: lean team flow (fewer tokens, same checks)

Status: superseded by `p4-skill-first-plan.md` before any code. Kept for its baseline numbers and root-cause trace.

Tier: standard (changes generated output across `core/roles`, `core/workflows`, `core/rules`, the guard and three adapters; no new adapter, no schema bump, no output layout change). Goldens of every sample change and are reviewed before merge.

## Goal

Cut the token cost of the `team` flow on a simple feature by roughly a quarter to a third, without dropping a role, a check or a gate. The cuts remove duplicated work, contradictions between roles, and an unbounded review loop. Nothing here changes what is reviewed or tested on the first pass.

## Evidence (baseline)

One test run of the `team` flow on a sample todo app (list, add, edit, delete, done, filter, local storage, Material 3). 16 role runs, PR merged. Numbers from the session and subagent transcripts; output tokens include thinking.

| Phase | Output tokens | Share |
|---|---|---|
| Planning (product-manager 25k, flutter-architect 81k, flutter-planner 215k) | 320k | 26% |
| flutter-engineer, first pass | 295k | 24% |
| Review round 1 (code-reviewer 39k, qa 62k, security 10k, tech-lead 23k) | 133k | 11% |
| Rework 1-3 and their re-reviews, tech-lead round 2 | 289k | 23% |
| Main session | 210k | 17% |
| **Total** | **~1.25M** | |

What drove the cost, traced to the source:

1. **Round 1 went NO-GO with zero BLOCKER and zero HIGH from code review.** The tech-lead's reason was a qa-engineer HIGH: "no UI criterion verified on a running app". That HIGH is built into the roles. `product-manager` must write UI and non-functional criteria (`core/roles/product-manager/role.md:43`). `flutter-planner` plans data-layer tests only, with UI tests only on request (`core/roles/flutter-planner/role.md:28`). `qa-engineer` marks every criterion without a test `NOT YET` (`core/roles/qa-engineer/role.md:21`). Each role followed its rules, and the rules contradict each other.
2. **MEDIUM findings turned into rework.** The tech-lead added MEDIUMs to the NO-GO as "cheap to fix now", although its own GO rule only blocks on BLOCKER or HIGH (`core/roles/tech-lead/role.md:35`). After each rework, "the review step runs again" (`:37`), with no limit. Each re-review found new MEDIUMs. The main session asked the user about each one separately, with no default, and the user chose "fix now" twice.
3. **The main session retypes read-only handoffs.** Read-only roles return their handoff and the main session saves it (`core/workflows/orchestrate/50-handoff.md:44`). That was about 60k characters written twice, and every handoff stayed in the main context (44M cache-read tokens over 241 turns).
4. **`code.md` duplicates the implementation.** It asks for "concrete per-file hunks that match the current code" (`core/roles/flutter-planner/role.md:33`). On a new app every file is new, so `code.md` reached 1,512 lines, about the same as `lib/` plus `test/`. The engineer then wrote it all again.
5. **Acceptance criteria are not scaled to the task.** The required non-functional group (delay, list size, text scale) produced criteria that no planned test covered. These became gaps G3 and G4 and extra questions to the user.

Not a toolkit cost: about 45 minutes of wall time in rework 2 were the machine sleeping and responses cut mid-stream. Time is not a target of this plan. Output tokens are.

Worth keeping: the planner read package source and caught two real defects before any code existed (a router default that dropped page transitions, and a SnackBar with an action that never closes on the current Flutter). Review round 1 and its re-review caught a duplicated task after Undo and a stuck form. L4 must keep the source reading; L2 must keep one re-review.

## Stages

### L1: every acceptance criterion gets a planned verification

- `flutter-planner`, plan.md item 6 becomes a verification map: every criterion id gets exactly one of `unit`, `widget`, `integration` or `manual`, with the test file for the first three. UI tests are still written only when the user asked. Without that request, UI criteria are mapped to `manual`.
- Items mapped to `manual` become one device checklist in plan.md. The main session hands it to the user at step 5 (`verify`). The PR Testing section records the result.
- `qa-engineer`, section 1: a criterion mapped to `manual` in the plan is reported as `MANUAL (planned)`, not `NOT YET`, and is never a finding. `NOT YET` and its severity stay for criteria the plan left unmapped, or mapped to a test that does not exist.
- `tech-lead`, Axis 2: `MANUAL (planned)` counts as covered pending the checklist. It is never a NO-GO reason. The open checklist goes under "For next phase" to the main session.
- Rule ids: `role.flutter-planner.verification-map`, `role.qa-engineer.manual-planned`, `role.tech-lead.manual-planned`.

### L2: one rework round, scoped re-review

- `tech-lead`, Decision: NO-GO only on a BLOCKER or HIGH from any axis, or an `Open:` item without an owner. MEDIUM and LOW never cause NO-GO, and "cheap to fix" is not a reason. Every MEDIUM goes into one `Notes:` list in the gate output.
- After the gate, the main session asks the user **once** about all MEDIUMs together, with the default "record in the PR Notes". The user may still pick any of them to fix. These fixes join the BLOCKER/HIGH rework if there is one. Otherwise they are made with no re-review, and step 5 `verify` is their evidence.
- `tech-lead` `:37` becomes a rework budget. After the one rework round, the re-review covers only the rework diff (`git diff` of the files the rework touched) and only by `flutter-code-reviewer`.
  - `qa-engineer` reruns only when the rework adds, changes or removes an identifier, or changes which test covers a criterion.
  - `flutter-security-reviewer` reruns only when the rework touches one of its triggers.
  - `tech-lead` round 2 checks only that each rework line is closed.
- A second NO-GO does not start a third round. Its findings go to the user as a decision: fix with no further review, or record in the PR Notes.
- `core/workflows/orchestrate/30-role-gate.md` gets the rerun conditions above as a "Rework rounds" paragraph under the role table.
- Rule ids: `role.tech-lead.medium-never-blocks`, `role.tech-lead.rework-budget`, `orchestrate.rereview-scope`, `orchestrate.medium-one-question`.

### L3: read-only roles save their own handoff

- New guard rule: a `read` role may write exactly one path family, `<plans>/<feature>/handoff/<role>[-<suffix>].md`, where `<role>` is its own name. Whole-file writes only; edits stay blocked. Every other write stays blocked with the current message.
- Claude Code adapter: `read` gains `Write` in its tool list. The guard is the enforcement.
- Codex adapter: no tool list exists, so nothing changes in the agent file; the guard enforces.
- Antigravity adapter: the tool allowlist is the only enforcement and cannot be limited to a path (`lib/src/adapters/antigravity.dart:72`). Read roles keep the read-only list there. On Antigravity the main session still saves their handoff (see Decisions needed, 1).
- `50-handoff.md:44`: "Every role saves its own handoff and returns only the verdict line and the handoff path; the main session reads the file instead of the reply."
- The role bodies of `flutter-code-reviewer`, `flutter-security-reviewer` and `tech-lead` get the same line.
- The tech-lead keeps returning its draft PR description as text, because `{{paths.pr}}` stays closed to it.
- Rule ids: `handoff.read-only-writes-own`, `guard.read-role-own-handoff`.

### L4: code.md carries decisions, not implementations

- `flutter-planner`, code.md:
  - New files: public signatures and the shape of each type. No method bodies.
  - Changed files: hunks only where the change is not obvious from plan.md, such as an ordering constraint, an API whose default is wrong (with the package source line that proves it), a persisted format, or a DI order.
  - The test list stays as it is today.
- Package-source checks move into plan.md "Real risks" with the source `file:line`. That way they survive even with a short code.md.
- Rule id: `role.flutter-planner.code-md-decisions-only`.

### L5: acceptance criteria scaled to the tier

- `product-manager`: happy path, edge cases and error states are always written. The non-functional group is written only when the prompt, the tracker task or the PRD asks for it, or when a selected pack requires it. Otherwise "Non-functional: none requested" goes under Assumptions.
- The orchestrator passes the step 0 tier into the role prompt as `Tier: <tier>`, and the PM keeps the criteria count proportionate. This is guidance, not a cap: no number is enforced, because a hard cap would drop real error states.
- Rule id: `role.product-manager.criteria-scale`.

### L6: tech-lead effort

- `core/roles/tech-lead/role.yaml` effort `high` -> `medium`. The tech-lead takes reviewer verdicts as-is and does not re-review lines (`role.md:20`), so its work is aggregation and spec mapping.
- The model stays `high`.

## Expected effect (estimate, to be measured)

| Stage | Baseline it targets | Expected |
|---|---|---|
| L1 + L2 | rework rounds 289k | ~100k (one round, scoped re-review, no QA rerun) |
| L3 | main session 210k | ~180k, plus a smaller main context |
| L4 | planner 215k | lower, not estimated; measured |
| L5 | indirect: fewer gaps and questions | measured as questions to the user |
| L6 | tech-lead 45k | small |

Overall target: ~1.25M -> 0.85-0.95M output tokens on the same task, with the same defects caught.

## How we check

- `dart analyze` and `dart test` in this repo; every sample golden regenerated and its diff read line by line.
- Guard unit tests (in `test/unit/`):
  - a `read` role writing its own `handoff/<role>.md` is allowed, and so is `handoff/<role>-rework-1.md`;
  - writing another role's handoff, a file outside `handoff/`, or anything under `lib/` is blocked;
  - an edit of its own handoff is blocked.
- The generated Claude agent file for each read role lists `Write`; the Antigravity one does not.
- Re-run the same prompt on the sample todo app from a fresh session and split the transcripts per role, with the same method as the baseline. Compare:
  - output tokens per role;
  - review rounds;
  - questions to the user;
  - the defects caught.

  The quality bar is that the four defects listed under "Worth keeping" are still found, or prevented in the plan.

## Not doing

- Downgrading the flow or skipping roles by default. The `team` flow keeps all its roles; only their duplicated work goes.
- Smaller models for any role.
- A hard cap on acceptance criteria or on test runs beyond the existing "aim for at most 6".
- Changing the tier table. A new app still reads as `large`; L1-L5 make `large` cheaper instead.
- A `dashcrew usage` command for the measurement. The split is a one-off script for now.

## Decisions needed

1. L3 on Antigravity: keep read roles read-only there, with the main session saving their handoff (proposed)? Or give them `write_to_file` with only an instruction to limit the path?
2. L2 MEDIUM default: one question with the default "record in PR Notes" (proposed)? Or record all MEDIUMs without asking?
3. L2 after a second NO-GO: hand the findings to the user as a decision with no third review (proposed)? Or allow one more scoped round?
4. L1 manual checklist: run by the user at step 5 `verify`, with the PR not opened until it is reported (proposed)? Or listed in the PR Testing section as pending, so it does not block the PR?
5. L6: effort `medium` for tech-lead only (proposed)? Or also for `flutter-security-reviewer`, which used 10k output tokens on its run?

## Progress

Not started.
