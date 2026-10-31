# P2 plan: init, project packs, stack fallback, flavors, worktree helper

Tier: large (new commands, catalog layout change, profile schema additions). P1 is done: `sync`, `doctor`, guard, Claude Code adapter, golden `sample-bloc-getx`.

## Goal

A team adopts dashcrew without editing dashcrew: `init` reads the project and proposes a profile, project-only rules live in the project as packs, a library dashcrew has no pack for still gets sensible rules, and the worktree workflow has the helper it already describes.

## Not doing (P2)

- New built-in packs (riverpod, go_router, mocktail). A project that needs one writes a project pack; a pack moves into dashcrew only when two projects share it.
- Codex adapter (P3), plugin/marketplace and pub.dev publish (P4).
- tmux or detached sessions in the worktree helper; it prepares worktrees, the user opens the sessions.

## Stage 1: `stack.<family>: other`

Every family accepts `other` with a required `name`:

```yaml
stack:
  state: { use: other, name: mobx }
```

- Profile: `other` is a valid option for every known family; missing or empty `name` is a profile error.
- Context: `stack_other` is a list of `{family, name}` sorted by family, so templates can loop.
- Catalog: `core/rules/stack-other.md` (`when` on a non-empty `stack_other`) renders one short block per entry: use the library's official docs and the patterns already in the codebase; read existing code before adding a new pattern; never introduce a second library for the same family.
- Guard: nothing family-specific is compiled for `other`.

Tests: profile accepts `other` with name, rejects without; render contains the block per family; AGENTS.md stays under 32 KiB with every family set to `other`.

## Stage 2: project packs

Layout, same format as built-in packs:

```
.ai/packs/<family>/<option>/pack.yaml
.ai/packs/<family>/<option>/*.md
.ai/packs/<family>/<option>/skills/<name>.yaml
```

- `Catalog.load(root)` stays; `catalog.withProjectPacks(projectDir)` returns a catalog where project packs are added. A project option with the same family and option as a built-in one replaces it (whole pack, no merge). A new family is allowed (e.g. `architecture/clean`) and becomes a valid `stack` key.
- `sync`, `doctor` and the guard compiler use the overlaid catalog. Profile validation uses its `knownOptions`.
- Lock: `packs` records `{family/option: source}` where source is `builtin` or `project`, so `doctor` can say which one rendered.
- Fragment `source` paths for project packs are relative to the project (`.ai/packs/...`), so render errors point at the project file.
- `doctor`: new check `project-packs` (fail on a pack that does not parse; warn on a project pack that is not selected by the profile).
- `docs/catalog-format.md`: section "Project packs".

Tests: overlay adds a family, replaces a built-in option, unknown family still rejected without a project pack, `stale` turns warn after editing a project pack, golden fixture `sample-project-pack` (synthetic: one replaced option, one new family).

## Stage 3: flavors in AGENTS.md

`project.flavors` stays a map in the profile. Context adds `build.flavors`, a list of `{name, entrypoint}` sorted by name, and `build.has_flavors`.

- New fragment in `core/rules/` ("Build and run"): one line per flavor, `flutter run --flavor <name> -t <entrypoint>`, plus the build wrapper line when set. This is the list `build-fix` already points at.
- `docs/catalog-format.md`: `build.flavors`, `build.has_flavors`.

Tests: render lists every flavor in order; no section without flavors; golden `sample-bloc-getx` updated (its profile already has two flavors).

## Stage 4: `dashcrew init`

```
dashcrew init [--project <dir>] [--yes] [--agents claude-code] [--force]
```

Reads, never guesses beyond it:

| Source | Fills |
|---|---|
| `pubspec.yaml` dependencies | `stack` (table below) |
| `pubspec.yaml` path dependencies | `project.local_packages` |
| `lib/main_<flavor>.dart` files | `project.flavors` |
| `git branch -r` (`origin/main`, `origin/master`, `origin/develop`, `origin/dev`) | `project.base_branches` |

