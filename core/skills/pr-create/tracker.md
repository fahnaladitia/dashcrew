---
target: skill/pr-create
order: 900
when: { has_tracker: true }
rules: [tracker.review-after-pr, tracker.unchanged-without-pr]
---
## Tracker task

- The PR exists and the task came from the tracker: set its review status and post the PR comment, both, as the `tracker-{{stack.tracker.use}}` skill says.
- The PR was declined or is not created yet: the status stays, no comment, and the summary says so.
