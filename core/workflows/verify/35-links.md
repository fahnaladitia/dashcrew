---
target: workflow/verify
order: 35
when: { has_links: true }
rules: [verify.check-links]
---
- With `project.links`, run `dashcrew check --links` before the commit command. When it refuses a dependency, say which one and why; the user merges or releases it first, then pins a tag or a commit on its default branch.
