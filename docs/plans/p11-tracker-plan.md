# P11 plan: the tracker as the team view

Tier: standard (a new tracker pack, gated skill text, one template flag; no new command, no guard change). Idea: `docs/ideas/knowledge-tracker-multirepo.md`, section P11. Ships in 0.2.0 with P9 and P10.

## Goal

The tracker shows the team where each task stands, without anyone updating it by hand and without sharing transcripts. Today only the ClickUp pack has a status flow, and it lives as bullets in AGENTS.md. P11 makes the flow an explicit step of the chain, adds a GitHub Issues/Projects pack for teams without ClickUp, and proves the flow live.

## Principles (unchanged)

- The flow runs only when the work started from a tracker task (an id or URL in the prompt). Never search the tracker for a matching task.
- Status only: `start` when the branch exists, `review` with the PR link when the PR exists. Never another status, never close a task, unless asked.
- Reading is free; writing happens only through the flow or on request.

## Stages

### T0: the flow as a chain step, in skills

- Template flag `has_tracker`: true when the profile's `tracker` choice is neither `none` nor `other`.
- `branching` skill, gated with `when: { has_tracker: true }`: after step 1's checks, with a tracker task, set its start status per the tracker skill; the summary records old -> new.
- `pr-create` skill, gated the same way: after the PR exists, set the review status and post the three-line PR comment; when the PR is declined or not created, the status stays and the summary says so.
- AGENTS.md does not change (its budget has 2 bytes left); a render test pins that.

### T1: `packs/tracker/github`

`pack.yaml` params:

```yaml
mode: project              # project | labels
project_owner: null        # user or org that owns the Project (mode project)
project_number: null       # the Project number (mode project)
status_field: Status       # single-select field in the Project
status_start: In Progress
status_review: In Review
label_start: "status: in progress"   # mode labels
label_review: "status: in review"
pr_link: Refs              # Refs | Closes; Closes also closes the issue on merge
```

- `agents_md`: the same flow and limits as ClickUp, in `gh` terms, and that the PR body carries `<pr_link> #<n>`.
- Skill `tracker-github`:
  - read: `gh issue view <n> --json number,title,body,labels,assignees,projectItems`, comments with `--comments`;
  - start: `gh issue edit <n> --add-assignee @me`, then the status: in mode `project`, the item, field and option ids from `gh project item-list`, `gh project field-list` and `gh project view --format json`, then `gh project item-edit`; in mode `labels`, swap the labels with `gh issue edit`;
  - review: the same status steps with the review value, and `gh issue comment <n> --body-file -` with the three lines;
  - an issue that is not in the Project: add it (`gh project item-add`) only in mode `project` and only for the flow's own issue;
  - `gh` needs the `project` scope for Projects: when a call fails on scope, stop and hand the user `gh auth refresh -s project`.
- `audit` fragment: an audit never changes the issue status; a drafted comment is sent only on the user's say.
- `init` lists `github` with the other tracker options.

### T2: live

- On the private sample `shop-api`: one issue in a Project with a Status field (In Progress, In Review), created by the user or by me on the user's yes.
- A Claude Code session starts from the issue URL: the issue moves to In Progress when the branch exists, and to In Review with the comment when the PR is opened; a second run in mode `labels`.
- Notes and token counts in this plan.

## How we check

- T0: render test: the gated lines appear in `branching` and `pr-create` only with a tracker, and AGENTS.md is byte-identical with and without one; Flutter goldens unchanged.
- T1: golden `sample-github-tracker` (mode project) and a render test for mode labels; every `{{pack.tracker.*}}` resolves.
- T2: the live run above.

## Not doing

- A ClickUp live run (the user chose GitHub for this phase).
- Creating Projects, fields or labels from the CLI; the skill tells the user what to create.
- A status board or summary command (personal views come with P10b, `role: lead`).
- Jira, Linear and other trackers.

## Decisions (all confirmed)

1. GitHub status lives where the team chooses: a Projects v2 field (mode `project`) or issue labels (mode `labels`).
2. The flow becomes a chain step through gated skill text; AGENTS.md stays as it is.
3. Live check on GitHub in the sample repo.
4. P11 ships in 0.2.0 together with P9 and P10.
5. `pr_link` defaults to `Refs`: merging never closes the issue unasked; a team sets `Closes` to have it closed.


## Progress

- [x] T0 chain step in skills: `has_tracker`, `core/skills/branching/tracker.md`, `core/skills/pr-create/tracker.md`; AGENTS.md unchanged (test).
- [x] T1 GitHub tracker pack: `packs/tracker/github` (agents_md trimmed to keep the heaviest AGENTS.md at 23,786 of 23,788), skill `tracker-github`, audit fragment, golden `sample-github-tracker`, labels-mode render test. `init` lists it with the other tracker options; no detection.
- [x] T2 live (2 Nov, private sample `shop-api`, Project 1 of the maintainer, dashcrew built as 0.2.0).
  - Found while setting it up: the maintainer's gh was 2.14.7 (2022), which has no `gh project` (added in 2.31). Added: the skill stops with the upgrade hint, and `doctor` warns (`tracker` check) for mode project.
  - Run 1, mode project, issue #2 not yet in the Project: the agent added it, assigned it and set In Progress after the branch existed; after the user's push it opened PR #4 with `Refs #2`, set In Review and posted the three-line comment. 40.2k output tokens (main 32.5k, reviewer 7.7k), no guard block. Merging PR #4 left the issue open, as `Refs` intends.
  - Found in run 1: with a separate status field, GitHub's built-in workflow set the default `Status` to Todo when the item was added, so the board showed Todo. The README and skill now say to add `In Review` to the built-in `Status`; and gh 2.103 sets a field by name (`--url`, `--field`, `--value`), which the skill now prefers over ids.
  - Run 2, mode labels, issue #3: `status: in progress` after the branch, then swapped for `status: in review` with PR #5 (`Refs #3`) and the comment; the issue stayed out of the Project. 26.8k output tokens (main 21.8k, reviewer 5.0k), no guard block.
