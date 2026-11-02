## Summary

What changes and why. Link the issue (`Closes #<n>`) and the plan in `docs/plans/` for a standard or large change.

## Changes

- 

## Generated output

None, or: the goldens rewritten with `DASHCREW_UPDATE_GOLDENS=1` and each changed file read. Name every guard decision that changed in `corpus_outcomes.txt`.

## Testing

- [ ] `go vet ./...`
- [ ] `gofmt -l .` prints nothing
- [ ] `go test ./...` (or the scoped packages, named here)
- [ ] `go test -race ./...` when concurrency changed

## Checklist

- [ ] README and the unreleased section of `CHANGELOG.md` updated when behaviour changed
- [ ] Commits pass `dashcrew commit lint --range <base>..HEAD` (no AI attribution, no email addresses)
- [ ] No company name, internal domain or private path anywhere in the diff
