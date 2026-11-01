---
target: agents_md
order: 30
rules: [git.no-agent-push, git.stage-diff-only-when-asked, guard.block-git-stash, guard.block-git-history-writes, git.no-ai-attribution, pr.never-merge, pr.create-yes-not-merge-yes]
---
## Git

- Never push. The user pushes, and sets the upstream on the first push.
- Showing a diff is fine; stage only when asked.
- Never run `git stash` in any form: it writes commit objects and moves the user's working tree.
- Never run `git reset`, `rebase`, `cherry-pick`, `revert`, `merge` or `tag`; history belongs to the user.
- No AI attribution anywhere: no generated-with footer, no AI co-author trailer, no robot line in suggested commit messages, PR descriptions, PR bodies or comments.
- Never merge a PR, enable auto-merge, or approve your own PR. A yes to opening a PR is not a yes to merging; merging is a separate request (see `pr-create`).
