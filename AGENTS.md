# dashcrew - contributor rules for AI agents

dashcrew is a Go CLI that renders an AI coding toolkit for software projects
(Flutter built in, other stacks through `platform: other`).
These rules apply to work on this repository, not to the projects dashcrew
generates files for.

## Dynamic chain

Size every task first, one line with evidence, then run the matching chain.

| Tier | Signal | Chain |
|---|---|---|
| trivial | one file, a few lines, one layer, no behavior change (typo, copy, doc) | edit -> `go vet` on the package -> one-line summary |
| small | 1-2 files, existing module, behavior obvious after reading the code | test first -> implement -> `go vet ./...` + scoped `go test` -> summary |
| standard | 3+ files or 2+ modules (profile, catalog, render, adapters, guard, lock, doctor), or any change to generated output | plan in `docs/plans/<slug>.md` -> wait for confirmation -> test first -> implement -> review the diff -> verify -> summary |
| large | new adapter, new pack family, schema version bump, output layout change | standard, plus golden fixtures updated and reviewed before merge |

Doubt between two tiers means the higher one.

## Git

- The maintainer owns commits, pushes and merges. Agents stage or diff when asked, never commit or push.
- No AI attribution in commit messages, PR bodies or comments.

## Code

- Go, standard library first. No new dependency without a stated reason in the plan; today they are `go.yaml.in/yaml/v3` and `github.com/modelcontextprotocol/go-sdk` (for `knowledge serve`).
- `cmd/dashcrew` stays thin: flag parsing and output. Logic lives in `internal/<concern>/`, one concern per package.
- Tests live next to the code they test (`internal/<pkg>/*_test.go`), with hand-written fakes and temp folders, no mocking library.
- Generated output is checked with goldens: `test/golden/` (sync and init output), `cmd/dashcrew/testdata/transcripts/` (CLI output) and `internal/guard/testdata/` (guard answers).
- The output format is fixed: `internal/dartjson`, `internal/mustache` and the scalar rules in `internal/yamlutil` define the JSON layout, template rendering and YAML values byte for byte. Change them only with a golden update.
- Neutral sources (`core/`, `packs/`, `platforms/`) are Markdown or YAML with front matter. Keep them agent-neutral: agent-specific fields are added by adapters, never written in `core/`.
- Generated files carry a dashcrew marker and are never edited by hand.

## Verify

```bash
go vet ./...
go test ./...
```

Scope `go test` to the touched packages (`go test ./internal/<pkg>/...`) for trivial and small tiers.

## Goldens

When a change to generated output is intended, rewrite the goldens and review every changed file before asking for a merge:

```bash
DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard
git diff --stat test/golden cmd/dashcrew/testdata internal/guard/testdata
```

`test/golden/sample-migrate-p3/p3/` is frozen: never rewrite it. A changed line in `internal/guard/testdata/corpus_outcomes.txt` is a guard decision that changed; a removed line is a case the guard no longer blocks.