Detection table (dependency -> family: option): `flutter_bloc`/`bloc` -> state: bloc; `get_it` -> di: get_it; `get` -> router: getx; `dio` -> network: dio; `hive`/`hive_ce`/`hive_flutter` -> storage: hive; `posthog_flutter` -> analytics: posthog. A known family whose library is found but has no pack -> `other` with the package name (`riverpod`, `flutter_riverpod`, `provider`, `mobx` -> state; `go_router`, `auto_route` -> router; `http`, `chopper` -> network; `isar`, `drift`, `sqflite` -> storage; `mocktail`, `mockito` -> test). Two candidates for one family: ask (or, with `--yes`, pick neither and say so).

- Interactive by default: prints the detected profile with the evidence per line, asks to confirm or edit each family. `--yes` writes the detected profile as is.
- Writes `.ai/project.yaml` only, then prints `next: dashcrew sync`. Refuses when the file exists, unless `--force`.
- The table lives in `lib/src/init/detect.dart` as data, so adding a library is one line plus a test.

Tests: detection per table row from fixture pubspecs, path deps, flavors from files, two-candidate conflict, `--yes` output equals a golden `project.yaml`, refusal without `--force`.

## Stage 5: `dashcrew worktree`

The helper the worktree workflow names. Subcommands:

```
dashcrew worktree create <branch> <base>
dashcrew worktree list
dashcrew worktree remove <name>
dashcrew worktree link <name>
dashcrew worktree watch [<name>...] [--once] [--state-dir <dir>]
```

- `create`: worktree at `<repo>.worktrees/<branch with / as ->`, `git worktree add --no-track -b <branch> <path> <base>`, verify no upstream, link the git-ignored toolkit files from the main checkout (AGENTS.md, CLAUDE.md, `.claude/`, `.ai/`, `{{paths.plans}}`, `{{paths.pr}}`) and add them to the worktree's `info/exclude`, share `{{paths.memory}}`, record the base in `git config branch.<branch>.dashcrew-base`, run `flutter pub get` (`fvm flutter` when `.fvmrc` exists).
- `link`: redo the links for an existing worktree. `list`: name, branch, base, upstream, dirty. `remove`: refuses a dirty worktree; never deletes the branch.
- `watch`: one line per event (pushed, PR opened or merged via `gh` when present, base moved, merge conflict with the new base via `git merge-tree --write-tree`), state kept in `--state-dir` so a re-run does not repeat events; `--once` for one pass; ends with a `done` event when every watched worktree is merged or removed.
- Writing git operations are limited to the ones the workflow lists; the guard is unchanged (none of them is blocked).
- `core/workflows/worktree/*.md` and the `worktree-monitor` role name `dashcrew worktree` instead of "the worktree helper".

Tests: each subcommand against a temp git repo with a bare origin (create links and no upstream, remove refuses dirty, watch emits pushed then base-moved once, `--once` exits).

## Order and gates

1 -> 2 -> 3 -> 4 -> 5. Stage 4 uses 1 (fallback) and 2 (a project family is a valid stack key). Each stage: tests first, `dart analyze --fatal-infos` + `dart test` on minimum SDK and stable, golden diff reviewed, commit by the maintainer.

## Decisions (confirmed)

1. A project pack with the same family and option as a built-in one replaces it.
2. `init` is interactive by default; `--yes` writes the detected profile.
3. `worktree watch` ships in P2.

## Progress

- Stage 1 done. Context adds `has_stack_other` next to `stack_other`, because `when` cannot test for a non-empty list. The fragment lists one line per family and states the shared rules once, which keeps AGENTS.md small with every family set to `other`.
- Stage 2 done. A built-in pack can add a fragment to a skill owned by another pack (`di/get_it` adds to `bloc-patterns` of `state/bloc`). When a project pack replaces the owner without shipping that skill, the adapter skips the skill with a warning instead of failing. Golden `sample-bloc-getx` changed only in `lock.json` (`packs`).
- Stage 3 done. `project.flavors` is now validated (each value a non-empty entrypoint path). The section is gated on flavors; a build wrapper without flavors stays covered by the security rule.
- Stage 4 done. Prompt per family: Enter keeps the proposal, `-` drops the family, `<option>` or `other <package>` replaces it; an invalid answer repeats the prompt with the valid options. A family with two candidates has no proposal (Enter leaves it out). A detected option the catalog has no pack for becomes `other <package>`. Golden `init-app` checks the `--yes` output and that it syncs. Follow-up: dependencies declared only in a path package's own pubspec are not read.
