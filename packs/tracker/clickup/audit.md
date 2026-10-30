---
target: workflow/audit
order: 360
rules: [clickup.audit-no-status-change, clickup.audit-comment-draft]
---
### Audit of a tracker task

- An audit never changes the task status. The status flow starts only when a confirmed finding is worked as a task on its own branch.
- The audit may draft a comment under {{pack.tracker.comment_max_chars}} characters: the problem, the evidence, the request. Send it only when the user says send.
