---
target: agents_md
order: 71
rules: [go.context-first, go.wrap-errors, go.handle-once, go.no-panic, go.goroutines-owned, go.gofmt-check]
---
## Go

- `context.Context` is the first parameter of every function that does I/O or may block, and is passed down. Never call `context.Background()` or `context.TODO()` below `main` and tests.
- Wrap a returned error with what was being done: `fmt.Errorf("load order %s: %w", id, err)`. Compare with `errors.Is` and `errors.As`, never by string.
- Handle an error once: log it or return it, never both. Never discard one with `_`.
- Never `panic` in library or handler code; return an error.
- Every goroutine has an owner that stops it and waits for it (`errgroup`, a done channel, a cancelled context). A goroutine nobody can stop is a leak, the counterpart of an undisposed controller.
- `gofmt -l <changed files>` must print nothing. Never run `gofmt -w` or `go fmt`: fix the lines it reports by hand, on task lines only.
