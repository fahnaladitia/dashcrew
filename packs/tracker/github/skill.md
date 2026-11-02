---
target: skill/tracker-github
order: 100
rules: [github-tracker.read, github-tracker.mode, github-tracker.project-ids, github-tracker.project-add-own-issue, github-tracker.labels, github-tracker.scope, github-tracker.exact-names, github-tracker.pr-link, github-tracker.comment-format, github-tracker.no-pr-no-comment]
---
# GitHub tracker

This repository keeps issue status in mode `{{pack.tracker.mode}}`: `project` uses the single-select field `{{pack.tracker.status_field}}` of a GitHub Project, `labels` uses issue labels.

## Reading

- One issue: `gh issue view <n> --json number,title,body,labels,assignees,projectItems,url`. Its thread: `gh issue view <n> --comments`.
- Never print a token or put one on a command line; `gh` reads its own credentials.

## Start (branch created)

1. `gh issue edit <n> --add-assignee @me`.
2. Set the status to `{{pack.tracker.status_start}}` (mode `project`) or the label `{{pack.tracker.label_start}}` (mode `labels`), as below.

## Review (PR created)

1. The PR body carries `{{pack.tracker.pr_link}} #<n>` before the PR is opened.
2. Set the status to `{{pack.tracker.status_review}}` (mode `project`) or swap to the label `{{pack.tracker.label_review}}` (mode `labels`).
3. Post the comment below.

## Mode `project`

The Project: owner `{{pack.tracker.project_owner}}`, number `{{pack.tracker.project_number}}`. When either is empty, ask the user for it before any `gh project` call.

- Set a status by name: `gh project item-edit <number> --owner <owner> --url <issue url> --field "{{pack.tracker.status_field}}" --value "<status>"`. When the issue is not in the Project yet, add this one issue first: `gh project item-add <number> --owner <owner> --url <issue url>`.
- A gh whose `gh project item-edit --help` has no `--field` takes ids instead: the project `id` from `gh project view <number> --owner <owner> --format json`, the field and option ids from `gh project field-list`, the item id from `gh project item-list`, then `gh project item-edit --id <item> --project-id <project> --field-id <field> --single-select-option-id <option>`.
- The status names must exist as options of `{{pack.tracker.status_field}}`. A missing field or option: stop and ask, never create one.
- The Project's built-in workflows may set its default `Status` field when an item is added. With `{{pack.tracker.status_field}}` being that same field, the flow's value wins; with another field, the board shows both.

Projects need gh 2.31 or newer and the `project` scope. When `gh project` is an unknown command, stop and hand the user the upgrade of `gh`; when `gh` refuses for scope, hand them `gh auth refresh -s project`.

## Mode `labels`

- Start: `gh issue edit <n> --add-label "{{pack.tracker.label_start}}"`.
- Review: `gh issue edit <n> --remove-label "{{pack.tracker.label_start}}" --add-label "{{pack.tracker.label_review}}"`.
- A label that does not exist in the repository: stop and ask, never create one.

## PR comment

- Post only after the PR exists, and only per the flow or when asked. It never replaces the PR description. No PR: say so and stop; never invent a URL.
- Exactly three lines, through `gh issue comment <n> --body-file -`:

```
PR: <url printed by the PR tool>
Branch: <branch name>
<one line: what changed>
```
