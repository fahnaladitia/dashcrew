# P1 stage 8: golden fixtures

Tier: large (adds golden fixtures). Test-only plus fixtures; no change to library code expected.

## Goal

Any change to what `sync` writes shows up as a reviewable diff in `test/golden/`, instead of slipping through unit tests that only assert fragments.

## Fixture

`test/golden/sample-bloc-getx/`, a synthetic project:

```
project.yaml     profile: bloc, get_it, getx, dio, hive, manual-fakes, semantics-identifier, posthog
expected/        every file sync writes, at its real path
  AGENTS.md, CLAUDE.md, .gitignore, .mcp.json
  .ai/guard.json, .ai/lock.json
  .claude/settings.json, .claude/agents/*.md, .claude/skills/*/SKILL.md
```

The profile also sets `project.flavors`, `project.local_packages`, `language` and `policy` values that differ from the defaults, so placeholders and policy switches are exercised. The guard is not compiled (`compileGuard: false`), so the lock has no `guard_source` and no binary lands in `expected/`.

## Test

`test/golden/golden_test.dart`:

1. Copy `project.yaml` to a temp project as `.ai/project.yaml`, run `Sync.run()`.
2. The set of files under the temp project (minus `.ai/project.yaml`) equals the set under `expected/`; each file is byte-equal. A failure names the first differing files and how to update.
3. A second `sync` writes nothing.
4. `doctor` on the result: no `fail` except `guard-binary` (not compiled).

Updating: `DASHCREW_UPDATE_GOLDENS=1 dart test test/golden` rewrites `expected/` from the current output, then the diff is reviewed in git like any other change. The test never rewrites goldens without that variable.

Output must be identical on every supported SDK; CI runs the golden test on the minimum SDK and stable, which proves it.

## Not doing

- A second fixture per stack option. Unit tests already render every option; one full-stack golden covers layout and merge behavior.
- Codex output (P3).
