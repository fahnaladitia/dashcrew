# P4 plan: skill-first toolkit

Tier: large (output layout change: most role files disappear, the `orchestrate` workflow is removed, profile keys change, the guard hash moves). Every golden fixture changes and is reviewed before merge.

Supersedes `p3g-lean-team-flow-plan.md`. Follow-ups P5-P7 (shared knowledge, tracker status, multi-repo tracks): `docs/ideas/knowledge-tracker-multirepo.md`.

## Goal

dashcrew keeps every agent on a team aligned: same rules, same skills, same guard, same PR format, whatever agent each member uses. P4 removes what is not alignment but process overhead.

- One agent (the main session) plans and builds. Skills, not personas, carry the expertise. They are picked once at step 0, in one visible line.
- Two subagents remain, only where a separate context earns its cost: `reviewer` (independent eyes on the plan and the diff) and `explorer` (read-only research that returns a `file:line` map).
- The GitHub plan gate is removed. The task owner confirms the plan in the session; the team reviews in the PR, where `dashcrew check` stays.
- Four general coding rules are added to core (think before coding, simplicity, surgical changes, goal-driven execution).
- Core becomes platform-agnostic. Flutter moves into the first platform pack, the reference one. Any other stack works from day one through `platform: other`, and a team can add depth with its own project platform. No stack ever needs a fork of core or a dashcrew release.

## Why (evidence)

A `team` flow run on a sample todo app (list, add, edit, delete, done, filter, local storage) took 16 role runs and ~1.25M output tokens. Split from the transcripts:

- Planning by three personas: 320k.
- First implementation: 295k.
- Four parallel reviewers and a tech-lead: 133k.
- Three rework rounds: 289k.
- Main session: 210k.

Each role started with an empty context and re-read the codebase (34M cache-read tokens across subagents). Handoffs, the roster file and the tech-lead existed only to coordinate between those contexts.

The run ended in NO-GO with zero BLOCKER and zero HIGH from code review. The cause was a contradiction between three roles' rules about UI criteria. Roles are also the least portable output: Antigravity cannot limit a role to a path, and Codex has no per-agent tool list.

What produced quality, and must survive:

- Package source read before use. The plan caught a router default that dropped page transitions, and a SnackBar with an action that never closes.
- An independent review of the diff. It caught a task duplicated after Undo, and a form stuck after save.
- Test-first data layer, and scoped verify with real output.

## The new chain

Step 0 stays the single confirmation point. It prints one block:

```
TASK: <polished request>
Tier: trivial | small | standard | large   (<evidence>)
Assumptions: <each one, or none>
Questions: <all of them, once, or none>
Skills: <name> (<why>), <name> (<why>)
Success criteria: <id> <criterion> -> unit | widget | integration | manual
```

The user confirms or edits the block once: tier, skills, criteria and answers together. No roster file, no handoff files.

| Tier | Chain after step 0 |
|---|---|
| trivial | edit -> analyze the file -> one-line summary |
| small | test first -> implement -> scoped analyze + test -> summary |
| standard | plan (one file) -> implement test first -> `reviewer` on the diff -> fix -> verify -> PR description |
| large | plan -> user confirms -> `reviewer` on the plan -> implement test first -> `reviewer` on the diff -> at most one rework round -> verify -> PR description |

- `policy.tier_gates` keeps its meaning: with it on, only standard and large stop after the plan.
- Loops stop on the success criteria written at step 0, not when a reviewer runs out of findings.
  - BLOCKER and HIGH are fixed.
  - MEDIUM and LOW go to the PR Notes by default. They are asked about once, as one list.
- A re-review after rework covers only the rework diff. A second failing review goes to the user as a decision, never into a third round.
- Items mapped to `manual` form one device checklist, handed to the user at verify and recorded in the PR Testing section.
- The PR description, the PR question and the summary stay as they are.

## Stages

### S0: platform layer (first, so later stages write Flutter content straight into the Flutter pack)

Today the engine in `lib/` is almost platform-free: `pubspec` appears only in `init/detect.dart` and `worktree/worktree.dart`. The content is not:

- 13 of 23 `core/rules` files mention Flutter or Dart;
- 29 of 51 workflow files do;
- 11 of 30 skill files do.

