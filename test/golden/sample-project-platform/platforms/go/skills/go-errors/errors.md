---
target: skill/go-errors
rules: [go.errors-is, go.sentinel-exported]
---
# Go errors

- Compare errors with `errors.Is` and `errors.As`, never with `==` on a wrapped error.
- Export a sentinel error only when callers must branch on it; document it on the package.
- Test each error path the change adds: the returned error wraps the cause, and `errors.Is` finds it.
