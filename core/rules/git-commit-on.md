---
target: agents_md
order: 31
when: { policy.agent_commit: true }
rules: [git.commit-lint-first]
---
- At the end of every task, after the PR description, ask with your question tool whether to commit{{#policy.agent_push}} and push{{/policy.agent_push}} for the user or leave it to them. The answer covers that round only; never commit{{#policy.agent_push}} or push{{/policy.agent_push}} unasked.
  - For the user: one logical change per commit, only files of the current task, with the message `dashcrew commit suggest` builds once `dashcrew commit lint` passes on it{{#policy.agent_push}}; then push the task branch by name{{/policy.agent_push}}.
  - Left to them: print the commit command from `dashcrew commit suggest` with your subject and body{{#policy.agent_push}}, and the push command with the real branch name{{/policy.agent_push}}.
