# P5b plan: commit commands

Tier: standard (a new profile key, two new commands, a chain step, a check in `dashcrew check`, catalog changes; no new dependency, no output layout change). Idea: `docs/ideas/knowledge-tracker-multirepo.md`, section P5b.

## Goal

Every member ends a task with a ready commit command in the team's pattern, so history reads the same whoever wrote it. When the member edits files by hand afterwards, a fresh command matches the new tree. Agents still never commit unless `policy.agent_commit` is on.

## What exists

- `core/rules/git-commit-off.md` (`policy.agent_commit: false`) currently says "Never commit. The user makes every commit; suggest a message only when asked." `git-commit-on.md` lets the agent commit one logical change, with only the task's files.
- `core/rules/git.md` forbids AI attribution in suggested messages.
- Profile `project.branch_prefix` maps `feat`, `fix` and `hotfix` to branch prefixes.
- The guard blocks `git commit` and `git push` for agents.

## Stages

### C0: profile

`project.commit`, with strict keys and defaults:

```yaml
project:
  commit:
    pattern: "<type>(<scope>): <subject>"     # placeholders: type, scope, subject, ticket
    types: [feat, fix, refactor, test, docs, chore, perf, build, ci]
    type_from_branch: { feat: feat, fix: fix, hotfix: fix }   # branch_prefix key -> type
    scopes: { "lib/features/cart/**": cart, "lib/core/**": core }   # glob -> scope, first match wins
    ticket: { from_branch: "([A-Z]+-[0-9]+)", trailer: "Refs" }       # optional
    subject_max: 72
    language: en
    body: optional            # optional | required | none
    trailers: []              # extra allowed trailers; P6 adds Ported-from
```

- With no `scopes` match, the scope is dropped along with its brackets (`fix: <subject>`). The member can still pass `--scope`.
- A profile without `project.commit` uses the defaults above minus `scopes` and `ticket`.

### C1: `dashcrew commit suggest`

Read-only. It reads the branch, the staged and unstaged changes, and the untracked files, then prints:

```
type:    fix            (branch fix/...)
scope:   cart           (3 of 4 files under lib/features/cart)
ticket:  ABC-123        (branch name)
files:   4 changed, 1 untracked (listed below, not included)
since verify: 1 file changed after the last verify   (only when known)

git add lib/features/cart/cart_cubit.dart lib/features/cart/cart_page.dart test/cart_cubit_test.dart
git commit -m "fix(cart): <subject>" -m "<body>" -m "Refs: ABC-123"
```

- The file list is explicit, never `git add -A` or `git add .`. Untracked files are listed for the member to add on purpose.
- `--subject "..."` and `--body "..."` fill the placeholders, and the result is linted before printing. The agent writes the subject and body; the CLI owns everything derivable.
- `--json` prints the derived parts for other tools (P8 bridge).
- The command is quoted so it pastes safely into zsh and bash. A subject with quotes or `$` is escaped.
- "Since verify": `verify` records the hash of the tree it checked in `<git common dir>/dashcrew/last-verify`. `suggest` compares the current tree with it. With no record, the line is left out.

### C2: `dashcrew commit lint`

- `dashcrew commit lint "<message>"` or `--range <a>..<b>` checks the pattern, allowed type, subject length, trailers, and the absence of AI attribution (generated-with footers, AI co-author trailers).
- Exit 1 on any failure, with one line per problem.
- `dashcrew check` runs it on the PR's commits (`--range <base>..HEAD` in CI, from the pull request event). Merge commits are skipped.

### C3: chain step and rules

- `git-commit-off.md`: "Never commit. At the end of every task, after the PR description, print the commit command from `dashcrew commit suggest` with your subject and body. When the user says they changed files, run it again and print the new command. When files changed since the last verify, say so and offer a scoped re-verify first."
- `git-commit-on.md`: the agent commits with the message `suggest` built, and `lint` passes before the commit.
- The chain table and summary step name the commit command after the PR description. AGENTS.md byte budget: pay for the added words with cuts in the same file. The heaviest combination stays at or below 23,791 bytes, enforced by the existing size test.
- The guard keeps blocking `git commit` and `git push` for agents when `agent_commit` is off. `dashcrew commit suggest` and `lint` are read-only, so the guard allows them.

### C4: verify record

`verify` records the tree hash it checked (`git write-tree` on a temporary index, so the user's index is never touched), via a small `dashcrew verify record` subcommand that the `verify` workflow calls at its end. `suggest` reads the record.

## How we check

- Unit tests:
  - profile defaults and errors;
  - type, scope and ticket derivation over table cases;
  - quoting with quotes, `$`, backticks and newlines;
  - lint cases (good, wrong type, long subject, AI trailer, missing required body);
  - `--range` over a temp repo;
  - the temporary-index tree hash leaving the real index untouched.
- Transcripts:
  - `commit suggest` on a temp repo with staged, unstaged and untracked files;
  - after a manual edit;
  - with and without a verify record;
  - `commit lint`;
  - `check` with a bad commit in range.
- Goldens: the changed rule text in every sample. Every AGENTS.md stays within budget.
- Live: in the sample app, finish one task and paste the printed command. Then edit a file by hand, ask for a new command, and confirm it lists the edited file and warns about the verify.

## Not doing

- Running `git commit` from the CLI on the member's behalf. The member pastes the command; `agent_commit: true` already covers teams that want the agent to commit.
- Writing the subject or body in the CLI. That is the agent's job, within the frame.
- Installing a git `commit-msg` hook. `check` in CI is the team-wide gate; a local hook may come later as opt-in.
- Rewriting history to fix old messages.

## Decisions (confirmed)

1. The default pattern is Conventional Commits `<type>(<scope>): <subject>`, and teams override it in `project.commit`.
2. `dashcrew check` fails on a PR commit that breaks the pattern.
3. With no derivable scope, the scope is dropped; `--scope` overrides.
4. No email address appears anywhere in a commit message: subject, body or trailers (co-author and sign-off included). `commit lint` rejects it, and `commit suggest` never emits one. `project.commit.allow_email_trailers` (default empty) lets a team that requires DCO allow `Signed-off-by` only.

## Progress

- C0-C4 done, plus decision 4 (no emails).
  - Commands: `commit suggest` (`--subject`, `--body`, `--type`, `--scope`, `--json`), `commit lint <msg> | - | --range a..b`, `verify record`, and `check --base` (a `commits` check). Package `internal/commit`; `profile.Commit`.
  - Derivation:
    - type from the longest `branch_prefix` match;
    - scope from the glob with the most files, ties by profile order;
    - ticket from the first regex group.
  - Output: single-quoted, checked through sh, bash and zsh. Untracked files are listed and never added. Emails in the branch or git config are masked.
  - Lint covers the pattern, the type, subject length, line 2 blank, placeholders, the body rule, trailers, emails (only `allow_email_trailers` exempt; `@mention`, `foo@`, `user@localhost` and `a@2x.png` are not emails), and AI attribution (generated-with footers, AI co-author trailers, robot lines).
  - Deviation: the verify record is per worktree (`git dir`), not the common dir, so parallel worktrees do not overwrite each other. It uses a temp `GIT_INDEX_FILE`; a test proves the real index is untouched.
  - The generated workflow uses `fetch-depth: 0` so CI can read the PR's commits.
  - Catalog: C3 text in `git-commit-off.md`; standard and large chains end with "-> PR description -> commit command". The heaviest AGENTS.md is 23,788 bytes.
  - README and CHANGELOG updated. 33 new tests, full suite passing.
