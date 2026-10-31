---
target: workflow/checkpoint
order: 10
rules:
  - checkpoint.read-only-git
  - checkpoint.store-outside-git
  - checkpoint.no-restore
  - checkpoint.phase-boundaries
---
# checkpoint

Arguments: `create <name>` | `verify <name>` | `list` | `clear`.

## Hard limits

- Never run a git command that writes: no `commit`, `stash`, `tag`, `reset`, `checkout`, `apply` or `push`.
- Use only these git reads: `git rev-parse HEAD`, `git branch --show-current`, `git diff`, `git diff --stat`, `git status --porcelain`.
- Store each checkpoint as plain files (`<name>.patch`, `<name>.status`, `<name>.meta`) in a folder git ignores, for example `.ai/checkpoints/`.
- Nothing a checkpoint stores may enter the git object database.
- `git diff` sees tracked files only. Untracked files are listed in `<name>.status`, but their content is not saved.

## Restoring

- This workflow never restores anything. Restoring overwrites work in progress, and that is the user's decision.
- When the user wants to restore, give them the commands to run themselves: `git apply --check <dir>/<name>.patch` first, then `git apply <dir>/<name>.patch`.

## When to take one

Take checkpoints at phase boundaries, not every few minutes:

```
task start         -> checkpoint create <task>-start
data layer green   -> checkpoint create <task>-data-green
UI ready           -> checkpoint create <task>-ui-ready
before refactor    -> checkpoint create <task>-pre-refactor
after refactor     -> checkpoint verify <task>-pre-refactor
```

A checkpoint is a session aid. It is not a backup and it does not replace a commit.
When the work must be safe, recommend a commit to the user.
