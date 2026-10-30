---
target: agents_md
order: 31
when: { policy.agent_commit: false }
rules: [git.no-agent-commit]
---
- Never commit. The user makes every commit; suggest a message only when asked.