**Profile.**
- A new top-level key `platform`, which takes one of:
  - a built-in platform (`flutter`);
  - a project platform name under `.ai/platforms/`;
  - `other`.
- A missing key reads as `flutter`, so existing profiles keep working, and `sync` writes the key on the next run.

**Three levels of support.** Every stack works at level 1.

| Level | How | What the team gets |
|---|---|---|
| 1. `platform: other` | The profile declares the platform inline: `platform_config: { detect: [<file>], commands: { analyze, test, test_scoped, deps, format? } }` | All of core: the chain, principles, guard, `reviewer` and `explorer`, PR check, and later knowledge and tracks. No stack-specific rules. |
| 2. Project platform | The team writes `.ai/platforms/<name>/` in its own repo, in the same format as a built-in platform: `platform.yaml`, rules, skills, review checklists, packs | Depth for that stack, following the team's conventions, without waiting for a dashcrew release |
| 3. Built-in platform | A project platform that proves generic moves into `platforms/` in this repo | Other teams can use it |

This is the pattern dashcrew already uses for pack families (`use: other`, project packs in `.ai/packs/`), one level up.

**Catalog layout.**
- `platforms/<platform>/platform.yaml` holds:
  - `detect`: the files that identify the platform, for example `pubspec.yaml` or `go.mod`;
  - `commands`: analyze, test, scoped test, format, get dependencies;
  - `doc_comment`: the doc comment convention, for example Dartdoc `///`.
- `platforms/<platform>/rules/`, `skills/` and `workflows/` hold fragments with the same front matter as core.
- `platforms/<platform>/packs/<family>/<option>/` holds the platform's pack families.
- Platform-free families stay in `packs/`: `tracker`, plus `design` while only a front-end platform uses it.

**Core content.**
- Commands become template variables: `{{platform.commands.analyze}}`, `{{platform.commands.test_scoped}}` and so on. Core never names `flutter test`.
- Flutter-only text moves to `platforms/flutter/`. This covers:
  - widget lifecycle, BuildContext and dispose rules;
  - Dartdoc;
  - the data flow `client -> DTO -> mapper -> model`;
  - QA identifiers via `Semantics`;
  - `pubspec_overrides`;
  - build_runner, Gradle and CocoaPods in `build-fix`.
- Core keeps the shape of each rule: test-first data layer, scoped verify with evidence, documentation on changed public units, a layered data flow named by the platform.

**Detect, and the place of the existing families.**
- `init` matches the `detect` files of every built-in and project platform.
- A repo that matches none is never silently given Flutter. `init` offers `platform: other` and asks for the commands.
- A repo that matches two platforms asks which one; a monorepo with two platforms stays out of scope.
- The way a platform links repos in a P7 track (`pubspec_overrides.yaml` for Flutter) is a platform field (`link`). Level 1 tracks work without automatic linking.
- The existing families (`state`, `di`, `router`, `network`, `storage`, `test`, `analytics`, `qa`) move under `platforms/flutter/packs/`.
- Profile `stack:` keys stay as they are; they are resolved inside the selected platform.

**Check.** The Flutter goldens must render byte-identical before and after S0, except for the new `platform: flutter` line. That proves the move lost nothing, before S1-S7 change content on purpose.

### S1: core coding rules

New `core/rules/principles.md` (agents_md, before `code.md`), adapted from the four general LLM coding principles:

- **Think before coding.** State assumptions; when the request reads two ways, name both instead of picking one silently. Say so when a simpler approach exists. Ask every question at step 0, together.
- **Simplicity.** The minimum that solves the request, in code and in documents. A plan, criteria list or doc that could be a third of its length is rewritten.
- **Surgical changes.** Every changed line traces to the request. Remove imports, variables and functions your own change made unused. Mention unrelated dead code; never delete it unasked.
- **Goal-driven.** Turn the task into verifiable criteria first. Loop until they pass, then stop.

`core/rules/code.md` keeps its project-specific lines; the overlap moves to `principles.md`.

### S2: skills carry the expertise

Role bodies become skills or workflow sections. Nothing they check is dropped.

