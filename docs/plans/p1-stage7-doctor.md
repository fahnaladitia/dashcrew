# P1 stage 7: doctor

Tier: standard. New module `lib/src/doctor/`, a small refactor in `lib/src/sync/`, one CLI command, tests.

## Goal

`dashcrew doctor` tells a user whether the toolkit in a project is installed, untouched and current, without writing anything. It is the place where a broken install shows up, since the guard itself fails open (exit 0) on any error.

## Checks

Each check reports `ok`, `warn`, `fail` or `skip` with one line of detail; `skip` means a check it depends on failed (no lock, invalid profile). Exit code 1 when any check fails, else 0.

| Id | Check | Fail / warn when |
|---|---|---|
| `profile` | `.ai/project.yaml` exists and parses against the catalog | fail: missing or invalid |
| `lock` | `.ai/lock.json` exists, schema 1 | fail: missing (never synced) or unknown schema |
| `drift` | every locked file exists and its hash matches the lock | fail: file missing or edited by hand (lists paths) |
| `stale` | what `sync` would render now equals what the lock records | warn: profile or dashcrew changed since the last sync, run `dashcrew sync` |
| `guard-policy` | `.ai/guard.json` parses as `GuardPolicy` | fail: missing or invalid |
| `guard-binary` | `.ai/bin/guard` exists, runs, and blocks a known event (`git push`) with exit 2 | fail: missing, crashes, or allows the event; warn: guard sources changed since it was compiled |
| `hook` | `.claude/settings.json` has the PreToolUse entry with the guard matcher and command | fail: missing or different |
| `claude-md` | `CLAUDE.md` contains `@AGENTS.md` | fail: missing import |
| `mcp` | `.mcp.json` defines every MCP server the adapter wants | warn: missing or defined differently |
| `skills` | every skill named in generated files exists | fail: a dashcrew skill is missing from `.claude/skills/`; warn: an external skill is not found in `.claude/skills/` or the user skills dir |
| `agents-md-size` | AGENTS.md <= 32 KiB | warn: over the limit some agents read |
| `gitignore` | `.gitignore` lists `.ai/bin/` | warn: missing |

Skill references are found with the two forms the catalog uses: `` `name` skill `` and `` skill `name` ``. External skills come from outside the catalog (plugins, user installs), so doctor cannot prove they are absent; that is a warning, not a failure. The user skills dir is `$CLAUDE_CONFIG_DIR/skills`, else `~/.claude/skills`.

## Refactor

`Sync.run` builds the file map and writes it in one method. Split the pure part out as `Sync.render()` returning `{path: content}` (AGENTS.md, guard.json, CLAUDE.md when owned, adapter files) so `doctor` compares the same output `sync` would write. No behavior change for `sync`; existing sync tests cover it.

The guard source hash moves into a static helper shared by `sync` and `doctor`.

## CLI

`dashcrew doctor [--project <dir>]`. Output, one line per check:

```
ok    profile         .ai/project.yaml valid
fail  drift           edited by hand: .claude/agents/planner.md
warn  stale           run dashcrew sync
```

## Tests (written first)

`test/unit/doctor_test.dart`, on a temp project synced with a test catalog:

- fresh sync: every check ok, exit 0
- locked file edited by hand: `drift` fail
- locked file deleted: `drift` fail
- profile changed after sync: `stale` warn, no fail
- guard.json removed: `guard-policy` fail
- hook entry removed from settings: `hook` fail
- CLAUDE.md without `@AGENTS.md`: `claude-md` fail
- reference to a missing catalog skill: `skills` fail; to an unknown external skill: `skills` warn
- no lock: `lock` fail and the checks that need it are skipped, not crashed

The guard binary smoke test compiles once, in the fresh-sync test; the other tests run without a compiled guard and assert `guard-binary` fails with "missing". `test/unit/cli_test.dart` checks that `dashcrew doctor` exits 1 on a project that was never synced.

## Not doing

- Fixing anything (`--fix`). Doctor reports; `sync` repairs.
- Codex checks (P3).
- Checking that an agent actually loads the files (that is the stage 8 dry run).
