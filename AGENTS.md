# dashcrew - contributor rules for AI agents

dashcrew is a Dart CLI that renders an AI coding toolkit for Flutter projects.
These rules apply to work on this repository, not to the projects dashcrew
generates files for.

## Dynamic chain

Size every task first, one line with evidence, then run the matching chain.

| Tier | Signal | Chain |
|---|---|---|
| trivial | one file, a few lines, one layer, no behavior change (typo, copy, doc) | edit -> `dart analyze` on the file -> one-line summary |
| small | 1-2 files, existing module, behavior obvious after reading the code | test first -> implement -> `dart analyze` + scoped `dart test` -> summary |
| standard | 3+ files or 2+ modules (profile, catalog, render, adapters, guard, lock, doctor), or any change to generated output | plan in `docs/plans/<slug>.md` -> wait for confirmation -> test first -> implement -> review the diff -> verify -> summary |
| large | new adapter, new pack family, schema version bump, output layout change | standard, plus golden fixtures updated and reviewed before merge |

Doubt between two tiers means the higher one.

## Git

- The maintainer owns commits, pushes and merges. Agents stage or diff when asked, never commit or push.
- No AI attribution in commit messages, PR bodies or comments.

## Code

- Dart, `package:lints/recommended`. Strict null safety, no `dynamic` in new code.
- Library code in `lib/src/`, one concern per folder. `bin/` stays thin.
- Tests live in `test/`; generated output is checked with golden fixtures in `test/golden/`.
- Neutral sources (`core/`, `packs/`) are Markdown or YAML with front matter. Keep them agent-neutral: agent-specific fields are added by adapters, never written in `core/`.
- Generated files carry a dashcrew marker and are never edited by hand.

## Verify

```bash
dart analyze
dart test
```

Scope `dart test` to the touched files for trivial and small tiers.
