---
name: pr-create
description: "Ask about, check and open the GitHub PR for the current branch; handle merge requests safely."
when_to_use: "After the PR description, or when the user asks to merge a PR."
---

# Opening a PR

Opening a PR is an outward-facing action. Ask once: "Open a PR for `<branch>` into `<base>`? (yes / draft / no)". Act only on yes or draft.

- On no: skip it, and say so in the summary.
- `<base>` is the base chosen when the branch was created. Never guess it and never default to `main`; if you do not know it, ask.
- Commit and push belong to the user. If either is missing, stop and hand over the exact command (first push: `git push -u origin <branch>`); never run it.

## Pre-flight: all must pass

1. `git status --porcelain`: task files still uncommitted would be missing from the PR. Report and stop.
2. `git log --oneline <base>..HEAD` is not empty.
3. `git rev-parse --abbrev-ref @{u}` prints `origin/<branch>` (the same name), and `git status -sb` shows no `[ahead N]`. Otherwise report and stop.
4. `gh pr list --head <branch> --state open`: if a PR is already open, never open a second. Offer `gh pr edit <n> --body-file docs/pr/<branch>.md`, again only on a yes.

## Create

```bash
gh pr create --base <base> --head <branch> \
  --title "<first line of the PR description file, without '# '>" \
  --body-file docs/pr/<branch>.md
```

- Add `--draft` only when the answer was "draft".
- No reviewers, labels, assignees or milestone unless the user names them.
- Report the URL that `gh` prints. Never invent or guess a URL.

## Back to base: suggest, never run

After the PR is created, or found merged:

1. `git status --porcelain` must be clean.
2. `git rev-parse --abbrev-ref <base>@{u}` must print `origin/<base>` (the same name); report how far behind it is.
3. Suggest exactly `git checkout <base>` and `git pull`, nothing else.

Never run the checkout or the pull yourself. Never delete or suggest deleting the task branch, locally or on the remote; the user keeps it.

## Merge

Only when the user explicitly asks to merge a specific PR: show the PR number, base, `mergeStateStatus` and merge method, then ask "Merge PR #<n> into `<base>`? (yes / no)". Run the merge only on that yes.
