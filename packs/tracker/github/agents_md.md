---
target: agents_md
order: 360
rules: [github-tracker.write-only-when-asked, github-tracker.status-only-if-sourced, github-tracker.status-start, github-tracker.status-review, github-tracker.status-unchanged-without-pr, github-tracker.no-other-status, github-tracker.report-in-summarize]
---
## Tracker: GitHub Issues

- Reading issues is free. Never edit, close or comment on one unprompted, except the status flow below.
- The status flow runs only when the work started from an issue number or URL. Never search for a matching issue.
  - Branch created after step 0: assign yourself and set `{{pack.tracker.status_start}}`.
  - PR actually created: body has `{{pack.tracker.pr_link}} #<n>`; set `{{pack.tracker.status_review}}` and post the PR comment, every time.
  - PR declined or not created yet: status unchanged, no comment, and the summary says so.
- Never close an issue or set another status unasked.
- The summary reports the issue, old -> new status and the comment.
- Commands (mode `{{pack.tracker.mode}}`): skill `tracker-github`.
