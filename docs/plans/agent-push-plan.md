# Agent commit and push, on the user's answer at the end of each task

Tier: large. The guard's decisions change: push becomes allowed under a new policy, with new protections. The guard policy file gains a key, the guard minimum rises for projects that opt in, and generated rules change for projects with `policy.agent_commit: true`. Goldens are updated and reviewed before merge.

Target: v0.8.0, beside the Cursor adapter. It does not change the default output.

## Goal

At the end of every task, the agent asks the user with its question tool:

- **"Commit and push for you"**: the agent commits the task's files with the message `dashcrew commit suggest` builds (after `dashcrew commit lint` passes), then pushes the task branch.
- **"Do it yourself"**: the agent prints the commit command from `dashcrew commit suggest` and the push command, ready to run.

The answer covers that round only. The agent never commits or pushes unasked.

## Today

- `policy.agent_commit: false` (default):
  - rule `core/rules/git-commit-off.md` prints the commit command;
  - the guard blocks `git commit`.
- `policy.agent_commit: true`:
  - rule `core/rules/git-commit-on.md` says "Commit only when the user asks…";
  - the guard allows `git commit`.
- **Push:**
  - always blocked: `push` is in `guard.AlwaysBlockedGit` (`internal/guard/policy.go:22`);
  - `core/rules/git.md` says "Never push. The user pushes".
  - "The guard blocks the agent's `git push`" is also the launch story in `docs/ideas/launch-direction.md`.
- **Golden samples:** none sets `agent_commit: true`.

## Design

### Profile

- **New key `policy.agent_push`**, default `false`.
- **`agent_push: true` requires `agent_commit: true`.** Otherwise the profile fails with "policy.agent_push needs policy.agent_commit: true".
- **Unchanged** default output and `agent_commit: true` without `agent_push`, apart from the end-of-task question (see Rules).

### Rules (generated AGENTS.md)

- **`git-commit-off.md` (default):** unchanged, byte for byte.
- **`git-commit-on.md` (`agent_commit: true`)** becomes the end-of-task question:
  - "At the end of every task, after the PR description, ask with your question tool: commit {{and push}} for you, or yourself.
  - For you: one logical change per commit, only the task's files, the message from `dashcrew commit suggest` once `dashcrew commit lint` passes{{, then push the task branch: `git push -u origin <branch>` the first time, `git push` after}}.
  - Yourself: print the commit command{{ and the push command}}.
  - The answer covers that round only; never commit{{ or push}} unasked."
- **`git.md`:** the first bullet depends on the policy:
  - "Never push…" stays when `agent_push` is false;
  - with `agent_push: true` it becomes "Push only the task branch, only on the user's answer at the end of a task; never force, never delete a remote branch, never push a base branch (the guard blocks these)."
- **`core/workflows/port/10-port.md`:** its `agent_commit` sentence gains the push case.

### Guard

With `agent_push: true`, `push` leaves `blocked_git`, and a new optional key in `.ai/guard.json` keeps the dangerous forms blocked:

```json
"push": { "protected": ["develop", "main"] }
```

`protected` is `project.base_branches`. Older `.ai/guard.json` files have no `push` key, and the guard reads that as "push blocked as before".

A `git push` (behind the same wrappers, nesting and global options the guard already unwraps) is still blocked when:

- it forces: `-f`, `--force`, `--force-with-lease`, `--force-if-includes`, or a refspec starting with `+`;
- it deletes: `-d`, `--delete`, or a refspec starting with `:`;
- it pushes everything: `--all`, `--mirror` or `--tags`;
- a destination is protected: an explicit refspec whose destination (after `:`, or the whole refspec) names a protected branch, as `main`, `refs/heads/main` or `HEAD:main`;
- it has no refspec and the current branch is protected. The guard reads the branch from the worktree's `HEAD` file, following a `.git` file to the worktree's git dir, and starts no `git` process (the latency budget). A detached HEAD with no refspec is blocked: the destination is unknown.

Each block carries a reason, for example "pushing to main is reserved for the user; push your task branch". Unknown push options are allowed, as unknown options are today.

**Guard minimum.** A project with `agent_push: true` gets a `guard_min_version` of the release that ships this. An older `dashcrew guard` would read `push` missing from `blocked_git` and allow every push, force included. The shim fails closed on an older binary. Projects without `agent_push` keep their current minimum, so they need no new binary.

### Docs

- README:
  - policy table (`policy.agent_push`);
  - guard section (what push still blocks);
  - Git rules description.
- `docs/catalog-format.md`: the `when` keys table.
- CHANGELOG: one entry for the new option, and one for the changed `agent_commit: true` behavior: a re-sync for those projects only.

## Tests (written first)

`internal/profile`:
- `agent_push` defaults to false.
- `agent_push: true` without `agent_commit: true` is an error with that message.

`internal/guard`, with a policy that has `push.protected: [develop, main]`. Each case is also run behind `sh -c`, `env`, `-C dir` and `&&`.

Allowed:
- `git push`, with the current branch `feat/x` read from a temp repo's `HEAD`;
- `git push -u origin feat/x`;
- `git push origin HEAD:feat/x`.

