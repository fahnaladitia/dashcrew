# P6 plan: release lines

Tier: large (a new profile key, a new workflow, new commands, a sync behaviour change on some branches, a new check, catalog changes). Idea: `docs/ideas/knowledge-tracker-multirepo.md`, section P6.

## Goal

Teams that build two releases at once and insert a patch line while both are open can work without three recurring failures:

- generated toolkit files conflicting on every merge between lines;
- a patch PR carrying a toolkit upgrade;
- fixes that never reach the newer lines, or vanish in a later merge or revert.

## What exists

- Profile `project.base_branches` and `project.branch_prefix`; the branching skill always asks for the base.
- `sync` refuses a lock written by a newer dashcrew (unless `--force`). It does not refuse writing a newer one over an older one.
- `worktree` creates one worktree per task from any base.
- P5b: `commit suggest` and `lint`, with allowed trailers from `project.commit.trailers`.
- `check` runs toolkit, PR-section and commit checks.

## Design

**Profile** (strict keys, all optional):

```yaml
project:
  release_lines:
    main: develop                         # the main development line
    release: "release/(?P<version>[0-9]+\\.[0-9]+\\.[0-9]+)"   # regex, named group "version"
    hotfix:  "hotfix/(?P<version>[0-9]+\\.[0-9]+\\.[0-9]+)"
```

The patterns are the team's own. A team with `development/app-v2.4.0` writes `development/app-v(?P<version>...)`. Lines are ordered by version, so "oldest affected line" and "upward" are well defined.

**A task knows its release:**
- step 0 gains `Release: <version> | none`;
- the branch records it in `git config branch.<name>.dashcrew-release`. Branch config is local and not committed, the same mechanism the P3 gate used for its issue number.

### R0: profile and line resolution

- Parse `release_lines`.
- `internal/lines`:
  - lists line branches (local and remote-tracking) that match the patterns;
  - orders them by version;
  - resolves the line a branch belongs to: its own name if it is a line, otherwise the recorded release, otherwise the nearest line by merge-base, reported as a guess.
- `dashcrew release lines` prints the lines in order with their head commits.

### R1: toolkit version per line

- On a branch that is a release or hotfix line, or records a release, `sync` refuses to change any generated file when the change comes from a newer catalog than the lock's. That covers the binary version newer than `dashcrew_version` in the lock, and output that differs only because of catalog text.
  - It prints what would change and how to proceed: `dashcrew sync --upgrade`, or run the upgrade on the main line and merge it down.
  - Profile-driven changes (the team edited `.ai/project.yaml` on that line) are still written.
- The binary keeps rendering older locks' sync-free operations (`check`, `doctor`, `guard`, `commit`).
- `doctor` on a line branch reports its toolkit version against the main line's.

### R2: merges between lines

- `doctor` and `check` detect generated files that disagree with the lock, or that carry conflict markers. They suggest the fix: take either side of each generated file, run `dashcrew sync`, commit.
- `dashcrew sync --after-merge` does exactly that for files dashcrew owns. It resolves conflict-marked generated files by rendering them fresh, then lists them for the member to stage. It never touches files dashcrew does not own, and never stages or commits.

### R3: plans and ledger per release

- Plans for a task with a release live in `<plans>/<release>/<task>/`; tasks without one stay at `<plans>/<task>/`. The `plan` workflow and the PR description skill use the path.
- `dashcrew release status <version>` regenerates `<plans>/<version>/README.md`, the ledger. It lists:
  - the tasks in that folder, each with its branch and PR description path;
  - commits on the line since the previous version tag that carry `Ported-from:`;
  - pending ports from R4.

  It writes only that README and never runs git write commands.

### R4: ports

- Trailer `Ported-from: <short sha>` is allowed by default when `release_lines` is set (added to the P5b allowed trailers), and lint checks its shape.
- Workflow `port` (catalog, run by the main session):
  1. inputs: a merged commit range or PR on a source line, and the target lines (default: every newer line, then main);
  2. one worktree per target from that line, via the existing `worktree`;
  3. merge upward by default; cherry-pick with `-x` only when merging would bring unrelated changes, and the agent says why;
  4. rename detection on (`-X find-renames`), and conflicts reported with the renamed paths;
  5. scoped verify per target;
  6. one PR description per target naming the origin, plus the commit command from `commit suggest` with the `Ported-from:` trailer. The member commits and pushes, as always.
- `dashcrew check --ports [--from <line>] [--to <line>]`: every fix commit on the older line (a commit whose type is `fix`, or that the team marks) must appear on the newer line, by the `Ported-from:` trailer, a `-x` cherry-pick note, or an equal patch-id. Missing ones are listed with their subject. In CI it runs only when the profile asks for it (`release_lines.check_ports: true`), because it needs both lines fetched.

### R5: catalog

- Step 0 `Release:` line, paid for in AGENTS.md bytes, with the heaviest combination at or below the current 23,788.
- The branching skill asks for the release together with the base, and records it.
- Knowledge (P5) guidance: pages describe the main line; tag version-specific parts in place (`(since x.3.0)`, `(x.2 only)`), never folders per version. This goes in the knowledge-gated `learn` text and the README.
- README section "Release lines".

