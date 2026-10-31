---
target: workflow/worktree
order: 20
rules:
  - worktree.backlog-matrix
  - worktree.write-surface-evidence
  - worktree.parallel-only-disjoint
  - worktree.base-asked-once
  - worktree.opening-prompt
---
## backlog

Use it when one input holds two or more tasks, directly or forwarded by `/orchestrate`.
It replaces step 0 for multi-task input: the matrix first, code later.

1. Split the input into tasks, one row each.
2. Give each task a write surface backed by repo evidence, not a guess. Include pages that use a changed model or state, and navigation targets, not only the folder of the bug:

```bash
ls lib/ui/ | grep -i "<feature>"
grep -rln "<concept>" lib/ --include="*.dart" | head
```

3. Fill `depends_on` when task B needs code task A has yet to write, or when their write surfaces share even one folder.
4. Two tasks run in parallel only when `depends_on` is empty and their write surfaces do not overlap by a single folder. Overlap means serial.
5. Give each task a tier and a proposed mode. `inline` only for tier `trivial` or `small`, with a disjoint write surface, and at most two inline rows. Otherwise `worktree`.
6. Propose a branch per task following the branching rules in AGENTS.md.
7. Ask the base branch once for all independent tasks; never guess it. A dependent task uses its parent's branch as base and waits until that branch exists.
8. Show the matrix and stop before touching any task:

```
BACKLOG
Task              tier      depends_on  Write surface                       Branch      Base            Mode
A <summary>       standard  -           lib/ui/orders/, lib/data/mappers/   <proposal>  <ask the user>  worktree
B <summary>       small     -           lib/ui/profile/                     <proposal>  <ask the user>  inline
C <summary>       small     A           lib/ui/orders/                      <proposal>  <branch of A>   wait A

Commands to run after confirmation:
  dashcrew worktree create <branch A> <base>
  dashcrew worktree create <branch B> <base>
  /worktree watch

AWAITING CONFIRMATION: are base, branch names, modes and this split correct?
```

9. After a yes, for each independent task in order:
   - `worktree`: run `create`, then write an opening prompt holding the task, its tracker id when there is one, the contract, the write surfaces it must not touch (they belong to other tasks), the merge order, and the line "the branch already exists, skip step 1". Give the user the command to open an agent session in the new path with that prompt.
   - `inline`: run `create`, then run that task's chain from here (see Inline).
   - A dependent task is not created now; record that it waits for its parent branch.
10. Start `watch` once for all new worktrees.