| From | To |
|---|---|
| `product-manager` | New skill `acceptance-criteria`: Given/When/Then, error states and recovery, non-functional only when asked. Active when the task has no criteria. |
| `flutter-architect` | New skill `feature-architecture`: widget split, state boundaries, DI placement, QA identifier placement. Active for a new screen, state holder or feature folder. |
| `flutter-planner` | The `plan` workflow, which already holds most of it. `code.md` carries signatures and decision hunks only. Package defaults are checked from source, with the line, under Risks. |
| `flutter-engineer`, `dart-tdd-guide` | The `tdd` workflow (already there) |
| `qa-engineer` | The verification map at step 0 and in the plan, plus a review checklist section in the qa pack skill |
| `flutter-security-reviewer` | The `mobile-security` skill (already there), loaded by `reviewer` when its triggers hold |
| `tech-lead` | Gone. Its spec axis (each criterion met, with `file:line`) moves into the `reviewer` diff review. Its draft PR description is already the `pr-description` skill. |
| `design-refiner` (figma pack role) | The figma pack's skill, next to `design-fidelity` |
| `local-storage-engineer` (hive pack role) | The hive pack's skill |
| `flutter-build-error-resolver` | The `build-fix` workflow, run by the main session |

Skill selection:

- AGENTS.md gets a generated **skill index**: one row per installed skill with its `when_to_use`, built from `meta.yaml`. The agent reads the index at step 0 and writes the `Skills:` line from it. No hand-kept table.
- New optional `meta.yaml` field `review: true`. The `reviewer` loads such a skill's checklist when that skill is in the step 0 line. This applies to `mobile-security`, the qa pack skill and `design-fidelity`.
- Pack `role.md` fragments become skill fragments. The `role/*` render target is kept only for the two remaining roles.

### S3: two roles

- **`reviewer`**: capability `read`, model high, effort high.
  - On a plan, it checks that the plan is complete against the criteria, the risks, and that it reuses existing code.
  - On a diff, it reviews every changed file in full against AGENTS.md and the active review checklists, and maps each criterion to `file:line` and a test.
  - It returns findings in the existing `code-review` format (BLOCKER, HIGH, MEDIUM with `file:line` and the fix) and a verdict. It writes nothing.
- **`explorer`**: capability `read`, model standard, effort medium.
  - Used by `plan` and `audit` when the answer needs a sweep across many files.
  - Returns a short map: the files, symbols and `file:line` that answer the question. No file dumps.
- Removed role definitions: all eleven others in `core/roles/` and the two pack roles.
- `worktree-monitor` is also removed. `worktree watch` runs its watcher in the main session's background shell instead (see Decisions needed, 1).

### S4: workflows

- **Removed:** `orchestrate` (its job is now the chain itself), `checkpoint` (see Decisions needed, 2).
- **`code-review`:** runs the `reviewer` subagent and passes it the base branch, the step 0 criteria and the active review skills.
- **`plan`:**
  - drops the roster and handoff sections;
  - adds the verification map and the reuse search (through `explorer` when the codebase is large);
  - keeps `Verdict: GO | NO-GO`.
- **`learn`:** becomes the retro. The memory proposals it writes now are the input P5 will route to the shared knowledge repo. In P4 the output format stays the same.
- **`audit`, `tdd`, `verify`, `build-fix`, `worktree`:** references to roles are replaced by `explorer`, `reviewer` or the main session.
- **Workflow set after P4:** `plan`, `tdd`, `code-review`, `verify`, `learn`, `audit`, `worktree`, `build-fix`.

### S5: remove the GitHub plan gate

- Delete:
  - `lib/src/gate/` and the `gate` CLI command;
  - `lib/src/guard/gate_state.dart` and the plan-gate branch in the guard;
  - the `plan-approval` check in `lib/src/check/check.dart`;
  - `core/rules/gate.md`, `core/workflows/plan/55-gate.md`, `core/pr/plan-link.md` and `core/skills/pr-description/plan-link.md`;
  - their tests (`gate_test.dart`, `gate_state_test.dart`, the plan-approval cases in `check_test.dart`).
- Keep:
  - `dashcrew check` with the toolkit and PR-section checks;
  - the generated GitHub workflow;
  - the guard rule that blocks `gh` commands containing `/approve`. It is harmless and protects any approval flow a team keeps on GitHub.
