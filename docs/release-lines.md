# Release lines

For teams that build two releases at once and cut patch lines while both are open. Set the lines in the profile; the patterns are the team's own branch names, each with a named group `version`, matched against the whole branch name:

```yaml
project:
  release_lines:
    main: develop
    release: 'release/(?P<version>[0-9]+\.[0-9]+\.[0-9]+)'
    hotfix: 'hotfix/(?P<version>[0-9]+\.[0-9]+\.[0-9]+)'
    check_ports: false   # true runs the port check in dashcrew check (CI)
```

**Lines.** `dashcrew release lines` lists local and remote-tracking line branches ordered by version (release before hotfix on a tie), then the main line. A branch resolves as the main line, then a line by its name, then the release it records in `git config branch.<name>.dashcrew-release`, then the nearest line by merge-base, shown as a guess. Only a line by name or a recorded release protects a branch.

**A task knows its release.** Step 0 gains a `Release` row (`<version> | none`), and the `branching` skill asks for it with the base and records it with `git config branch.<name>.dashcrew-release <version>`, local config the guard allows and nothing commits. The step 0 row, the `port` workflow and the release text in `branching`, `prompt-polish`, `plan`, `pr-description` and `learn` render only when `release_lines` is set.

**Toolkit version per line.** On a protected branch without `--upgrade`, when the lock's version or catalog digest is older or different, `sync` checks each differing file on its own: a file is written as profile-driven when rendering the lock's profile with the current catalog reproduces its locked hash; otherwise it is held as catalog-driven. Held files mean exit 1, with the two ways forward: `dashcrew sync --upgrade` on the line, or upgrade on the main line and merge it down. With `release_lines` set the lock records `inputs` (catalog digest, profile hash). `doctor`'s `release-line` check compares the line's toolkit with the main line's. Limits: the catalog digest covers only the built-in catalog, and adapter config merges (`.mcp.json`, `settings.json`) are not held.

**Merges between lines.** `doctor` and `check` report generated files with conflict markers or out of step with the lock. `dashcrew sync --after-merge` refuses a conflicted profile, rebuilds a conflicted lock from our side plus the files only their side has, renders fresh the owned files that are conflicted or drifted, and prints the `git add -- ...` line. It never stages or commits, and never touches files dashcrew does not own.

**Plans and the ledger.** A task with a release plans in `<plans>/<release>/<task>/`; one without stays in `<plans>/<task>/`. `dashcrew release status <version>` regenerates `<plans>/<version>/README.md`: each task folder with its branch (a local branch named `<task>` or ending in `/<task>`, one recording the release first) and PR description path; the commits on the line since the previous release's line or version tag (`2.4.0` or `v2.4.0`) that carry `Ported-from:`; and the pending ports into and out of the line.

**Ports.** With `release_lines` set, `commit lint` allows the trailers `Ported-from: <sha>` (7 to 40 hex characters) and `Port: yes`. `dashcrew check --ports` takes every fix of the older line since its branch point (the newest commit of its history on the main line's first-parent chain, so merging the line back does not move it): commits whose type is `fix` in the commit pattern, plus any commit with `Port: yes`, minus fixes reverted on that line. Each must reach the newer line, found in this order: merged (in its history), a `Ported-from:` trailer, a `cherry-pick -x` note, an equal `git patch-id --stable`. A port of a port counts, since the trailers and notes chain back to the original. A found fix is lost when a commit on the newer line says `This reverts commit <sha>` of it and is not reverted itself. It prints one line per fix, a summary, and exits 1 when any is missing or lost. Without `--from` and `--to` it checks every adjacent pair up to the main line, which covers older lines through the newer ones. A fast-forward merge of a line into the main line puts its commits on that chain, so an older merge-base with a newer line wins; only a line both fast-forwarded into the main line and merged into the newer line loses its cut. A CI checkout needs `fetch-depth: 0` with every line fetched.

**The `port` workflow.** The main session takes a merged range or PR on one line and the target lines (default every newer line, then main), creates one worktree per target with `dashcrew worktree create`, and prints the merge for the member to run: `git merge --no-ff --no-commit -X find-renames <source line>` by default, `git cherry-pick -x --no-commit` only with a stated reason. The guard keeps merge and cherry-pick the user's in every policy. Conflicts are listed with renamed paths, `verify` runs scoped per target, each target gets its own PR description naming the origin, and the commit command comes from `dashcrew commit suggest --ported-from <sha>`. The member commits and pushes.

**Knowledge.** Pages describe the main line. A version-specific part is tagged in place, such as `(since 2.5.0)` or `(2.4 only)`; never a folder or page per version. The knowledge-gated `learn` text says so when `release_lines` is set.

Not done: creating, tagging or deleting release branches; pushing, merging or opening PRs; version folders for code, generated files or knowledge; automatic ports on merge; releases across repos.
