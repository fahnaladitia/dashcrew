---
target: agents_md
order: 31
when: { policy.agent_commit: true }
rules: [git.commit-lint-first]
---
- Commit only when the user asks, one logical change per commit, and only files of the current task, with the message `dashcrew commit suggest` builds once `dashcrew commit lint` passes on it.
