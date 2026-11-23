# Flutter verify: a read-only dart format check

Tier: standard (a guard decision changes, and the generated verify workflow and AGENTS.md of every Flutter project change; goldens rewritten and reviewed).

Issue: #67. Decision taken with the maintainer: the guard lets a read-only `dart format` through even with `policy.formatter: false`, and verify always runs the check.

## Problem

Flutter CI commonly runs `dart format --output=none --set-exit-if-changed` on every Dart file. Verify never checks formatting, so a branch reports `PR ready: yes` and then fails CI on a wrapped argument. With `policy.formatter: false` (the default) the agent could not even run the check: the guard blocks every `dart format`, read-only or not (`internal/guard/decide.go`, the `BlockFormatters` branch).

## Guard

`dart format` (also through `fvm`, `bash -c` and the other forms the guard unwraps) is allowed under `block_formatters` when it cannot write: its last `--output`/`-o` value (`--output=none`, `--output none`, `-o none`, `-onone`), read before a `--`, is `none`, `show` or `json`. Anything else, `write` or no `--output`, stays blocked. `dart fix` and `flutter format` stay blocked whatever their flags.

Options are read from an allowlist (`--set-exit-if-changed`, `--follow-links`, `--fix`, `-v`, and the value options `-l`, `--line-length`, `--page-width`, `--indent`, `--language-version`, `--enable-experiment`, `--show`, `--summary`, `--trailing-commas`). Any other option counts as writing: in `dart format --enable-experiment -o none lib/` the experiment takes `-o` as its value, `none` becomes a path and `lib/` is rewritten.

- No policy key and no `guard_min_version` change: an older guard blocks the check, which costs a blocked call, not a write. The verify text says what to do then.
- Tests first, in their own `TestReadOnlyDartFormatPassesTheFormatterBlock` (the `blockedShell` and `allowedShell` tables are pinned to the ported Dart test count): allowed `dart format --output=none --set-exit-if-changed lib/a.dart`, `dart format -o none lib/`, `fvm dart format --output=show lib/a.dart`, the check under `bash -c`; blocked `dart format lib/`, `dart format --output=write lib/`, `dart format -o none -o write lib/`, `dart format --set-exit-if-changed lib/` (no `--output`), `dart format -- --output=none`, an unknown option before `-o none`, `dart fix --dry-run`.
- Three corpus seeds, appended at the end of `seedCommands` so existing seed numbers keep their names. The random cases are drawn from the seed list, so they change too: review `corpus_outcomes.txt` by rendering it with the old `decide.go` and the new one over the same corpus, and expect only `dart format` blocks turning into allows.

## Generated text (Flutter platform)

- `platforms/flutter/workflows/verify/20-phases.md`, Phase 1, after analyze:

  ```bash
  dart format --output=none --set-exit-if-changed <Dart files from the scope, without *.g.dart and *.freezed.dart>
  ```

  - It writes nothing; it lists each file whose layout differs and exits 1.
  - `policy.formatter: true`: run `dart format` on the listed files, then rerun the check.
  - `policy.formatter: false`: format the listed lines by hand when they belong to the task; a file whose differences are outside the task is reported and the user decides. Verify does not pass while a task line fails the check.
  - When the guard blocks the check (a dashcrew older than this release), report it and name the files in scope; do not count verify as passed on format.
  - The two policy branches are separate fragments gated with `when: { policy.formatter: ... }`, as `25-no-formatter.md` is. Phase 1 stays in `20-phases.md`, the branches are `21-format-fix-on.md` and `21-format-fix-off.md`, and phases 2 to 6 move to `22-phases.md`; fragments join with a blank line, so the split adds no bytes.
- `platforms/flutter/workflows/verify/25-no-formatter.md`: "Never let `dart format` or `dart fix` rewrite files to clear analyzer or diff output; the read-only check in phase 1 is the only formatter call."
- `platforms/flutter/rules/code-formatter-off.md` (AGENTS.md): reword to forbid rewriting, not running: "Never let a formatter or an auto-fixer (`dart format`, `dart fix`) rewrite files as part of a change." Same length or shorter: the AGENTS.md budget has 5 bytes left.
- `docs/guard.md`: the formatter row names the read-only exception. `docs/profile.md`: the `policy.formatter` row says the read-only check always runs.

## Tests first (render)

`internal/render/catalog_render_test.go`: with the default profile, `workflow/verify` contains `--output=none --set-exit-if-changed` and the by-hand branch; with `policy: { formatter: true }`, the `dart format` branch. `TestLargestOptionOfEveryFamilyStaysUnderTheLimit` still passes.

## Goldens

`DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard`. Expected: the verify workflow or skill of every Flutter golden for every agent, the reworded AGENTS.md line in every Flutter golden with `formatter: false`, the lock hashes, and new corpus lines. Not expected: any change under Go or `platform: other` goldens; `test/golden/sample-migrate-p3/p3/` stays frozen.

## Verify

`gofmt -l .`, `go vet ./...`, `go test ./...`; `dashcrew sync` in a scratch Flutter project, then pipe the check command through `dashcrew guard` with the generated policy to see it allowed, and `dart format lib/` blocked.

## Not doing

- A format check in core verify for other platforms. Go already lists `gofmt -l` as a check in its rules; `platform: other` can put a check command in `commands.format`. A follow-up can render `{{platform.commands.format}}` in core verify once its meaning (check or rewrite) is fixed per platform.
- Writing a Flutter CI workflow for the project.
- Letting `dart fix --dry-run` through.