## How we check

- Unit tests with temp repos holding three lines (`x.2.0`, `x.2.5`, `x.3.0`) and a main line:
  - line ordering and resolution, including a branch that resolves by merge-base;
  - sync refusal on a line branch, and `--upgrade`;
  - `--after-merge` on real conflict markers in generated files;
  - ledger output;
  - `check --ports` finding a missing port, a ported one by trailer, one by `-x` and one by patch-id, and one lost to a revert.
- Transcripts for `release lines`, `release status`, `sync` refusal, `sync --after-merge` and `check --ports`.
- Goldens: the step 0 and branching text in every sample, within the AGENTS.md budget.
- Live: on a test repo, cut a patch line, fix a bug there, run `port` to the newer line and main, then run `check --ports`; revert the fix on the newer line and confirm `check --ports` reports it.

## Not doing

- Creating, tagging or deleting release branches.
- Pushing, merging or opening PRs. The member does, with the printed commands.
- Version folders for code, generated files or knowledge.
- Automatic port on merge (a bot or CI that pushes ports).
- Multi-repo releases. P9 tracks would extend `port` across repos.

## Decisions (confirmed)

1. A task's release comes from step 0 and is recorded in `git config branch.<name>.dashcrew-release`. When it is missing, the nearest line by merge-base is shown as a guess.
2. On a line branch, `sync` refuses catalog-driven changes unless `--upgrade` is given.
3. `check --ports` covers commits of type `fix` (P5b pattern), plus any commit with a `Port: yes` trailer.
4. `check --ports` in CI is opt-in, with `release_lines.check_ports: true`.

## Progress

- R0-R2 done.
  - Profile `project.release_lines` (`main` required; `release` and/or `hotfix` anchored regexes with a named group `version`; `check_ports`). It is kept out of templates.
  - `internal/lines`: lists local and remote-tracking lines and orders them by version. A branch resolves as the main line, then a line by name, then the recorded `branch.<b>.dashcrew-release`, then a merge-base guess. Only a line by name or a recorded release protects the branch. CLI: `dashcrew release lines`.
  - R1 rule (on `LineHold`, internal/sync/release.go):
    - On a protected branch without `--upgrade`, when the lock's version or catalog digest is older or different, each differing file is checked on its own.
    - A file is written as profile-driven when rendering the lock's profile with the current catalog reproduces its locked hash. Otherwise it is held as catalog-driven.
    - The lock gains `inputs` (catalog digest, profile hash) only when `release_lines` is set.
    - Held means exit 1, with the two ways forward.
  - R2:
    - `sync --after-merge` refuses a conflicted profile.
    - It rebuilds a conflicted lock from our side plus files only their side has, re-renders owned files that are conflicted or drifted, and prints `git add -- ...`. It never stages.
    - `lock` and `drift` checks point to it. Doctor gets a `release-line` check against the main line's lock.
  - Limits: the catalog digest covers only the built-in catalog, and adapter config merges (`.mcp.json`, `settings.json`) are not held.
  - Transcripts `release-lines`, `sync-on-a-release-line` and `sync-after-a-merge-between-lines`. 26 new tests. Goldens unchanged.
- R3-R5 done.
  - Ledger: `dashcrew release status <version>` (`internal/ledger`) rewrites only `<plans>/<version>/README.md`, then prints it. It has three sections: Tasks (folder, branch, PR description), Ported here, and Pending ports, for the pair coming into the line and the pair going out of it.
  - Ports (`internal/ports`):
    - The fix set is commits of type `fix` plus `Port: yes`, counted since the line left main's first-parent chain. Fixes reverted on their own line are dropped.
    - A fix counts as found by merge ancestry, then a `Ported-from:` trailer, then a `-x` note, then patch-id. Ports of ports chain back.
    - A fix is lost when an unreverted "This reverts commit" names it.
  - `check --ports` exits 1 on any missing or lost fix. Plain `check` adds `ports` only with `check_ports: true`.
  - `port` workflow: the guard blocks merge and cherry-pick for agents, so the agent prints them for the member. `commit suggest --ported-from` builds the trailer; lint allows `Ported-from` and `Port: yes` when `release_lines` is set.
  - Catalog: the `Release:` line in step 0, the branching question and record step, the plan path, pr-description, prompt-polish and the knowledge guidance, all gated on `has_release_lines`. README section "Release lines".
  - AGENTS.md: every golden shrank by 53 bytes. The heaviest combination is 23,735 (23,786 with release lines), and the budget test covers all four combinations.
  - New golden `sample-release-lines`. Transcripts `release-status-and-ports` and `sample-release-lines`. 19 new tests.
  - CI fix: the transcript bin folder holds a `gh` stub that exits 127, because Ubuntu runners ship `/usr/bin/gh`. `ci.yaml` uses `setup-go@v6`; the generated workflow still uses v5.
  - Known: a fast-forward merge of a line into main moves its branch point (documented in the README).

P6 is complete in code. Left: the live check in "How we check".