- Profile: `policy.plan_gate` is accepted and ignored for one release, with a warning (`plan_gate was removed in P4; delete it from .ai/project.yaml`). After that it is an unknown key.
- `.ai/guard.json` loses `plan_gate`; the guard hash changes.

### S6: forced migration through `sync`

- `sync` already deletes files that are in the lock but no longer generated, so the eleven role files per agent, the `orchestrate` and `checkpoint` skills, and the gate rule disappear on the next `sync` with no new code. A golden checks this, starting from a P3 output.
- Files the team wrote are never touched:
  - plans and handoffs under `{{paths.plans}}`;
  - project memory;
  - the AGENTS.md project block.
- `sync` prints one migration note when the lock's `dashcrew_version` predates P4:
  - roles were removed;
  - the plan gate was removed;
  - `orchestrate team` is now the default chain with skills;
  - where the README explains it.
- `doctor` reports a stale `plan_gate` key and leftover `.ai/plans/*/handoff/00-roster.md` files as information, not failure.

### S7: README and docs

- README:
  - "How it works": roles become two subagents.
  - Remove the "Gates on GitHub" plan-gate half; keep the merge gate.
  - New section "Choosing skills" (index, step 0 line, review checklists).
  - Migration section.
- `docs/ideas/github-gates-multi-repo.md`: a note at the top that the plan gate was dropped in P4; the merge gate and the multi-repo part live on.

## How we check

- `dart analyze`, `dart test` in this repo.
- Goldens:
  - after S0 alone: Flutter samples identical except the `platform: flutter` line;
  - a new golden `sample-other` with `platform: other` (a non-Flutter fixture with declared commands): core renders with those commands, and no Flutter text appears anywhere in the output;
  - a new golden `sample-project-platform` with a minimal `.ai/platforms/<name>/` that adds one rule and one review skill;
  - every sample regenerated and read line by line;
  - a new golden `sample-migrate-p3` runs `sync` over a P3 output, and the role files, `orchestrate`, `checkpoint` and the gate rule are gone, while plans and the project block are untouched.
- Guard unit tests: no plan-gate input, the `/approve` block still works, and the new hash is stable.
- `check` tests: PR sections pass without a `Plan:` line.
- Live run on the sample todo app from a fresh session, the same prompt as the baseline. Compare:
  - output tokens: target about half of 1.25M, measured, not assumed;
  - review rounds and questions to the user;
  - whether the four defects named under Why are still caught, or prevented in the plan. If one is missed, P4 is not done.

## Not doing

- Shared knowledge repo, knowledge MCP, tracker status changes, multi-repo tracks: P5-P7.
- A second built-in platform. P8 takes the team's next stack through levels 1 and 2 first.
- A repo with two platforms (monorepo with Flutter and Go side by side).
- A web board. The tracker board is the team view (decided).
- Smaller models for the reviewer.
- Changing pack families, the guard's other rules, or the PR template sections.
- Keeping the old team flow as an optional pack (decided: forced migration).

## Decisions (confirmed)

1. Roles kept: `reviewer` and `explorer`.
2. GitHub plan gate: removed entirely.
3. Migration: forced through `sync`; no optional legacy pack.
4. `worktree-monitor` is removed; `worktree watch` runs as a background shell in the main session.
5. `checkpoint` is removed. When a test fails at verify, the main session checks once whether it also fails on the base.
6. `reviewer` reviews the plan for tier large only.
7. Audience: internal team use first. Depth for the team's platforms comes before public docs and a pack ecosystem. Team-specific conventions live in the team's private project packs and knowledge repo, never in this repo.
8. Platforms: stack-agnostic. Flutter is the built-in reference platform. Any other stack enters through level 1 (`platform: other`), then level 2 (a project platform); a built-in platform only once one proves generic. Go is the current candidate for the next stack. The name stays dashcrew.

## Progress

