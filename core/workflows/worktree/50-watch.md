---
target: workflow/worktree
order: 50
rules:
  - worktree.watch-single-monitor
  - worktree.merge-tree-recheck
---
## watch

```
/worktree watch [<name>...]
```

- Start exactly one `worktree-monitor` role in the background, with the main repo path and the worktree names, then return to the user.
- Never start a second monitor for the same set of worktrees; check first whether one is already running.
- Without names, the monitor watches every worktree with a recorded base whose PR is not merged.
- Relay the monitor's messages to the user in one or two lines, as is.
- Never act or run git on behalf of a worker session.
- A monitor that reports it is waiting for events is normal. Never stop its background watcher.
- The monitor stops when it reports DONE, or when the user asks to stop it.

## Merging

- Merge in dependency order.
- After each merge, check every open worktree against the updated base with `git merge-tree --write-tree origin/<base> <branch>`.
- Rerun scoped `/verify` in that worktree before its PR is opened.
- After a parent merges, moving a dependent branch onto the base is the user's call.
