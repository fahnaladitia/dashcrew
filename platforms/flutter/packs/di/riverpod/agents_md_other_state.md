---
target: agents_md
order: 311
rules: [di.riverpod-with-other-state]
when: { stack.state.use: [bloc, provider, none, other] }
---
- State is not Riverpod here: the screen that creates a state holder is a `ConsumerWidget` and hands it what it needs from `ref.read(...)`.
