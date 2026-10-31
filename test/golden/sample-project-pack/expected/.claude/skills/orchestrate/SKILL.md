---
name: orchestrate
description: "Run a chain of role agents for large work, one phase at a time, with a roster decided up front and handoffs saved as files."
argument-hint: "team | feature | bugfix | refactor | security | slicing <task description>"
---

# orchestrate

Arguments: a flow name (`team`, `feature`, `bugfix`, `refactor`, `security`, `slicing`), then the task description.

`orchestrate` replaces `plan` through `code-review`. Checkpoint, verify, the PR description and the PR question stay with the main session.

## More than one task

- When the description holds two or more tasks (separated by `;`, new lines or a numbered list), pick no flow and run no role.
- Forward the description as is to `worktree backlog` and follow it to the end: matrix, base question, confirmation, one worktree per independent task.
- Only then run the flow, and only for the task the matrix marks `current` or `inline`.
- Never run the flow for a task that belongs to another worktree, except an `inline` task.

## Flows

| Flow | Chain |
|---|---|
| `feature` | flutter-planner -> dart-tdd-guide -> flutter-code-reviewer |
| `bugfix` | flutter-planner -> dart-tdd-guide -> flutter-code-reviewer |
| `refactor` | flutter-architect -> flutter-code-reviewer -> dart-tdd-guide |
| `security` | flutter-security-reviewer -> flutter-code-reviewer |
| `slicing` | flutter-planner -> flutter-architect -> flutter-code-reviewer |
| `team` | product-manager -> design-refiner -> flutter-architect -> flutter-planner -> [plan gate] -> flutter-engineer (+ roles added by selected packs) -> flutter-code-reviewer, qa-engineer, flutter-security-reviewer (in parallel) -> tech-lead |

Plan gate: verdict GO on tier `trivial` or `small` continues; anything else waits for the user.
The tech-lead gate always waits for the user, whatever the tier.

- `slicing` builds a new screen from a design; the architect phase decides the component split and QA identifier placement.
- `team` is the full role flow, for features with a PRD, a tracker task or a design, or changes across more than one layer.
- A one-file or copy change stays in the main session without any role.

The step 2 order in `team` is fixed: product-manager, then design-refiner, then flutter-architect, then flutter-planner.
Acceptance criteria decide which states need a design; the design decides the widget split and QA identifier placement.

Each chain is a maximum list, not a list that always runs. The roster decides which roles actually run.

## Running phases

- Run each phase as a role agent, invoked with the role name exactly as its definition names it.
- Finish one phase completely before the next one starts.
- Consider compacting between phases (see `strategic-compact`); phase boundaries are the best compact points.

## Roster (required, before the first phase)

- Before the first phase, decide every role in the flow chain as RUN or SKIP, using only the role gate triggers below.
- Give every RUN role its active skills, using only the skill gate below.
- Show the roster in the step 0 block, next to the polished prompt, so the user confirms scope, roster and tier once.
- Run no role without a RUN line in the roster.
- Run no skill outside its role's active skills, even when the role definition itself names that skill.
- Non-team flows use the same role gate table for the roles in their chain.

```
ROSTER: <feature>   flow: <flow>   category: feat | fix | hotfix   tier: trivial | small | standard | large
Tier evidence: <files and layers, contracts, ambiguity>
Input: prompt | tracker <id> | PRD <path> | design <link | none>

Role                         Decision  Evidence
product-manager              RUN       no acceptance criteria file for this feature
design-refiner               SKIP      no design link in the prompt
flutter-architect            RUN       new state holder under lib/ui/orders/
flutter-planner              RUN       always
flutter-engineer             RUN       lib/ and test/ change
flutter-code-reviewer        RUN       code changes
qa-engineer                  RUN       new QA identifiers on order_list_page
flutter-security-reviewer    SKIP      no auth, storage, network, deep link or analytics in scope
tech-lead                    RUN       2 review roles run + acceptance criteria from product-manager

Active skills
product-manager:             idea-refine, user-story, deliver-acceptance-criteria
flutter-engineer:            flutter-expert, flutter-development, data-layer-tdd
qa-engineer:                 verification-loop, flutter-add-widget-test
```

- Evidence names a file, an input or a path that exists or does not exist.
- "Seems unnecessary" and "usually used" are not evidence; a decision without evidence is invalid.
- Save the roster to `.ai/plans/<feature>/handoff/00-roster.md`.
- Every role prompt carries that path and the role's own active skills line.

## Changing the roster mid-flow

- A role may be added without asking when a running role requests it under "For next phase" with evidence. Update the roster and tell the user in one line.
- Removing a role already marked RUN needs an explicit yes from the user.

## Role gate

These triggers are the only basis for RUN or SKIP.

