---
target: workflow/worktree
order: 30
rules:
  - worktree.create-validation
  - worktree.report-script-output
---
## create

```
dashcrew worktree create <branch> <base> [--name <dir>] [--no-pub]
```

Before running it:

- The user must name the base, and it must exist. If it is missing, ask; never default to `main` or to the current branch.
- It refuses a branch or a folder that already exists, and a base that does not exist.
- The branch name follows the branching rules. If the user's name breaks them, propose 3 compliant names and wait.
- The current working tree does not need to be clean; the new worktree does not touch it.

After it runs, report its output as is: path, branch and base, `upstream: none`, the `git status -sb` line, the links and the `deps` result.
If `deps` failed, say so and give the command to rerun in the worktree. Never retry it silently.

## link

`dashcrew worktree link <name>` adds the missing toolkit links to an existing worktree. It is idempotent and also removes an upstream the branch should not have.

## list

`dashcrew worktree list` shows each worktree with its branch, base, upstream, dirty line count and missing toolkit links. Show the output as is; the upstream must be empty for a branch never pushed.
