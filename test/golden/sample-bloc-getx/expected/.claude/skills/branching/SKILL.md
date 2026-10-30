---
name: branching
description: "Create a task branch that never tracks its base, and keep upstreams correct."
when_to_use: "When creating a branch at step 1, or when a branch tracks the wrong upstream."
---

# Branching

## Create

1. Propose 3 names, for example `feature/order-history-filter`, `bugfix/order-total-rounding`, `hotfix/profile-page-null-avatar`.
2. Ask for the base. Features and fixes usually fork from the active development branch, hotfixes from the release or main branch, but the user decides.
3. `git status`: if the tree is dirty, show the changes and ask before switching.
4. `git checkout --no-track -b <name> <base>` (or `git branch --no-track <name> <base>`). `git checkout -b --no-track ...` is rejected as an invalid branch name.
5. `git branch --show-current` prints the new name.
6. `git rev-parse --abbrev-ref @{u}` must fail, and `git status -sb` must print the branch name alone: no `...origin/...`, no `[ahead N]`.

## Why no tracking

With git's default `branch.autoSetupMerge`, a branch created from a remote-tracking ref silently inherits that ref as its upstream. A plain `git pull` then merges the shared branch into the task branch, `git status` measures against the wrong ref, and only `push.default` stands between the work and a push onto the shared branch.

## Wrong upstream

- Symptom: `git status -sb` shows `## feat/x...origin/<shared-branch> [ahead 1]`.
- Fix: `git branch --unset-upstream`, without asking. It is local config, reversible, and touches no commit or remote. A read-only role reports it to the main session instead.
- Never "fix" it by pointing the upstream at the base branch.

## Upstream and pull

- The user sets the upstream exactly once, on the first push: `git push -u origin <branch>`. Never earlier, and never to a branch with a different name.
- Never run or suggest `git pull` on a branch, or a base, whose upstream you have not verified.