- S0 done.
  - `platforms/flutter/` holds `platform.yaml` (detect, commands, link, doc_comment, mcp_servers) and the 8 Flutter pack families. `tracker` and `design` stay in `packs/`.
  - Moved whole: 7 files. Split by order: 1 file. Replaced at the same path: 37 files. A platform fragment at the same path as a core fragment replaces it, and core holds a generic rewrite with `{{platform.commands.*}}`. Mixed text cannot be split by order without changing the Flutter output, which is why this replacement rule exists.
  - Profile: `platform` (`flutter` by default, a project platform, or `other` with `platform_config`). `.ai/platforms/<name>/` loads like project packs. `init` detects the platform, and with `--yes` stops when none or several match. Worktree runs `platform.commands.deps`.
  - New goldens: `sample-other`, `sample-project-platform`, and the term scan `platform_terms_test.dart`. Its allowlist covers the role files and role names, `orchestrate` and `checkpoint`; S3 and S4 remove it.
  - Flutter goldens are byte-identical; `init-app` only gains `platform: flutter`. Analyze clean, 384 tests pass.
  - Left for later: the pubspec scan and flavor detection (run only for flutter), the `fvm` prefix in worktree, and the guard's post-edit Dart check. `guard/compile.dart` is unchanged, so the guard hash stays.
- S5 done.
  - Removed: `lib/src/gate/`, `guard/gate_state.dart`, the `gate` command (it now exits 64), the `plan-approval` check, `rules/gate.md`, `plan/55-gate.md` and both `plan-link.md` files.
  - The `gh ... /approve` guard block stays.
  - `policy.plan_gate` parses with a warning in `sync` and `doctor`. An old guard.json with `plan_gate` still loads.
  - Golden diffs only in guard.json (field removed), the shim and lock (new guard source hash).
  - Analyze clean, 370 tests pass.
  - Left over: the generated workflow still passes `GITHUB_TOKEN`, which nothing reads now. Remove it in S7 or P4b.
- S1-S4 done.
  - S1: `core/rules/principles.md` (order 49); `code.md` lost the overlap.
  - Chain rules rewritten to the step 0 block (with a `Branch:` line) and the tier table, plus `core/rules/skill-index.md`.
  - S2:
    - new skills `acceptance-criteria` and `feature-architecture`, plus pack skills `figma-design`, `hive-storage` and `qa-identifiers`;
    - `meta.yaml` `review: true` on `mobile-security`, `qa-identifiers` and `design-fidelity`;
    - the generated skill index (sorted by name), which `doctor` also checks;
    - local-storage checks in `data-layer-tdd/20-local-storage.md`.
  - S3: only `reviewer` (read/high/high) and `explorer` (read/standard/medium) remain. 13 role directories deleted.
  - S4:
    - `orchestrate`, `checkpoint` and `plan/30-branch.md` deleted;
    - `code-review` runs the reviewer, which also reviews a large plan after the user confirms it;
    - `verify` checks a failing test once against the base in a detached worktree and hands over the manual checklist;
    - `code.md` only for tier large; LOW severity added.
  - Rule coverage: 387 ids, 333 stated, 54 dropped with reasons (checkpoint, orchestrate, handoff, roster, persona write scopes). `sample-other` has zero Flutter/Dart terms; the allowlist is gone.
  - Analyze clean, 374 tests pass.
  - Risk: the heaviest pack combination renders AGENTS.md at 23,792 bytes against Antigravity's 24,000-byte cut (largest golden 22,526). New AGENTS.md text needs a matching cut.
- S6-S7 done.
  - `sync` detects a pre-P4 project by locked role files of the 13 old roles that it no longer renders (the lock version is 0.0.1 either way). It then prints one `note: Migrated to P4: ...`.
  - Golden `sample-migrate-p3` is a frozen P3 output (three agents, `plan_gate: true`, hive and figma) plus team files. After `sync`, the old roles, `orchestrate`, `checkpoint` and the gate are gone; the team files and the project block are byte-identical; the tree equals a fresh sync; a second sync writes nothing.
  - `doctor` gained an `info` status and a `handoff-rosters` check.
  - README rewritten for platforms, skills and migration; the `gate` docs are gone.
  - The generated workflow lost `GITHUB_TOKEN` and `issues: read`.
  - CLI and pubspec descriptions now say "with Flutter built in".
  - Analyze (`--fatal-infos`) clean, 385 tests pass. 511 files changed against HEAD.
- Not yet done: the live run on the sample todo app (fresh session, same prompt as the baseline), which measures tokens per role and whether the four defects are still caught. P4 is not closed until that run is reported.
