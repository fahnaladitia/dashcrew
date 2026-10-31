---
name: worktree
description: "Run two or more tasks at once - build a backlog matrix of dependencies and write surfaces, then give each parallel task its own branch and worktree."
argument-hint: "backlog <task A>; <task B> | create <branch> <base> [--name <dir>] [--no-pub] | link <name> | list | watch [<name>...] | remove <name>"
---

# /worktree

This workflow sits outside the chain. It only prepares one worktree per parallel task; each worktree session then runs the chain itself.

## Principles

- One task, one branch, one worktree.
- Never check out a second branch in a working tree another task is using.
- A new branch never tracks its base: no upstream until the user's first push.

## `dashcrew worktree`

- Create worktrees only with `dashcrew worktree create`, never with a hand-typed `git worktree add`.
- `create`:
  - adds the worktree at `<repo>.worktrees/<name>` with `git worktree add --no-track -b <branch> <path> <base>`
  - links the toolkit files the main checkout ignores (AGENTS.md, agent folders, `.ai/plans`, `.ai/pr`), and adds those links to the git exclude file; a file the main checkout does not ignore is never linked
  - shares the memory folder `.ai/memory` with the main checkout
  - records the base branch in `branch.<branch>.dashcrew-base`
  - runs `flutter pub get` in the worktree (`fvm flutter` when the project has `.fvmrc`)
- Its only writing git operations: `git worktree add --no-track -b`, `git branch --unset-upstream`, `git config branch.<branch>.dashcrew-base`, `git worktree remove`, and `git fetch origin <base>` during `watch`.
- It never commits, pushes, rebases or deletes a branch, and it starts no agent session.

## Modes

| Mode | Who works | Where the user answers gates | Tier |
|---|---|---|---|
| `worktree` | a separate agent session the user opens in the worktree | that session | any |
| `inline` | this session, through role agents pointed at the worktree path | this chat | `trivial` or `small` only, at most two |
| `current` | this session, in the current working tree, branch created by chain step 1 | this chat | any, at most one task |
| `wait <parent>` | not created yet | - | - |

- A message from another session is never user approval.
- This session never answers a gate that belongs to a `worktree` session.
- Each worktree session runs the full chain itself: checkpoint, plan, verify and its own PR description.
- Plans and PR descriptions stay shared from the main checkout, so `.ai/plans` and `.ai/pr` hold every task in one place.

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

## create

```
dashcrew worktree create <branch> <base> [--name <dir>] [--no-pub]
```

Before running it:

- The user must name the base, and it must exist. If it is missing, ask; never default to `main` or to the current branch.
- It refuses a branch or a folder that already exists, and a base that does not exist.
- The branch name follows the branching rules. If the user's name breaks them, propose 3 compliant names and wait.
- The current working tree does not need to be clean; the new worktree does not touch it.

After it runs, report its output as is: path, branch and base, `upstream: none`, the `git status -sb` line, the links and the `pub get` result.
If `pub get` failed, say so and give the command to rerun in the worktree. Never retry it silently.

## link

`dashcrew worktree link <name>` adds the missing toolkit links to an existing worktree. It is idempotent and also removes an upstream the branch should not have.

## list

`dashcrew worktree list` shows each worktree with its branch, base, upstream, dirty line count and missing toolkit links. Show the output as is; the upstream must be empty for a branch never pushed.

## Inline

A `trivial` or `small` task that this session runs in another worktree, without a new session.

- The full chain still runs. Only the place of work changes. This session still owns checkpoint, scoped `/verify`, the PR description and the PR question, per task.
- Every role prompt carries `Worktree: <absolute path>` and these rules:
  - prefix every shell command with `cd <path> &&` in the same command, because the working directory may persist between calls
  - run git as `git -C <path>`
  - edit only under `<path>/lib/` and `<path>/test/`
  - write plans to `.ai/plans` and PR descriptions to `.ai/pr` in the main checkout, never through the worktree's links
- Prefix every gate shown to the user with `[<task>]`.
- Show one gate at a time; the next task's gate waits until the current one is answered.
- After each phase, run `git -C <path> status --short` and `git status --short` in the main checkout.
- Report and stop on any stray change in the main checkout. Never move it silently.

## watch

```
/worktree watch [<name>...]
```

- Start exactly one `worktree-monitor` role in the background, with the main repo path and the worktree names, then return to the user. The monitor runs `dashcrew worktree watch`.
- Never start a second monitor for the same set of worktrees; check first whether one is already running.
- Without names, it watches every worktree with a recorded base, until each one is merged or removed.
- Relay the monitor's messages to the user in one or two lines, as is.
- Never act or run git on behalf of a worker session.
- A monitor that reports it is waiting for events is normal. Never stop its background watcher.
- The monitor stops when it reports DONE, or when the user asks to stop it.

## Merging

- Merge in dependency order.
- After each merge, check every open worktree against the updated base with `git merge-tree --write-tree origin/<base> <branch>`.
- Rerun scoped `/verify` in that worktree before its PR is opened.
- After a parent merges, moving a dependent branch onto the base is the user's call.

## remove

```
dashcrew worktree remove <name>
```

- Remove a worktree only when the user asks for that worktree.
- It refuses a dirty worktree. Show the reason and stop.
- Committing, stashing and closing sessions stay the user's decisions.
- The branch is never deleted, and never suggest deleting it.

## Platform notes

- An iOS run in a worktree needs its own `pod install`.
- Analyze and unit tests need only `flutter pub get`.
