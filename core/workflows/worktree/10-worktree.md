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
# /worktree

This workflow sits outside the chain. It only prepares one worktree per parallel task; each worktree session then runs the chain itself.

## Principles

- One task, one branch, one worktree.
- Never check out a second branch in a working tree another task is using.
- A new branch never tracks its base: no upstream until the user's first push.

## The worktree helper

- Create worktrees only through the worktree helper, never with a hand-typed `git worktree add`.
- On `create`, the helper:
  - adds the worktree next to the repo with `git worktree add --no-track -b <branch> <path> <base>`
  - links the toolkit files git ignores (AGENTS.md, agent folders, `{{paths.plans}}`, `{{paths.pr}}`) from the main checkout, and adds those links to the local git exclude file
  - shares the memory folder `{{paths.memory}}` with the main checkout
  - records the base branch for later checks
  - runs `flutter pub get` in the worktree
- The helper runs only these writing git operations: `git worktree add --no-track -b`, `git branch --unset-upstream`, `git worktree remove`, and `git fetch origin <base>` during `watch`.
- The helper never commits, pushes, rebases or deletes a branch.

## Modes

| Mode | Who works | Where the user answers gates | Tier |
|---|---|---|---|
| `worktree` | a separate agent session the user opens in the worktree | that session | any |
| `detached` | a background agent session the helper starts with an opening prompt file | that session, after attaching to it | any |
| `inline` | this session, through role agents pointed at the worktree path | this chat | `trivial` or `small` only, at most two |
| `current` | this session, in the current working tree, branch created by chain step 1 | this chat | any, at most one task |
| `wait <parent>` | not created yet | - | - |

- A message from another session is never user approval.
- This session never answers a gate that belongs to a `worktree` or `detached` session.
- Each worktree session runs the full chain itself: checkpoint, plan, verify and its own PR description.
- Plans and PR descriptions stay shared from the main checkout, so `{{paths.plans}}` and `{{paths.pr}}` hold every task in one place.
