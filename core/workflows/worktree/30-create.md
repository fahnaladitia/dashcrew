---
target: workflow/worktree
order: 30
rules:
  - worktree.create-validation
  - worktree.report-script-output
---
## create

```
<worktree helper> create <branch> <base> [--name <dir>] [--no-pub] [--detached [--prompt-file <file>]]
```

Before running it:

- The user must name the base, and it must exist. If it is missing, ask; never default to `main` or to the current branch.
- The helper refuses a branch or a folder that already exists.
- The branch name follows the branching rules. If the user's name breaks them, propose 3 compliant names and wait.
- The current working tree does not need to be clean; the new worktree does not touch it.

After it runs, report its output as is: path, branch, "no upstream", the `git status -sb` line, the `pub get` result, and for `--detached` the session name and the command to attach to it.
If `pub get` failed, say so and give the command to rerun in the worktree. Never retry it silently.

## link

`<worktree helper> link <name>` adds the missing toolkit links to an existing worktree. It is idempotent and also removes an upstream the branch should not have.

## list

`<worktree helper> list` shows each worktree with its upstream, dirty line count, toolkit link state and detached session. Show the output as is; the upstream must be empty for a branch never pushed.
