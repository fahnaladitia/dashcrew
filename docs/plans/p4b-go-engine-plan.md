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

Not started.
