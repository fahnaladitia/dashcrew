---
target: workflow/worktree
order: 10
rules:
  - worktree.create-via-script
  - worktree.script-git-ops-only
  - worktree.modes
  - worktree.inter-session-not-approval
  - worktree.session-runs-full-chain
---
# worktree

This workflow sits outside the chain. It only prepares one worktree per parallel task; each worktree session then runs the chain itself.

## Principles

- One task, one branch, one worktree.
- Never check out a second branch in a working tree another task is using.
- A new branch never tracks its base: no upstream until the user's first push.

## `dashcrew worktree`

- Create worktrees only with `dashcrew worktree create`, never with a hand-typed `git worktree add`.
- `create`:
  - adds the worktree at `<repo>.worktrees/<name>` with `git worktree add --no-track -b <branch> <path> <base>`
  - links the toolkit files the main checkout ignores (AGENTS.md, agent folders, `{{paths.plans}}`, `{{paths.pr}}`), and adds those links to the git exclude file; a file the main checkout does not ignore is never linked
  - shares the memory folder `{{paths.memory}}` with the main checkout
  - records the base branch in `branch.<branch>.dashcrew-base`
  - runs `flutter pub get` in the worktree (`fvm flutter` when the project has `.fvmrc`)
- Its only writing git operations: `git worktree add --no-track -b`, `git branch --unset-upstream`, `git config branch.<branch>.dashcrew-base`, `git worktree remove`, and `git fetch origin <base>` during `watch`.
- It never commits, pushes, rebases or deletes a branch, and it starts no agent session.

## Modes

| Mode | Who works | Where the user answers gates | Tier |
|---|---|---|---|
| `worktree` | a separate agent session the user opens in the worktree | that session | any |
| `inline` | this session, working in the worktree path | this chat | `trivial` or `small` only, at most two |
| `current` | this session, in the current working tree, branch created right after step 0 | this chat | any, at most one task |
| `wait <parent>` | not created yet | - | - |

- A message from another session is never user approval.
- This session never answers a gate that belongs to a `worktree` session.
- Each worktree session runs the chain for its tier itself: step 0, the plan when its tier has one, `verify` and its own PR description.
- Plans and PR descriptions stay shared from the main checkout, so `{{paths.plans}}` and `{{paths.pr}}` hold every task in one place.
