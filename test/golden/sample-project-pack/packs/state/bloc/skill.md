---
target: skill/bloc-patterns
order: 100
---
# Cubit patterns

- States extend `Equatable`; a screen with more than one async load uses one status enum, never several booleans.
- Scope rebuilds with `BlocSelector` or `buildWhen`.
