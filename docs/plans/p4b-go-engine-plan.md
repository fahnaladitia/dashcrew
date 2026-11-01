# P4b plan: Go engine

Tier: large (the whole engine and CLI change language; install and release change). The catalog content (`core/`, `platforms/`, `packs/`) does not change.

Starts after P4 is complete. P5 (knowledge, MCP server) is then built in Go directly.

## Goal

Replace the Dart engine (`lib/`, `bin/`) with a Go engine that renders byte-identical output for every golden sample. That gives the team:

- **One static binary per OS**, installed with Homebrew or `go install`, with no SDK. Today a Go developer or a devops engineer must install Dart only to run dashcrew.
- **The catalog embedded in the binary** (`embed.FS`). Today the engine reads `core/` from the package root at run time, so the install must carry the source tree.
- **The guard as a subcommand of the same binary** (`dashcrew guard`). This removes the per-machine compile step and its hash-keyed cache (`~/.dashcrew/guard/<hash16>/`).
- **The official Go MCP SDK** (`github.com/modelcontextprotocol/go-sdk`) for P5's `knowledge serve`.
- **A language the whole team already reads**, now that the second platform is a Go backend.

## What exists

- Engine: about 4,200 lines of Dart in `lib/src/` (catalog, render, profile, adapters for three agents, sync, lock, doctor, check, guard with a shell parser, init and detect, worktree and watch), plus a 415-line CLI in `bin/dashcrew.dart`.
- Tests: about 4,100 lines (unit tests plus the golden tests). The golden fixtures (`test/golden/*/project.yaml`, `expected/`) are plain files, independent of the language. They are the conformance suite for the port.
- Dependencies: `args`, `crypto`, `mustache_template`, `path`, `yaml`.
- Mustache features used by the catalog: variables (222), sections (31), inverted sections (5). No partials and no lambdas.
- CI: `dart analyze --fatal-infos` and `dart test` on Dart 3.9 and stable.
- Local toolchain: Go 1.26.

## Stages

### G0: skeleton and conformance harness

- In the same repo: `go.mod` at the root, `cmd/dashcrew/`, `internal/<module>/`, one package per Dart module.
- The golden fixtures move to `testdata/golden/`. Both engines read them during the transition.
- `internal/conformance`: for each golden sample, run the Go `sync` into a temp dir and compare every file byte for byte with `expected/`.
- CI runs both engines on every push until G8.

### G1: profile, catalog, render

- Profile parsing with the same strict keys and the same error messages, so tests can compare messages too.
- Catalog loading from `embed.FS`, plus project packs and project platforms from `.ai/`.
- Mustache: `github.com/cbroglie/mustache`, checked against the goldens. If its standalone-line or whitespace handling differs from the Dart renderer, write a small renderer for the three features used (variables, sections, inverted sections) instead of patching around the library.
- Exit criterion: the AGENTS.md and rendered workflows, skills and roles of every sample are byte-identical.

### G2: adapters

- Claude Code, Codex (TOML agent files) and Antigravity, with the same file layout, front matter and MCP merging (owned MCP entries).
- Exit criterion: every per-agent file of every sample is byte-identical.

### G3: sync, lock, doctor, check

- Lock JSON with the same key order and indentation. Removal of locked files that are no longer generated. Conflict detection and `--force`.
- `doctor` and `check` with the same check names, annotations (`::error` in GitHub Actions) and exit codes.
- The generated GitHub workflow installs the Go binary at the pinned ref instead of activating the Dart package.

### G4: guard

- `dashcrew guard` reads the hook payload and `.ai/guard.json` and returns the same decisions and messages.
- The shell parser (`shell.dart`) is ported with every existing guard test case. This is the security-critical part: every Dart guard test becomes a Go table test before the Go guard is wired into any hook.
- The committed shim `.ai/bin/guard` calls the dashcrew binary on PATH, checks it meets the lock's minimum version, and fails closed with a clear message when it is missing.
- The guard hash in `guard.json` and `lock.json` becomes the dashcrew version. This is the one golden change allowed, listed per file.
- Latency: a benchmark of 1,000 hook calls; median under 10 ms (the Dart guard measured 11.5 ms).

### G5: init, detect, worktree, watch

