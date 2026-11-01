---
target: workflow/worktree
order: 50
rules:
  - worktree.watch-background-shell
  - worktree.watch-single
  - worktree.watch-event-filter
  - worktree.watch-message-format
  - worktree.watch-stop
  - worktree.merge-tree-recheck
---
## watch

```
worktree watch [<name>...]
```

- Run `dashcrew worktree watch [<name>...] --state-dir <dir>` once as a background shell in this session, with a fixed state directory so a restart does not repeat old events, then return to the user.
- Never start a second watcher for the same worktrees; check first whether one is already running. A watcher waiting for events is normal; never stop it for that.
- Without names, it watches every worktree with a recorded base, until each one is merged or removed.
- It prints one line per event: `commit`, `pushed`, `pr`, `base-moved`, `merge-tree`, `overlap`, `error`, `done`. Relay to the user only:
  - the first commit (`commit`) or first push (`pushed`) in a worktree
  - a new PR, a merged PR, or a PR that is conflicting (`pr`)
  - file overlap between worktrees that grows (`overlap`; it only prints growth)
  - a merge-tree result, as is (`merge-tree`)
  - the `done` event
- Keep every other event to yourself; summarise it only when the user asks.
- Relay in one to three lines, starting with the branch name, with no suggested git action:

```
fix/order-total-rounding: PR #12 MERGED. merge-tree feat/profile-page vs origin/main: CLEAN
feat/profile-page: overlap with fix/order-total-rounding on src/orders/order_total
```

- When a long-running background shell is not available, run `dashcrew worktree watch --once` with the same state directory every minute instead, with the same filter.
- Stop on `done`, when the user asks, or after two consecutive `error` lines; report the last error first.
- Never act, run git or message another session on behalf of a worker session.

## Merging

- Merge in dependency order.
- After each merge, check every open worktree against the updated base with `git merge-tree --write-tree origin/<base> <branch>`.
- Rerun scoped `verify` in that worktree before its PR is opened.
- After a parent merges, moving a dependent branch onto the base is the user's call.
