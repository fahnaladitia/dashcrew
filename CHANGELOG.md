## 0.1.1 (unreleased)

- `ci.dashcrew_ref` must be a release tag, `v0.1.0` or newer. A commit or branch is refused when the profile is read, because CI installs dashcrew with `go install`: a branch moves, and a commit from before 0.1.0 has no Go module, so the workflow failed before `dashcrew check` ran. A project migrated from 0.0.1 still holding a commit gets the error from `sync` and `doctor` with the fix.
- The output rule now says what copy-paste safe means: no `<placeholder>` in a command the user runs, since a shell reads `<` as a redirect and the whole block fails. Paid for by shorter wording in the same section; the heaviest AGENTS.md is 6 bytes smaller.

## 0.1.0

- The engine is Go: one static binary per OS with the catalog built in, installed with `brew install fahnaladitia/tap/dashcrew` or `go install`. Output is byte-identical to the Dart engine of 0.0.1.
- Removed: the Dart engine (`lib/`, `bin/`, `pubspec.yaml`) and the `dart pub global activate` install. An old Dart install still on `PATH` is refused by the guard shim and reported by `doctor`.
- `dashcrew guard` is a subcommand. `.ai/bin/guard` runs it from `PATH` and refuses every tool call (fail closed) when dashcrew is missing or older than the lock's `guard_min_version` (0.1.0). The lock's `guard_source` hash is replaced by `guard_min_version`.
- Removed: the per-machine compiled guard in `~/.dashcrew/guard/`, `dashcrew guard install`. `sync --[no-]compile-guard` is a deprecated no-op.
- `dashcrew --version`. Release builds stamp their tag; `go install` builds report their module version.
- The generated GitHub workflow installs dashcrew with `go install` at `ci.dashcrew_ref`.
- `doctor` `guard-binary` fails without dashcrew on `PATH` and warns when the first one is an old Dart install or older than the minimum.
- `dashcrew commit suggest` prints a ready `git add` and `git commit` command in the team's pattern (`project.commit`, Conventional Commits by default), and `dashcrew commit lint` checks a message or a range, email addresses and AI attribution included. `dashcrew check` lints the PR's commits (`commits`, `--base`), and the generated workflow fetches the full history. `dashcrew verify record` stores the verified tree, built in a temporary index; `suggest` lists files changed after it. The git rule and chain now end every task with the commit command.
- Repo hardening: `go.mod` pins the go1.26.10 toolchain (standard library fixes found by `govulncheck`); CI pins every action to a commit SHA, sets read-only permissions, and adds a Linux, macOS and Windows test matrix (Windows may fail), `gofmt`, `go test -race`, `govulncheck` and CodeQL; Dependabot updates Go modules and actions weekly. SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md, issue templates and a pull request template are added. Tests that need a POSIX `sh` skip on Windows.

## 0.0.1

- Repository bootstrap.
