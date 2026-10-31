---
target: workflow/worktree
order: 60
rules:
  - worktree.remove-only-clean-on-request
  - worktree.ios-pod-install
---
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
