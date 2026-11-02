---
target: skill/branching
order: 800
when: { has_tracker: true }
rules: [tracker.start-after-branch, tracker.no-search]
---
## Tracker task

When the task came as a tracker id or URL: once the checks above pass, set the task to its start status as the `tracker-{{stack.tracker.use}}` skill says, and keep old -> new for the summary. Without a tracker task there is nothing to do; never search the tracker for one.
