---
target: workflow/audit
order: 10
rules:
  - audit.read-only
  - audit.mode-selection
---
# audit

Four modes, one workflow. It sits outside the chain.

## Read-only

- Edit nothing under `lib/`, `test/` or the toolkit.
- Create no branch.
- Fixes are proposals only. Each one becomes a new task that runs the normal chain.

`audit` judges code that is already merged. It does not replace `code-review` (the change in progress) or `verify` (the gate before a PR).

## Mode selection

Pick the mode from the first argument, checking in this order:

1. `toolkit` -> toolkit mode
2. `flow` -> flow mode
3. a path that exists on disk -> code mode
4. a task id or URL of the project's tracker -> investigation mode with a task
5. anything else (a free sentence) -> investigation mode without a task
