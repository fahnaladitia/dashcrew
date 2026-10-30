---
target: agents_md
order: 40
rules: [branching.branch-before-code, branching.prefix-by-type, branching.name-rules, branching.propose-three, branching.ask-base, branching.common-bases, branching.clean-tree-check, branching.no-track-create, branching.never-track-base, branching.one-branch-per-working-tree]
---
## Branching

- Every feature, fix or hotfix gets its own branch, created and checked out before any code is written.
- Prefix by type: `{{project.branch_prefix.feat}}` feature, `{{project.branch_prefix.fix}}` fix, `{{project.branch_prefix.hotfix}}` hotfix.
- Names are lowercase kebab-case and scoped to the area (name the screen or feature, not the symptom). No spaces, camelCase, ticket-only names or dynamic values.
- Propose exactly 3 names for the user to pick or edit, then ask which base to fork from (common bases:{{#project.base_branches}} `{{.}}`{{/project.base_branches}}). Never assume a base, not even the current branch.
- Run `git status` first; surface uncommitted changes and ask before switching.
- Create with `git checkout --no-track -b <name> <base>` (the flag goes before `-b`), then confirm with `git branch --show-current`. A feature branch never tracks its base; the `branching` skill has the upstream checks.
- Never check out a second branch in a working tree another task is using: each parallel task gets its own worktree and session (`worktree`).
