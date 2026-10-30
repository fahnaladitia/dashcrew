---
target: agents_md
order: 31
when: { policy.agent_commit: true }
---
- Commit only when the user asks, one logical change per commit, and only files of the current task.
