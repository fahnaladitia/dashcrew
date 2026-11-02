---
target: skill/branching
order: 900
when: { has_links: true }
rules: [branching.cross-repo-same-name, branching.cross-repo-link, branching.cross-repo-never-commit-link]
---
## Across repos

A task that changes a dependency listed in `project.links` together with a repo that uses it:

- Use the same branch name in every repo the task touches, each created from that repo's own base.
- Point the user's repo at the dependency's checkout with `dashcrew link add <package> <path>`. It writes a git-excluded file; never commit it, and never edit the manifest's git ref or path to reach local work.
- Merge order is dependency first. The dependent PR names the dependency's PR or tag under `## Dependencies`, and `dashcrew check --links` fails it until the dependency is merged or released.
