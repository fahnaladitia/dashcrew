---
target: workflow/audit
order: 360
rules: [github-tracker.audit-no-status-change, github-tracker.audit-comment-draft]
---
### Audit of a tracker issue

- An audit never changes the issue's status, labels or assignees. The status flow starts only when a confirmed finding is worked as a task on its own branch.
- The audit may draft a comment under {{pack.tracker.comment_max_chars}} characters: the problem, the evidence, the request. Send it only when the user says send.
