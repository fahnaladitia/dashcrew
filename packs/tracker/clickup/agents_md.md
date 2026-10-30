---
target: agents_md
order: 360
rules: [clickup.write-only-when-asked, clickup.status-only-if-sourced, clickup.status-in-progress, clickup.status-in-review, clickup.status-unchanged-without-pr, clickup.no-other-status, clickup.report-in-summarize]
---
## Tracker: ClickUp

- Reading tasks is free. Never create, update, close or comment on anything unprompted, except the status flow below.
- The status flow runs only when the work started from a task id or URL. Never search the tracker for a matching task.
  - Branch created (step 1): set the task to `{{pack.tracker.status_start}}`.
  - PR actually created: set `{{pack.tracker.status_review}}` and post the PR comment, both, every time.
  - PR declined or not created yet: status unchanged, no comment, and the summary says so.
- Never move a task to any other status unless asked.
- The summary reports the task id, old -> new status, and the posted comment.
- Tools, fallback and comment format: skill `tracker-clickup`.
