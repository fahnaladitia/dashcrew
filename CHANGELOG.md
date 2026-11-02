## 0.1.0 (unreleased)

- The engine is Go: one static binary per OS with the catalog built in, installed with `brew install fahnaladitia/tap/dashcrew` or `go install`. Output is byte-identical to the Dart engine of 0.0.1.
- Removed: the Dart engine (`lib/`, `bin/`, `pubspec.yaml`) and the `dart pub global activate` install. An old Dart install still on `PATH` is refused by the guard shim and reported by `doctor`.
- `dashcrew guard` is a subcommand. `.ai/bin/guard` runs it from `PATH` and refuses every tool call (fail closed) when dashcrew is missing or older than the lock's `guard_min_version` (0.1.0). The lock's `guard_source` hash is replaced by `guard_min_version`.
- Removed: the per-machine compiled guard in `~/.dashcrew/guard/`, `dashcrew guard install`. `sync --[no-]compile-guard` is a deprecated no-op.
- `dashcrew --version`. Release builds stamp their tag; `go install` builds report their module version.
- The generated GitHub workflow installs dashcrew with `go install` at `ci.dashcrew_ref`.
- `doctor` `guard-binary` fails without dashcrew on `PATH` and warns when the first one is an old Dart install or older than the minimum.
- `dashcrew commit suggest` prints a ready `git add` and `git commit` command in the team's pattern (`project.commit`, Conventional Commits by default), and `dashcrew commit lint` checks a message or a range, email addresses and AI attribution included. `dashcrew check` lints the PR's commits (`commits`, `--base`), and the generated workflow fetches the full history. `dashcrew verify record` stores the verified tree, built in a temporary index; `suggest` lists files changed after it. The git rule and chain now end every task with the commit command.

## 0.0.1

- Repository bootstrap.
