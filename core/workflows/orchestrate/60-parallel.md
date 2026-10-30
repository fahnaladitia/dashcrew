---
target: workflow/orchestrate
order: 60
rules:
  - parallel.read-only-roles-only
  - parallel.writers-alone
  - parallel.pm-designer-architect-sequential
  - parallel.no-nested-subagents
  - orchestrate.background-allowed
---
## Parallel rules

- Only the read-only reviewers run in parallel: flutter-code-reviewer, qa-engineer and flutter-security-reviewer. Start all three in one turn, then pass all three handoffs to tech-lead.
- Roles that write code run one at a time and alone in the working tree.
- A second task that writes code needs its own worktree (see `/worktree`).
- product-manager, design-refiner and flutter-architect run one after another, because each reads the previous output.
- A role never starts another role. When it needs one, it writes the need under "For next phase", and the main session runs that role.
- Within one task, only these may run in the background without a worktree: analyze and test runs, the three read-only reviewers, and read-only research.
