---
target: workflow/plan
order: 30
rules:
  - plan.branch-section
---
## Branch

For a new feat, fix or hotfix, end the plan with:

- exactly 3 branch names, kebab-case, naming the area, with the prefix `{{project.branch_prefix.feat}}`, `{{project.branch_prefix.fix}}` or `{{project.branch_prefix.hotfix}}`
- a question asking which base branch to fork from; never assume it (common bases:{{#project.base_branches}} `{{.}}`{{/project.base_branches}})
- the output of `git status` when the working tree is dirty
