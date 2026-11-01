---
target: workflow/worktree
order: 60
rules:
  - worktree.ios-pod-install
---
## Platform notes

- An iOS run in a worktree needs its own `pod install`.
- Analyze and unit tests need only `flutter pub get`.
