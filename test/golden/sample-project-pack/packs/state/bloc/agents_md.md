---
target: agents_md
order: 300
rules: [team.cubit-only, team.cubit-per-screen]
---
## State: Cubit only

- Use a Cubit, never a Bloc with events. One Cubit per screen in `{{pack.state.feature_dir}}cubit/`.
- A Cubit never calls another Cubit; share data through a repository.
