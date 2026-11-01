# GitHub-enforced gates and multi-repo tasks

> Note (P4): the plan gate was dropped in P4 (`docs/plans/p4-skill-first-plan.md`, S5). The task owner confirms the plan in the session; there is no `dashcrew gate` command and no `plan-approval` check. The merge gate (`dashcrew check` as a required status check) and the multi-repo part below live on.

## Problem Statement
How might a small team (four people, three Flutter apps, later one shared core repo), each using their own AI
agent, get gates that cannot be skipped, one place to review, and rules that stay identical across repos, with
GitHub as the only shared infrastructure?

## Recommended Direction
Make GitHub the board and the gate. A `dashcrew check` command, run locally and as a required GitHub Action,
enforces what today are only instructions: the toolkit is current, the PR description has every required
section, and the branch's plan was approved.

Plan approval happens on the task's GitHub issue: the agent posts the plan there and stops; a human answers
with a `/approve plan` comment, from the browser or the GitHub mobile app. The agent never needs to push for
this. The diff gate is the normal PR review. The local guard keeps the agent from posting the approval itself
and from writing app code before the plan is approved.

When the core repo is split out, add a multi-repo task: one branch name across core and the apps that use it,
apps pointed at the core worktree through `pubspec_overrides.yaml` (git-ignored, never committed), PRs merged
core first, and `dashcrew check` refusing an app PR that still depends on an unmerged core branch. A shared
toolkit repo, extended by every app at a version tag, keeps the rules identical.

## Key Assumptions to Validate
- [ ] The team reviews in GitHub PRs: run one sprint with `dashcrew check` as a required check.
- [ ] A GitHub issue per task is acceptable next to the team's tracker: try it on three tasks.
- [ ] Branch protection can be enabled on every repo: confirm admin access.
- [ ] The core split will happen: get a date before building multi-repo tasks.

## MVP Scope
In: `dashcrew check` (CLI and GitHub Action), a generated PR template as the list of required sections,
`dashcrew gate submit plan` and `dashcrew gate status` on a GitHub issue, guard rules against agent-posted
approvals and app-code writes before approval.
Out, in this order: golden-test diffs on the PR, lessons via PR, shared toolkit repo, multi-repo tasks.

## Not Doing (and Why)
- Own web board or Flutter companion app: the issue, the PR and the GitHub mobile app already cover review and phone approval.
- Hosted relay or server: GitHub is the hub; nothing else to run or secure.
- Emulator screenshots in CI first: slow and costly; golden diffs cover most UI regressions cheaply.
- Telling human from agent on GitHub: both use the same token; the local guard prevents mistakes, not malice.
- Auto-merge: merging stays a human action.

## Open Questions
- Issue per task: created by `dashcrew gate submit plan`, or linked to an issue the team already opened?
- After the core split (no date yet): how will apps consume core - git tag, path, or a private pub server?