- Platform detection, `platform: other` prompts, `--yes` behaviour, worktree create, remove and watch, with the same output lines.

### G6: CLI parity

- The same commands, flags, help text and exit codes. A test runs every documented command of the README against both engines and diffs stdout, stderr and the exit code.

### G7: release and migration

- goreleaser builds darwin/linux/windows for amd64 and arm64, publishes a GitHub Release and a Homebrew tap formula.
- The version is stamped through `-ldflags`. This ends "every commit reports 0.0.1".
- README install becomes `brew install` or `go install`. The `dart pub global activate` route is documented as deprecated for one release.
- `sync` run by the Go engine rewrites the shim. `doctor` warns when a Dart-installed dashcrew is still first on PATH.

### G8: remove Dart

- After one release with both engines green in CI: delete `lib/`, `bin/`, `pubspec.yaml`, the Dart tests and the Dart CI job.
- Update AGENTS.md (verify commands become `go vet ./...` and `go test ./...`) and `docs/catalog-format.md`.

## How we check

- The conformance harness: every golden sample byte-identical, except the guard hash fields listed in G4.
- Every Dart unit test has a Go counterpart. The count is compared at G8, and any test dropped is named with the reason.
- Guard: all ported cases pass, and the latency benchmark meets its bound.
- CLI parity test green.
- A live run: install with Homebrew on a clean machine with no Dart SDK, `init` and `sync` a Flutter sample and a `platform: other` sample, and run one task through Claude Code with the Go guard.

## Not doing

- Changing any catalog content, rule or rendered text. Output is byte-identical by design.
- New features during the port. P5 starts after G8.
- A Rust or TypeScript engine.
- pub.dev publishing. Homebrew and GitHub Releases replace it.

## Risks

- Mustache whitespace differences between the two libraries. The goldens catch them; the fallback is the small renderer in G1.
- YAML parsing differences between Dart `yaml` and Go `yaml.v3`, for example in booleans, integer types and key order. Profile tests cover each profile key, with values taken from the samples.
- Shell parsing in the guard: a missed case is a security gap, not a cosmetic one. Mitigated by porting every test first and adding fuzz cases from the Dart parser's behaviour.
- Windows paths and line endings: the Dart engine's Windows behaviour is mostly untested. The Go CI matrix adds Windows, and differences are fixed in Go, not mirrored.
- Two engines in one repo during the transition: contributors must change both, or the conformance CI fails. Keep the window short: G0 to G8 without unrelated work in between.

## Decisions (confirmed)

1. Timing: start after P4 is complete, so the port targets a stable catalog and engine.
2. Mustache: try `cbroglie/mustache` first. Fall back to a small renderer (variables, sections, inverted sections) only if the goldens fail.
3. Guard shim: call `dashcrew` from PATH, check the lock's minimum version, and fail closed when it is missing.
4. Repo: the same repo, with both engines side by side until G8.

## Progress

Started before the P4 live run (user decision). The port targets the P4 goldens as committed in `f7ae017`.

- G0-G2 done.
  - `go.mod` (`github.com/fahnaladitia/dashcrew`, go 1.26). `catalog_embed.go` embeds `all:core all:platforms all:packs`.
  - `cmd/dashcrew` has `sync` only for now. `internal/` holds profile, catalog, render, mustache, adapters, sync, plus the helpers projectfiles, yamlutil, dartjson and omap.
  - CI has a `go` job next to the Dart job.
  - Deviation: the goldens stay in `test/golden/`; they were not moved to `testdata/`.
  - Dependencies: only `go.yaml.in/yaml/v3` v3.0.6. Plain scalars are resolved with Dart's YAML 1.2 core rules.
  - Mustache: `cbroglie/mustache` v1.4.2 matched every golden, but differed from Dart on 6 of 53 recorded value cases (`<nil>`, Go map and list printing, falsy `""`, `" "` and `0`) and missed 6 strict errors. It was replaced by a small port of Dart `mustache_template` 2.0.4 that passes all 53 (`internal/mustache/testdata/dart_cases.json`).
  - Conformance: all 7 samples byte-identical (316 files), with no extra files and an idempotent second sync. Excluded until their stages: `.ai/lock.json` (G3), `.ai/guard.json` and `.ai/bin/guard` (G4).
  - Tests: 164 Go tests pass, and the 385 Dart tests are unchanged.
  - Known differences: malformed inputs that crash Dart with a type error now return an ordinary error, and YAML syntax error wording differs.
