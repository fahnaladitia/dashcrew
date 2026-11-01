---
target: agents_md
order: 71
rules: [go.wrap-errors, go.no-panic]
---
## Go

- Wrap every returned error with context: `fmt.Errorf("load order %s: %w", id, err)`.
- Never `panic` in library code; return an error instead.
- Apply the `go-errors` skill when a change adds or handles an error path.
