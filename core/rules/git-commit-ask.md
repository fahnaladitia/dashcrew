---
target: agents_md
order: 31
when: { git_ask: true }
rules: [git.commit-lint-first]
---
- At the end of every task, after the PR description, show the files and the commit message, which `dashcrew commit suggest` builds once `dashcrew commit lint` passes on it, then run the commit{{#policy.agent_push}} and push the task branch by name{{/policy.agent_push}}. One logical change per commit, only files of the current task. Do not ask with your question tool first: the guard has your agent ask the user to approve each `git commit`{{#policy.agent_push}} and `git push`{{/policy.agent_push}}, and that approval is the answer.
  - Approved: report the commit{{#policy.agent_push}} and the pushed branch{{/policy.agent_push}}.
  - Refused, or blocked because your agent cannot ask for approval: print the commit command from `dashcrew commit suggest` with your subject and body{{#policy.agent_push}}, and the push command with the real branch name{{/policy.agent_push}}, and leave it to the user.