- G3 done.
  - Ported: sync (lock in Dart key order, removals, conflicts and `--force`, owned MCP, warnings, the P4 migration note), `internal/doctor` and `internal/check` (same names, statuses, messages, `::error` lines, exit codes).
  - CLI: `sync`, `doctor` and `check`, with exit codes 0, 1 and 64. Adapter `Files` is now ordered, so the `wrote` lines follow Dart's order.
  - The lock's `guard_source` hash comes from the Dart guard sources, embedded via `guard_source_embed.go` until the guard switch.
  - Conformance: 323 of 323 files identical, the lock included, and `sample-migrate-p3` added. The Dart and Go CLIs were run side by side on 5 samples and the migration, with identical output.
  - Not resolved: JSON error wording in doctor details, and `--compile-guard` is accepted but does nothing.
  - 16 Go packages pass; Dart 385.
- Sequencing change (decided during G3): G4 ports the guard logic as `dashcrew guard` with every test and the latency benchmark, but the emitted shim and `guard.json` stay byte-identical to Dart. The shim switch to `dashcrew guard` (and the guard hash becoming the dashcrew version) moves to G7, done in both engines at once with the workflow switch, as one deliberate golden change. This keeps conformance strict until then.
- G4 done.
  - Ported tests: `guard_test` 58/58, `shell_test` 9/9 and `hook_command_test` 3/3, with count checks.
  - Differential test against the compiled Dart guard: 11,187 cases, all identical (exit code, stdout, stderr). It skips unless `DASHCREW_DART_GUARD` is set. The one divergence found was a Go bug (a leading BOM), now fixed.
  - Fuzz: 60 s Go-only (14.7M runs), plus 2 x 120 s with a Dart oracle (`DASHCREW_DART_SHELL`). No failures.
  - Latency over 1,000 process calls: Go median 3.08 ms (p95 3.29); Dart AOT median 10.40 ms (p95 11.41).
  - Conformance: the exclusion list is empty, 337 of 337 files identical.
  - `dashcrew guard` is in the CLI. `guard install` exits 64 until G7.
- Known gaps in the guard, inherited from Dart: both engines allow these, so they are not divergences.
  1. `bash -c`, `sh -c`, `eval` and here-strings are not unwrapped; only Codex argv lists are.
  2. Shell keywords hide the command: `{ ...; }`, `then`, `do`, `!`.
  3. Wrappers with options, or not on the wrapper list: `sudo -u`, `env -i`, `/usr/bin/env`, `nice`, `timeout`, `exec`, `xargs`, `find -exec`, `command -p`, `builtin`.
  4. A backslash-newline inside `git ... push` leaves an empty word.
  5. Indirection: `$(echo git) push`, `git${IFS}push`, `$x push`.
  6. Fail-open by design on malformed payloads, invalid UTF-8 and deep nesting. `apply_patch` with CRLF headers is not checked, and a leading `*** Move to:` line lets the whole patch through.

  Hardening is a decision for the user (see G4b).
- G5-G6 done.
  - `internal/initproject`: `detect_test` 37/37, `init_test` 11/11, and the `init-app` golden in conformance.
  - `internal/worktree`: `worktree_test` 16/16, run in temp git repos.
  - `cmd/dashcrew/cli.go` ports the parts of Dart `args` we use (help, usage errors, "Did you mean").
  - Parity test (`DASHCREW_DART` set): 21 cases, 224 calls per engine. It compares stdout, stderr, the exit code and the file tree, and is green.
  - Known exceptions until G7: `--compile-guard` installs nothing, `guard install` exits 64, and bare `guard` runs the Go hook.
  - Crash paths: where Dart exits 255 on malformed YAML, Go prints the error and exits 1.

### G4b: guard hardening (Go only, with the G7 switch)

Pending the user's decision on scope (gaps 1-4 plus `apply_patch`, or also 5) and on malformed payloads (fail-open with a doctor warning, or fail-closed). Go only: after G7 every hook calls the Go guard, so the Dart guard is not fixed.