| Role | RUN when | SKIP when |
|---|---|---|
| `product-manager` | no checkable acceptance criteria exist: no acceptance criteria file for the feature, and neither the prompt nor the tracker task states explicit criteria | valid acceptance criteria already exist; or the category is fix or hotfix and the prompt already holds the reproduction and the expected result |
| `design-refiner` | the user gave a design link in the prompt | there is no design link; the role never searches for designs itself, the UI follows comparable merged components, and the plan records "no design" |
| `flutter-architect` | a new screen, a new state holder, a new feature folder, or a change across two or more layers | the change stays inside an existing widget or state holder with no new state boundary; copy; a one-field mapper; a fix backed by an audit under `.ai/audits/` with a `file:line` trace and a proposed fix, adding no new state or screen (the planner reads the audit directly) |
| `flutter-planner` | always in the `team` flow | never |
| `flutter-engineer` | `lib/` or `test/` changes | never when code changes |
| `flutter-code-reviewer` | code changed | never when code changed |
| `qa-engineer` | QA identifiers are added, changed or removed; or product-manager acceptance criteria must map to tests; or the user asks for widget or integration tests | pure data-layer work whose tests flutter-engineer already wrote, with no new criteria and no identifier change |
| `flutter-security-reviewer` | scope touches any of: auth, token or session; local storage; network client or config; deep links; WebView; analytics properties; permissions; platform manifests or entitlements; logging of user data | none of those: UI slicing, copy, layout, mappers without sensitive data |
| `tech-lead` | two or more review roles run, or product-manager or design-refiner handoffs need a spec check | only flutter-code-reviewer runs and there is no PRD and no design spec; the code-reviewer verdict is the gate |

Selected packs add rows for their own roles.

## Skill gate

- For each RUN role, write its active skills in the roster and in the role prompt.
- An "always" skill runs every time the role runs.
- A "trigger" skill runs only when its condition holds, and the condition is written in the roster as evidence.

| Role | Always | Trigger |
|---|---|---|
| `product-manager` | `idea-refine` | `prd` when the feature has no PRD yet; `user-story` and `deliver-acceptance-criteria` when acceptance criteria are missing |
| `design-refiner` | the design-to-code skill of the selected design pack, loaded before the first design-context fetch | none |
| `flutter-architect` | none | the state-management patterns skill of the selected state pack, only when a state holder is new or its state changes; `codebase-design` only for a new module or seam (new feature folder, new repository) |
| `flutter-planner` | `spec-driven-development` | `source-driven-development` when the plan uses a Flutter or package API with no example in the repo yet |
| `flutter-engineer` | `flutter-expert`, `flutter-development` | `data-layer-tdd` when touching a mapper, repository, model or state holder; `flutter-fix-layout-issues` only on an overflow or unbounded-constraint error; `flutter-build-responsive-layout` only when acceptance criteria name tablet, orientation or breakpoints |
| `qa-engineer` | `verification-loop` | `qa-expert` when product-manager ran; `flutter-add-widget-test` when writing widget tests; `flutter-add-integration-test` only when the user asks for a journey test |
| `flutter-security-reviewer` | the OWASP Mobile Top 10 reference and the project security rules | the OWASP scanner only when scope touches storage, network or credentials; `security-and-hardening` only for WebView, deep link or URL input, or form validation |

- `flutter-code-reviewer` and `tech-lead` have no skills.
- A role not listed here has no skills, unless a selected pack adds a row.
- Every role closes its HANDOFF with "Skills used" and "Skills skipped" (or "none").
- A used skill outside the role's active skills is a process violation; tech-lead records it.

## Handoff

Every phase closes with this block, and the next phase starts by reading it:

```
HANDOFF: <phase name>

Done:
- <what actually changed, with file paths>

Files touched:
- lib/... [new|changed]

Decided:
- <decisions that bind the next phase>

Open:
- <what was left undone on purpose, and why>

Skills used:
- <name> - <what for, one phrase>

Skills skipped:
- <name from the active list that turned out unnecessary> - <why>

For next phase:
- <concrete instructions, not a summary>
```

- A handoff that summarizes without file paths is invalid. The next phase needs addresses, not a story.
- Save each handoff to `.ai/plans/<feature>/handoff/<role>.md`.
- Roles that may write save their own handoff. Read-only roles return it, and the main session saves it.
- A role starts with an empty context and reads files. The main session passes paths in the prompt, never copied content, and roles report paths, not file copies.
- A role that cannot follow an earlier decision writes it under "Open". The main session raises it to the user instead of choosing.

### Role prompt

The main session sends each role only this:

```
Feature: <folder name>
Base branch: <base>
Roster: .ai/plans/<feature>/handoff/00-roster.md
Active skills: <the role's list from the roster, or "none">
Read: <existing handoffs, PRD folder, design spec>
Design link: <design-refiner only, from the user>
Task: <one sentence for this role>
Close with HANDOFF (including Skills used / Skills skipped) and save it to .ai/plans/<feature>/handoff/<role>.md
```

- "Read" names only files that exist.
- A skipped role has no handoff. Receivers never wait for it and never invent it.

## Parallel rules

- Only the read-only reviewers run in parallel: flutter-code-reviewer, qa-engineer and flutter-security-reviewer. Start all three in one turn, then pass all three handoffs to tech-lead.
- Roles that write code run one at a time and alone in the working tree.
- A second task that writes code needs its own worktree (see `worktree`).
- product-manager, design-refiner and flutter-architect run one after another, because each reads the previous output.
- A role never starts another role. When it needs one, it writes the need under "For next phase", and the main session runs that role.
- Within one task, only these may run in the background without a worktree: analyze and test runs, the three read-only reviewers, and read-only research.

## Final output

```
ORCHESTRATE: <flow>
Roster: .ai/plans/<feature>/handoff/00-roster.md

Phase 1 <role>  [DONE]   <n> files   skills: <used list>
Phase 2 <role>  [SKIP]   <evidence from the roster>
Phase 3 <role>  [DONE]   <n> files   skills: none
Phase 4 <role>  [ADDED]  <evidence from the phase that asked for it>  <n> files

flutter analyze: <result, scoped>
flutter test:    <result, scoped>

Open:
- <list>
```

- The number of phase lines equals the number of roles in the flow chain.
- SKIP phases stay in the output with their reason, so a reviewer can tell the question was asked.
