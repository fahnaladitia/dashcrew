## 0.5.0

Readable chat blocks, skills a team can adopt and each member can switch on, and a guard that runs under Antigravity. A project's generated files change: run `dashcrew sync` and commit what it writes. In a project with the antigravity agent the shim then accepts dashcrew 0.5.0 or newer only, so every member installs 0.5.0 before that sync is committed.

- Readable chat blocks (#18): step 0, review, verify, learn, build-fix and the end-of-task summary print as plain-text blocks with one layout: a title line, a 72-column `=` rule, `Label  value` rows, then each list under its own `-- Name ---` rule, columns aligned with spaces so every value copies clean. A comparison is a table in that layout. Step 0 numbers its assumptions and questions (`A1`, `Q1`), lists skills, criteria and branches as rows, and ends with the reply it expects; with a question tool the agent asks through it. The review verdict comes first. The full step 0 template moves from AGENTS.md to the `prompt-polish` skill, and the `SUMMARY` template to `pr-description`, where tracker and analytics packs add their rows. The main session always asks an open question, with the agent's question tool when it has one, and waits.
- One source per chat block (#21): a platform overrides the prose around a block, never the block, so a layout change is made in one file; the `reviewer` role names the `code-review` verdict format instead of copying it, and the verify QA line reads the same on every platform.
- Project skills (#20): a skill the team adopts lives in `.ai/skills/<name>/` (`SKILL.md`, copied as written, never rendered, and `meta.yaml` with `description`, `when_to_use`, `review`, `optional`). A required one is written for every agent and joins the skill index. An optional one is written by `dashcrew me sync` only into the clone of a member who lists it in `.ai/me.yaml` `skills`, git-excluded, so no agent of another member can pick it; a `review` skill cannot be optional. A member tries a skill first in `~/.dashcrew/skills/<name>/` with `skills` in `~/.dashcrew/me.yaml`, linked for Claude Code. New `dashcrew skill adopt <name | folder> [--required]` copies it into the project.
- Supporting files (#22): a project skill's other files (`references/`, `scripts/`, templates) are written next to each agent's `SKILL.md`, with the source's executable bit; `me sync` tracks the files it writes for an optional skill in `.dashcrew-me` and removes only those.
- Skill limits warn (#23): a symlink, a pipe or device, a file over 1 MiB, or a file past the first 50 in a skill folder is left out with a `sync` and `doctor` warning instead of stopping every command; a symlinked `SKILL.md` or `meta.yaml` is still refused. `me sync` reads `.ai/skills` alone, so a broken project pack no longer blocks a member's optional skills.
- Antigravity guard (#25): agy runs a hook in the folder of its `hooks.json`, so `./.ai/bin/guard` in `.agents/hooks.json` was not found and the guard often did not run. The hook now finds the shim from `.agents/`, the workspace root, a project below the git toplevel or a checkout without git, and passes the policy. The guard answers every Antigravity call with a decision (`allow` hands the call on; agy fails closed without one) and also checks agy's other write tools (`create_file`, `edit_file`, `str_replace_editor` and more). `doctor` asks `agy -p /hooks` whether the guard is active and says to trust the folder when it is not. A project with the antigravity agent needs dashcrew 0.5.0 or newer: run `dashcrew sync` to rewrite `.agents/hooks.json` and the shim.
- The guard corpus record keeps its line order when the goldens are rewritten (#19).

## 0.4.0

Role and level per repo on top of the 0.3.0 personal profile, and a fix for release lines merged by fast-forward. Nothing changes for a project or its guard.

- Role and level per repo (P10c): `~/.dashcrew/me.yaml` keeps what describes you (stacks, chat language, agents, default role and level); a repo's `.ai/me.yaml` holds your `role` and `level` there when they differ, listed in `.git/info/exclude`. `dashcrew me init` inside a repo asks only those two once the home profile exists. `me sync` writes each repo's file for Claude Code (`.claude/rules/dashcrew-me.md`) and Antigravity (`.agents/rules/dashcrew-me.md`), and an "In the repository at" section for Codex; personal skills are linked when home or any repo selects them. New `me sync --remove-repo` and `--prune`; task worktrees link the personal files from the main checkout like the rest of the git-excluded toolkit; `doctor` shows the effective role and level with their source and fails when git tracks a personal file.
- `check --ports` and the release ledger no longer lose a line's branch point when the line is merged into the main line by fast-forward (the main line had not moved since the cut): the older merge-base with a newer line wins, so the line's fixes are still checked instead of passing silently.

## 0.3.0

The personal profile: how agents work with each member, outside every repo. Nothing changes for a project or its guard; `doctor` adds one `me` line.

- Personal profile (P10b): `~/.dashcrew/me.yaml` (`role`, `stacks`, `level`, `chat_language`, `agents`), never committed and never read by the guard. `dashcrew me init` proposes it from the repo's platform, `me show` prints it, and `me sync` writes personal instructions outside every repo: a Claude Code user rule, a marked block in the Codex user AGENTS.md, an Antigravity `always_on` rule. They cover work outside your stacks, `level: new`, a personal chat language and `role: lead`. Claude Code also gets the personal skills `dashcrew-onboarding` and `dashcrew-lead`, linked from `~/.dashcrew/skills/`. A file dashcrew did not write is never overwritten; `me sync --remove` takes everything out. `doctor` adds a `me` check that warns, never fails.

## 0.2.1

Fixes from the live checks of P5, P6 and P7 on a sample service and a private knowledge repo.

- Knowledge (found in the P5 live run): a proposal's `target` may be written as the page path (`features/orders.md`, `repos/shop-api.md`) when its folder matches `kind`; the learn skill now asks for the slug and a section the page has. A page `knowledge review` creates keeps only the sections a proposal filled, instead of the template's empty headings.
- `knowledge review` fetches first and stops when the clone is behind its upstream, so a review on stale pages does not turn into a conflicting pull request (`--no-fetch` skips it; offline it only warns). The README and the knowledge repo's own README describe the team flow: members send `inbox/` files, one member reviews at a time.
- Questions are asked when they come up (found in the P7 live run, where a spec was handed over with a question the user could have answered): the `spec` workflow gains a questions round before lint and hand-over; a plan ends with a `Questions:` line that makes the verdict NO-GO on every tier; `tdd` stops and asks when the code contradicts the plan or the spec, a criterion reads two ways, or the task needs an unplanned change; the reviewer's open questions go to the user with the MEDIUM and LOW findings; the principle says a question found after step 0 is asked when found.
- `dashcrew spec revise` and `spec lint` take a path that starts with `specs/`, as a plan's `Spec:` line names it, from the knowledge clone when it is not found from the current folder (found in the P7 live run); `revise` gains `-p` and `--clone`.
- `check --ports` reports a fix as lost when the newer line reverts the merge that brought it in (`git revert -m 1` of a port merge), which ancestry alone read as merged (found in the P6 live run).
- Commands handed to the user go on one line: `dashcrew commit suggest` prints `git add -- ... && git commit ...`, and the output rule says that commands run together are joined with `&&`, paid for by shorter wording in the same section.

## 0.2.0

Upgrade note: a project whose platform declares `guard:` rules requires dashcrew 0.2.0 in its guard shim after `dashcrew sync`; every teammate on that project upgrades first. Other projects keep accepting 0.1.x.

- Multi-repo work (P9), for any layout and any way an app consumes a dependency. `project.links` lists the dependencies a task may point at a local checkout. `dashcrew link add/ls/remove` writes the platform's link file (`pubspec_overrides.yaml`, or `go.work` through `platform_config.link`) after checking the package name, and excludes it from git; `doctor` reports a link file git would commit, a committed one, missing checkouts, and linked checkouts on another branch (info).
- The guard checks a file under the policy of the dashcrew project holding its real path, so a linked dependency keeps its own rules. A hook running in a folder above several projects checks shell commands under all of their policies.
- `dashcrew check` adds `links` with `project.links`: a git ref must be a tag or a commit on the dependency's default branch, a path must stay in the repository, a local Go `replace` and a committed link file are refused, and an unreachable dependency repository fails. `check --links` runs it alone.
- `check --links` refuses a `pubspec.lock` that resolves a linked package from a path, as `pub get` writes it while the link is active (found in the P9 live run); `link add` says so, and the branching skill runs `dashcrew link remove` before the commit command.
- With `project.links`, the branching, pr-description and verify skills get the cross-repo steps and the PR template a `## Dependencies` section. AGENTS.md does not change.
- Go as a second platform (P10), as a starter rather than a built-in: `dashcrew platform new go` writes `.ai/platforms/go/` (rules, review skill, service skills, http and db packs, `link: go.work`) for the team to own and edit. `init` mentions it on a Go module.
- Platforms may declare backend guard rules with a `guard:` key (`commands`, `sql`, `migrations`): deploy and infrastructure commands, `DROP`/`TRUNCATE` through SQL clients, and migrations against a non-local host are blocked for agents. A project whose policy has them requires dashcrew 0.2.0 in the shim and the lock; other projects keep 0.1.0.
- Tracker (P11): a `github` tracker pack for GitHub Issues, with status in a Projects field (`mode: project`) or in labels (`mode: labels`), PRs linked with `Refs #<n>` by default. With any tracker, the `branching` and `pr-create` skills carry the status flow as explicit steps; AGENTS.md is unchanged.
- The output rule names placeholder words too (`OWNER`, as well as `<x>`): a command handed to the user holds real values, or the agent asks first. Found in the P10 live run.

## 0.1.1

- `ci.dashcrew_ref` must be a release tag, `v0.1.0` or newer. A commit or branch is refused when the profile is read, because CI installs dashcrew with `go install`: a branch moves, and a commit from before 0.1.0 has no Go module, so the workflow failed before `dashcrew check` ran. A project migrated from 0.0.1 still holding a commit gets the error from `sync` and `doctor` with the fix.
- The output rule now says what copy-paste safe means: no `<placeholder>` in a command the user runs, since a shell reads `<` as a redirect and the whole block fails. Paid for by shorter wording in the same section; the heaviest AGENTS.md is 6 bytes smaller.

## 0.1.0

- The engine is Go: one static binary per OS with the catalog built in, installed with `brew install fahnaladitia/tap/dashcrew` or `go install`. Output is byte-identical to the Dart engine of 0.0.1.
- Removed: the Dart engine (`lib/`, `bin/`, `pubspec.yaml`) and the `dart pub global activate` install. An old Dart install still on `PATH` is refused by the guard shim and reported by `doctor`.
- `dashcrew guard` is a subcommand. `.ai/bin/guard` runs it from `PATH` and refuses every tool call (fail closed) when dashcrew is missing or older than the lock's `guard_min_version` (0.1.0). The lock's `guard_source` hash is replaced by `guard_min_version`.
- Removed: the per-machine compiled guard in `~/.dashcrew/guard/`, `dashcrew guard install`. `sync --[no-]compile-guard` is a deprecated no-op.
- `dashcrew --version`. Release builds stamp their tag; `go install` builds report their module version.
- The generated GitHub workflow installs dashcrew with `go install` at `ci.dashcrew_ref`.
- `doctor` `guard-binary` fails without dashcrew on `PATH` and warns when the first one is an old Dart install or older than the minimum.
- `dashcrew commit suggest` prints a ready `git add` and `git commit` command in the team's pattern (`project.commit`, Conventional Commits by default), and `dashcrew commit lint` checks a message or a range, email addresses and AI attribution included. `dashcrew check` lints the PR's commits (`commits`, `--base`), and the generated workflow fetches the full history. `dashcrew verify record` stores the verified tree, built in a temporary index; `suggest` lists files changed after it. The git rule and chain now end every task with the commit command.
- Repo hardening: `go.mod` pins the go1.26.10 toolchain (standard library fixes found by `govulncheck`); CI pins every action to a commit SHA, sets read-only permissions, and adds a Linux, macOS and Windows test matrix (Windows may fail), `gofmt`, `go test -race`, `govulncheck` and CodeQL; Dependabot updates Go modules and actions weekly. SECURITY.md, CONTRIBUTING.md, CODE_OF_CONDUCT.md, issue templates and a pull request template are added. Tests that need a POSIX `sh` skip on Windows.

## 0.0.1

- Repository bootstrap.
