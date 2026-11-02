---
target: skill/pr-description
order: 150
when: { has_links: true }
rules: [pr.section.dependencies]
---
- `## Dependencies` - each dependency in `project.links` this PR needs, with its PR or tag, and whether it is merged; "None" when the PR changes no linked dependency.
