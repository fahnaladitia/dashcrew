---
target: agents_md
order: 31
when: { policy.agent_commit: false }
rules: [git.no-agent-commit, git.commit-command]
---
- Never commit. At the end of every task, after the PR description, print the commit command from `dashcrew commit suggest` with your subject and body. When the user says they changed files, run it again and print the new command. When files changed since the last verify, say so and offer a scoped re-verify first.
