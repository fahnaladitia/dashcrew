---
target: workflow/audit
order: 60
rules:
  - audit.save-paths
  - audit.close-with-question
---
## Save

The main session writes the file. Subagents only return their report.

- code mode: `{{paths.audits}}/<path>/<YYYY-MM-DD>.md`, keeping `/` as folders
- flow mode: `{{paths.audits}}/flow/<name>/<YYYY-MM-DD>.md`, with the Phase 1 map at the top; with `--task`: `{{paths.audits}}/flow/<name>/<YYYY-MM-DD>-<task id>.md`
- investigation with a task: `{{paths.audits}}/task/<task id>/<YYYY-MM-DD>.md`
- investigation without a task: `{{paths.audits}}/question/<question-slug>/<YYYY-MM-DD>.md`
- toolkit mode: `{{paths.audits}}/toolkit/<YYYY-MM-DD>.md`
- A second audit on the same day updates the same file.

## Close

- End by asking which findings should become tasks.
- Never start fixing without an answer.
- Every fix is a new task on its own branch, through the normal chain.
- Several items on the same screen may share one branch while its PR is still open.