Blocked:
- `git push --force`, `-f`, `--force-with-lease`, `--force-if-includes`;
- `git push origin +feat/x`;
- `git push origin --delete feat/x`, `-d`, `:feat/x`;
- `git push --all`, `--mirror`, `--tags`;
- `git push origin main`, `refs/heads/main`, `HEAD:main`, `feat/x:develop`;
- `git push` while on `main`, in a linked worktree on `main` (the `.git` file case), and on a detached HEAD.

Unchanged:
- A policy without `push` blocks every `git push`, as today.
- Corpus outcomes do not change for any existing case. The corpus policies have no `push` key.

`internal/guard/policy.go`: the compiled policy for `agent_push: true` drops `push` from `blocked_git` and writes `push.protected` from `base_branches`; for `false` it is byte-identical to today.

`internal/render` / conformance:
- A new golden sample `sample-agent-push` (`agent_commit: true`, `agent_push: true`, two base branches) covers the rules, `.ai/guard.json` and the lock's `guard_min_version`.
- No existing golden changes.

`cmd/dashcrew` transcripts: none should change.

## Verify

```bash
gofmt -l .
go vet ./...
go test ./...
DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard && git diff --stat test/golden cmd/dashcrew/testdata internal/guard/testdata
```

Expected golden diff: only the new `test/golden/sample-agent-push/`.

Live check:

- In a scratch repo with `agent_push: true` and a real Claude Code session, finish a task and answer "for you". The agent commits and pushes the branch to a local bare remote.
- In the same session, ask it to `git push origin main` and to `git push --force`. The guard blocks both.

## Not in this plan

- **Opening the PR.** `pr-create` keeps its own yes.
- **Changing the default.** `agent_commit` and `agent_push` stay false, and the launch story ("the guard blocks the agent's `git push`") stays true by default.
- **Push to a remote other than the task branch's upstream** (forks, several remotes). The protection is by branch name on any remote.
- **A push suggestion in the default (`agent_commit: false`) rule.** It would change every golden and make every project re-sync; see question 1.

## Decisions (maintainer, 2026-11-17)

1. The default rule stays as it is: no push suggestion when `agent_commit` is false.
2. All the guard protections.
3. Existing `agent_commit: true` projects switch to the end-of-task question.
4. v0.8.

## Change from the design: a push must name its branch

The design read the current branch from the worktree's `HEAD` for a bare `git push`. The guard judges each command's words and cannot see a `cd` earlier in the same script. So `cd ../main-checkout && git push` would be judged against the hook's folder, on a task branch, and pushed `main`.

Instead, a push that names no branch is blocked: `git push`, `git push origin`, `git push origin HEAD` and `@`. The rules tell the agent to push with `git push -u origin <branch>`. This is stricter and simpler, and the guard reads no file.

## Result

- **Profile** (`internal/profile/profile.go`): `policy.agent_push`, default false. It must be a bool and needs `agent_commit: true`, checked in `checkAgentPush`.
- **Guard.**
  - `internal/guard/push.go`: `PushRule` and its check.
  - `CompilePolicy` drops `push` from `blocked_git` and sets `Push.Protected` from `project.base_branches`.
  - `ToJSON` writes `push`; `LoadPolicy` and `PolicyFromJSON` read it, and a malformed one is an error.
  - `checkCommand` applies the rule after the subcommand (`gitSubcommandAt`).
  - `Policy.Minimum` raises to `version.PushGuardMinimum` (0.7.0).
- **Rules.**
  - `core/rules/git.md` (first bullet).
  - `git-commit-on.md` (the end-of-task question).
  - `core/workflows/port/10-port.md`.
  - `core/skills/pr-create/pr-create.md`.
  - `core/skills/branching/branching.md`.

  Every one renders the default text unchanged.
- **Tests.**
  - `internal/guard/push_test.go`: 7 allowed and 24 blocked forms, each bare, behind `sh -c`, `env`, `cd … &&` and `git -C`. Also covered: no push rule keeps every push blocked, compile and JSON round trip through both readers, and a malformed `push` key.
  - Profile defaults and two error paths.
- **Goldens.**
  - New `test/golden/sample-agent-push` (claude-code and codex, base branches develop and main).
  - Every other golden's generated text is unchanged. Only the catalog digest in `sample-release-lines` moved (any catalog source change does that), and with it the lock hashes in three release-line transcripts.
  - `internal/guard/testdata` is unchanged.
- **Docs:** README (profile example, policy table, guard list), `docs/catalog-format.md`, and two CHANGELOG entries.
- **End-to-end** (in place of a live agent session): the generated Claude Code hook, run through `sh` with the real guard built as 0.7.0.
  - `git push -u origin feat/login` exit 0.
  - `git push origin main`, `git push --force origin feat/login` and `git push` exit 2 with their reasons.
  - `git commit` exit 0, `git reset --hard` exit 2.
  - The same shim with dashcrew 0.6.0 on PATH refuses the call (exit 2, install line).

Known limits:

- **`PushGuardMinimum` is 0.7.0, the next release.** If this ships later, raise it to that release.
- **A `0.7.0-rc.N` build is older than 0.7.0 in semver,** so the shim of an `agent_push` project refuses it. The rc live run in `docs/release-checklist.md` can cover `agent_push` only once the final 0.7.0 exists, or by building `main` with `-X …version.Version=0.7.0` as the end-to-end check did.
- **The guard judges words, not the remote's real refs.** `git push origin feat/x:main` is caught, but a remote whose `push` refspec config maps a task branch onto `main` is not. That needs `remote.<name>.push` set by hand and is out of scope.
