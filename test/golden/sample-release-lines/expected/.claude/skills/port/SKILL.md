---
name: port
description: "Carry merged fixes from an older release line to every newer line and the main line - one worktree per target, merge upward by default, a PR description and commit command per target."
argument-hint: "<commit range | PR> [<target line>...]"
---

# port

The main session runs this workflow, outside the chain. It carries fixes that are merged on an older release line (`project.release_lines`) to the newer lines. `dashcrew release lines` lists the lines oldest first.

## Inputs

- The source: a merged commit range (`<base>..<head>`) or a PR, on one line. Read its commits with `git log --format='%h %s' <range>`.
- The targets: the lines named by the user, or by default every newer line, then the main line. Print the list and wait for the user to confirm or edit it once.
- `dashcrew check --ports --from <source line>` shows what is still missing: commits of type `fix`, and any commit with a `Port: yes` trailer. Port those, not more.

## One worktree per target

For each target, oldest first: `dashcrew worktree create fix/port-<slug>-<version> <target line>`, per the `worktree` workflow. Record the target's release with `git config branch.<branch>.dashcrew-release <version>`; skip it for the main line.

## Bring the change

Merging and cherry-picking are the user's: print the exact command, wait until the user reports it ran, then read the result.

- Default, merge upward: `git merge --no-ff --no-commit -X find-renames <source line>`.
- Cherry-pick only when a merge would bring unrelated changes (other fixes not meant for this target, release-only version bumps); say which ones. Then `git cherry-pick -x --no-commit -X find-renames <commit>...`, oldest first.
- Conflicts: list each file from `git status --porcelain`, and for a file renamed on the target the old and new path (`git diff --name-status -M <source>...<target>`). Resolve only conflict hunks of this port; ask when a resolution changes behavior.

## Verify and hand over

- Run `verify` scoped to the files this port touched in the worktree.
- One PR description per target branch (`pr-description`): the Summary names the origin line, the commits or PR, and how they came (merge or cherry-pick and why).
- The commit command comes from `dashcrew commit suggest --ported-from <short sha>[,<short sha>...]`, which adds a `Ported-from:` trailer per origin commit. The user commits, pushes and opens the PR.
- After the ports land, `dashcrew check --ports` must list none missing or lost; `dashcrew release status <version>` refreshes each release's ledger.
