---
name: go-errors
description: "Error handling checklist for Go code - wrapping, sentinel errors and what a caller may compare."
when_to_use: "When a change adds, wraps or handles an error path in Go code."
---

# Go errors

- Compare errors with `errors.Is` and `errors.As`, never with `==` on a wrapped error.
- Export a sentinel error only when callers must branch on it; document it on the package.
- Test each error path the change adds: the returned error wraps the cause, and `errors.Is` finds it.
