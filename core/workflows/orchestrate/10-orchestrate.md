---
target: workflow/orchestrate
order: 10
rules:
  - worktree.multi-task-to-backlog
  - worktree.no-flow-for-other-worktree
  - orchestrate.flows
  - orchestrate.team-order
  - roster.flow-chain-is-maximum
  - orchestrate.agent-name-exact
  - orchestrate.phase-complete-first
---
# /orchestrate

Arguments: a flow name (`team`, `feature`, `bugfix`, `refactor`, `security`, `slicing`), then the task description.

`/orchestrate` replaces `/plan` through `/code-review`. Checkpoint, verify, the PR description and the PR question stay with the main session.

## More than one task

- When the description holds two or more tasks (separated by `;`, new lines or a numbered list), pick no flow and run no role.
- Forward the description as is to `/worktree backlog` and follow it to the end: matrix, base question, confirmation, one worktree per independent task.
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

{{#policy.tier_gates}}
Plan gate: verdict GO on tier `trivial` or `small` continues; anything else waits for the user.
{{/policy.tier_gates}}
{{^policy.tier_gates}}
Plan gate: the user confirms the plan before implementation.
{{/policy.tier_gates}}
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
