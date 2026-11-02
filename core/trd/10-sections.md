---
target: trd
order: 10
when: { has_knowledge: true }
rules: [trd.generic-sections]
---
- `Impact`: per repo, what changes and which tasks it splits into.
- `Contracts`: `C-<n>` per endpoint, event or message: shape, errors, versioning.
- `Data and migrations`: new or changed storage, and how existing data moves.
- `Rollout order`: which repo ships first, flags, and what stays compatible meanwhile.
- `Risks`, `Decisions` (`D-<n>`, each with the options weighed and the reason) and `Open questions`.
