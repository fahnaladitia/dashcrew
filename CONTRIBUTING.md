# Contributing to dashcrew

Thanks for helping. Bugs and ideas start as an issue (use the templates); a security problem goes through [SECURITY.md](SECURITY.md) instead. AI agents working on this repository also follow [AGENTS.md](AGENTS.md).

## Build and test

Requires Go 1.26 or later (`go.mod` pins the toolchain patch release; `go` downloads it when `GOTOOLCHAIN` allows), git, and a POSIX `sh`.

```bash
go build ./cmd/dashcrew
go vet ./...
gofmt -l .                                 # must print nothing
go test ./...                              # or scoped: go test ./internal/<pkg>/...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 ./...
```

CI runs the same on every pull request: `go vet` and `go test` on Linux, macOS and Windows, plus `gofmt`, `-race`, `govulncheck` and CodeQL. Windows is not supported yet (the guard shim is a `sh` script), so tests that need `sh` skip there and a Windows failure does not fail the run.

Style:

- Standard library first. A new dependency needs a stated reason in the plan.
- `cmd/dashcrew` stays thin; logic lives in `internal/<concern>/`, one concern per package.
- Tests sit next to the code, with hand-written fakes and temp folders, no mocking library.
- Run `gofmt` on the files you change, nothing else.

## Goldens and transcripts

Generated output is pinned byte for byte:

- `test/golden/`: the files `sync` and `init` write, checked by `internal/conformance`.
- `cmd/dashcrew/testdata/transcripts/`: stdout, stderr, exit code and file changes of every CLI call in the README.
- `internal/guard/testdata/`: the guard's answer to every corpus case it blocks.

When a change to output is intended, rewrite them and read every changed file before you ask for review:

```bash
DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard
git diff --stat test/golden cmd/dashcrew/testdata internal/guard/testdata
```

A changed line in `internal/guard/testdata/corpus_outcomes.txt` is a guard decision that changed; a removed line is a case the guard no longer blocks. Say why in the pull request. `test/golden/sample-migrate-p3/p3/` is frozen and never rewritten.

## Commits

Conventional Commits, `<type>(<scope>): <subject>` with a subject of at most 72 characters, as `git log` shows. Check a message, or your branch, with dashcrew itself:

```bash
dashcrew commit lint "fix(guard): block git push behind nice"
dashcrew commit lint --range main..HEAD
```

- No AI attribution: no generated-with footers, no AI co-author trailers. `commit lint` rejects them.
- No email addresses in messages. `commit lint` rejects them too; set your git email to your GitHub noreply address.

## What the repo never contains

dashcrew is a general tool. Code, docs, examples, fixtures and commit messages never name a company or its internal domain, product or project. Use neutral examples, such as `lib/features/cart/` or `notes-sync`.

## Plans and design history

`docs/plans/` holds the design history: one plan per change of the standard tier or larger, written and confirmed before the code. Read the plans of the area you touch before changing it, and add a plan for a standard or large change (sizing rules in [AGENTS.md](AGENTS.md)). `docs/ideas/` holds ideas not yet planned.

## Pull requests

Fill in the pull request template. Keep one concern per pull request, update the README and the unreleased section of `CHANGELOG.md` when behaviour changes, and include the golden diff review when output changed. The maintainer merges.

By contributing you agree that your work is licensed under the [BSD-3-Clause license](LICENSE) and that you follow the [Code of Conduct](CODE_OF_CONDUCT.md).
