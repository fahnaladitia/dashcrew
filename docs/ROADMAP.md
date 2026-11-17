# dashcrew roadmap to 1.0.0

Status as of 2026-11-17, at v0.6.0. Dates are estimates for one full-time maintainer and move with feedback.

The direction behind this order is in `docs/ideas/launch-direction.md`: build for teams that mix agents, lead with the guard, launch once Cursor is supported, and freeze 1.0 after real users have shaped it.

## Where dashcrew stands

### Facts

- 18 days old, 139 commits, 9 tags (v0.1.0 to v0.6.0), 3 of them hotfixes (0.1.1, 0.2.1, 0.5.1).
- About 46k lines of Go in 33 `internal/` packages; one `cmd/dashcrew`.
- `gofmt`, `go vet`, `go test ./...` and `go test -race ./...` are all green.
- Coverage: `internal/*` 75-97%, `cmd/dashcrew` 40.7%. 43 end-to-end transcripts run the real binary. The guard corpus records 8.7k decisions.
- GitHub: 1 star, 0 forks, no topics, no homepage, default social preview, Discussions off, 1 open issue (#30).
- Used daily by one team (the maintainer's office team).

### What "unstable" means here

The code is not fragile. The test suite is broad and green. What is unstable is everything around it:

1. **The contract moves every release.** Three minors required "run `dashcrew sync` and commit" (0.2.0, 0.5.0, 0.6.0). `guard_min_version` rose twice, and the shim fails closed, so a teammate on an older binary has every tool call refused. The lock format changed without a schema bump (`guard_source` became `guard_min_version`). Lock goldens were rewritten 20+ times.
2. **Bugs are found after release, in live agent runs.** CHANGELOG 0.2.0 and 0.2.1 list fixes "found in the P5/P6/P7/P9/P10 live run". Goldens pin bytes, not agent behavior.
3. **The guard can stop protecting without anyone noticing.** It happened twice: the Codex hook's escaped `$` (55ba40d) and the Antigravity hook path (a67453b). The guard fails open on read or parse errors by design (`internal/guard/run.go:30,45`), and nothing tells the user it is not running.
4. **Sync is not crash-safe.** Files are written in place (`internal/projectfiles/projectfiles.go:81`) and the lock is written last (`internal/sync/sync.go:325`). An interrupted sync can leave a truncated file, and files it did write no longer match the old lock, so the next sync refuses until `--force`.
5. **Windows is shipped but not supported.** Release builds Windows zips (`.goreleaser.yaml:17`), but the guard shim is `#!/bin/sh`, the README says Windows is unsupported (`README.md:394`), and transcripts skip Windows. The README says Windows CI "is allowed to fail" (`README.md:583`), but `ci.yaml` has no `continue-on-error`.
6. **The first five minutes do not sell it.** The README is 595 lines with no demo. It still says "no tagged release yet" (`README.md:9`, `:47`), and SECURITY.md still has a "before the first tagged release" row. `init` on a Go project ends with exit 1 unless the user finds `dashcrew platform new go`. A fresh Flutter project gets a `doctor` warning (`skills not found locally: idea-refine`).

### Strengths to build on

- One Go binary, attested releases, no Node runtime.
- A cross-agent git-flow guard with nested-shell unwrapping and a large decision corpus.
- `doctor` and the lock detect drift; `check` gates PRs in CI.
- Team profiles plus a personal profile (`me`).
- Commit lint, release lines, port checks and knowledge repos in the same tool.
- Byte-exact, golden-tested output.

### Gaps against similar tools

- **Agent reach.** rulesync and Ruler target 15+ agents; dashcrew targets 3 (Claude Code, Codex, Antigravity). No Cursor, Gemini CLI, Copilot, Windsurf or OpenCode.
- **Importing existing setups.** Only `sync --adopt` for an existing AGENTS.md.
- **Stack framing.** It is perceived as Flutter-first. Go is a starter (`starters/go`), not a built-in platform.
- **Safety.** The guard covers git flow only. `rm -rf ~`, `cat .env` and `curl … | sh` pass. That is by design, but visitors will read "guard" as a safety guard.
- **Install channels.** Only Homebrew and `go install`. No install script, scoop or winget.

## What 1.0.0 promises

1.0.0 is a promise, not a feature count. From 1.0.0 on, semver covers:

- **CLI:** command names, flags and exit codes (0, 1, 2 for a guard block, 64 for usage errors).
- **Files:** the schema of `.ai/project.yaml`, `~/.dashcrew/me.yaml`, `.ai/lock.json` and `.ai/guard.json`.
- **Guard decisions:** the same policy gives the same allow or block.

Upgrades follow these rules:

- A patch release never requires a re-sync.
- A minor release may ask for a sync, but a project synced by 1.N keeps working with a 1.(N-1) binary on a teammate's machine for at least one minor.
- Breaking changes wait for 2.0 and go through a deprecation warning first.

Generated Markdown text is not covered: wording in rules and skills may improve in any minor.

## Milestones

```
v0.7  Trust          2026-11-28  sync, guard and release process stop surprising users
v0.8  Launch-ready   2026-12-19  front page, guard init, Cursor, install; soft launch
      Launch         2027-01-12  Show HN, Reddit, X
v0.9  Reach          ~2027-02    Gemini CLI, Copilot, Windows, what launch users ask for
v1.0  Freeze         ~2027-03    public surface cleaned up, migration, RC soak
```

### v0.7.0 "Trust" (target 2026-11-28)

Goal: no release breaks a project, and a broken guard is never silent.

- **Crash-safe sync** (plan: `docs/plans/v0.7-crash-safe-sync-plan.md`)
  - [x] Write every generated file as temp file + `fsync` + rename in `projectfiles`; same for `me sync` writes to `~/.claude`, `~/.codex` and the registry.
  - [x] A locked file that already holds the rendered content is not a conflict, so the next plain `sync` finishes an interrupted one.
- **Guard you can trust** (plan: `docs/plans/v0.7-guard-trust-plan.md`)
  - [x] `dashcrew check` runs the guard hook checks of every agent, so a project's CI sees a hook lost to a hand edit or a merge. (Running hook commands through `sh` was already covered by `internal/adapters/hook_command_test.go`.)
  - [x] Log the fail-open causes the guard did not log (a named policy missing, unreadable, invalid or not applicable), so `guard-log` reports them.
  - [x] Pin the SECURITY.md blind spots (`$` as first word, `source`, `.`, `fish -c`, `stdbuf`, `doas`, deep nesting) in a test, so a future fix updates the docs.
  - The in-session notice when the guard fails open moves to v0.9, with the Cursor adapter.
- **Windows investigation** (1-2 days, no shipped change required)
  - [ ] Find which shell Claude Code, Codex and Cursor run hooks with on Windows, and whether the `sh` shim already works there.
  - [ ] Decide between keeping the shim, adding a `.cmd` shim that still fails closed, or stopping Windows archives until v0.9. A hook that calls `dashcrew` directly is not an option: a missing binary exits 127, which agents do not treat as a block.
  - [ ] Make the README and CI agree (`continue-on-error` and "experimental", or green).
- **Known bugs and dropped errors**
  - [x] #30: not reproduced on v0.5.0 or `main`. A regression test now puts HEAD one commit ahead of the base; `worktree create` branches from the resolved base commit and stops, naming both commits, when the new worktree lands elsewhere, as a `post-checkout` hook can make it (#44). The reporter was asked for their hooks and git config.
  - [x] Fix the ignored read error in `worktree.go:438`, which could rewrite the git exclude file without the user's lines. The other dropped errors in the audit (`deps.go`, `sync.go:749`, `commit/git.go`, `projectskill/adopt.go`, `usage`, `doctor`, `main.go:204`) were reviewed: best-effort cleanup, tolerant parsing of undocumented formats, or report-only walks, so they stay.
- **Release process**
  - [x] A goreleaser snapshot build on every PR (plan: `docs/plans/v0.7-release-process-plan.md`). `goreleaser check` cannot gate PRs: it exits 2 on the `brews` block, which stays on purpose because a formula installs on Linux too.
  - [x] Keep `-rc` tags off "Latest" and out of the Homebrew tap (`release.prerelease: auto`, `skip_upload: auto`).
  - [x] An upgrade job: the previous release syncs a project, the current build re-syncs it without `--force`.
  - [x] Add `staticcheck` to CI; run the existing fuzz targets nightly for 3 minutes each. (First staticcheck findings: see the PR's CI run.)
  - [x] A pre-tag live-run checklist (`docs/release-checklist.md`): init + sync + a real session in each supported agent, guard probe, upgrade from the previous tag. Tag a `-rc.1` first for any minor that changes generated output.
- **Ask the office team.** 15 minutes with each teammate: their last three frustrations with dashcrew. Anything that is not on this roadmap gets an issue.

Exit criteria:

- An interrupted sync recovers without `--force`.
- `doctor` fails when any agent's guard hook does not block.
- Upgrading from v0.6.0 to v0.7.0 is tested in CI.
- The Windows decision is written down.

### v0.8.0 "Launch-ready" (target 2026-12-19)

Goal: a visitor understands dashcrew in 30 seconds, tries it in one command, and finds the agent they already use.

- **The guard is the story**
  - [ ] Test the pitch first. In Claude Code and Codex, set native deny rules for `git push` and try `bash -c "git push"`, `sh -c` and `env git push`. If the native rules miss them and dashcrew blocks them, that is the GIF. If not, the pitch is "one policy for every agent".
  - [ ] `dashcrew guard init`: install the guard hooks and a default git-flow policy without a `project.yaml`, as the 10-second way to try dashcrew. Decide project-level or user-level hooks first (see the open questions in `docs/ideas/launch-direction.md`).
  - [ ] The guard section opens with what it blocks and what it does not, and lists the known misses: protection against accidents, not a sandbox.
- **Cursor**
  - [ ] Adapter for Cursor: rules plus the guard hook, golden-tested, covered by the doctor probe. Spike the hook first to confirm it can fail closed.
  - [ ] Keep the agent list in one place (today it is defined three times: `adapters/adapter.go:40`, `profile.go:56`, `me/profile.go:25`).
  - [ ] `docs/adding-an-adapter.md`, written from the Cursor work, so Gemini CLI can be the first outside contribution.
- **Front page**
  - [ ] README under 150 lines:
    - a one-sentence hero;
    - the guard GIF;
    - three "why" bullets with the team layer (lock and drift check in CI, team and personal profiles, one guard policy);
    - a quickstart;
    - a comparison table against rulesync, Ruler and standalone guard hooks.
  - [ ] Move reference material to `docs/` (profile reference, release lines, knowledge, specs, troubleshooting, migration notes), and add `docs/architecture.md`.
  - [ ] Remove stale lines (`README.md:9`, `:47`, SECURITY.md "before the first tagged release"); drop internal plan codes (P3, P4, P10b) from the README and CHANGELOG.
  - [ ] Repository description ("One source of rules, skills and guard hooks for every AI coding agent. One Go binary, any stack."), topics, homepage, a 1280×640 social preview, badges, Discussions on, and 5+ `good first issue` items.
- **Agent commit and push** (plan: `docs/plans/agent-push-plan.md`)
  - [x] `policy.agent_push`: at the end of a task the agent asks whether to commit and push the task branch; the guard still blocks force, delete, `--all`/`--mirror`/`--tags`, base branches and a push without a named branch.
- **Install and `init`**
  - [ ] An `install.sh` that downloads the release archive and checks `checksums.txt`; install smoke tests in CI (brew, `go install`, script).
  - [ ] `init` without dead ends:
    - when `go.mod` is found, offer the Go platform directly, and promote Go from starter to a built-in platform;
    - under `platform: other`, propose detected files as defaults;
    - print detected stacks before the prompts;
    - a fresh project passes `doctor` with zero warnings.
  - [ ] Fix messages: the `/..` path in "No .ai/project.yaml in …", `ok mcp defined: ` with an empty value, odd command text such as `ruff check . <changed paths>`, and the status table's 16-column ID field that `antigravity-hook` fills, so it runs into its detail (`cmd/dashcrew/main.go:339`).
- **Windows**, if the v0.7 investigation found a small fix: ship it here. Otherwise it moves to v0.9.
- **Soft launch** in December to the office team and a few outside developers. Their friction notes are fixed or filed before the launch.

Exit criteria:

- Three people outside the project install and run dashcrew on Flutter, Go and one other stack without help.
- The Cursor guard probe passes.
- `guard init` works in a repo with no dashcrew setup.

### Launch (Tuesday 2027-01-12)

Not a release. The tag is whichever v0.8.x is current, or v0.9.0 if it is ready; see the open question on calling it `v1.0.0-rc.1`. Launching after the holidays avoids the late-December traffic dip.

- [ ] Show HN: "Show HN: dashcrew – one guard and one set of rules for Claude Code, Codex and Cursor".
- [ ] Posts on r/ClaudeAI, r/ChatGPTCoding, r/cursor, r/golang and r/FlutterDev; an X/Bluesky thread with the GIF.
- [ ] Someone besides the maintainer is available to answer issues and comments for the first 24 hours.
- [ ] For the following two weeks, track release downloads, new issues and Discussions from new people, not only stars.

### v0.9.0 "Reach" (target ~2027-02)

Goal: answer what launch users asked for, and reach the next agents.

- [ ] Gemini CLI adapter, ideally an outside contribution against `docs/adding-an-adapter.md`; then GitHub Copilot.
- [ ] Windows support, if it did not fit in v0.8:
  - hooks that fail closed without a POSIX shell;
  - transcripts running on Windows;
  - a copy fallback for worktree links (`worktree.go:356`);
  - CRLF handled when hashing generated files.
- [ ] The top requests from launch issues and Discussions, triaged against the "Not Doing" list in `docs/ideas/launch-direction.md`. Candidates, built only if asked:
  - import from rulesync, Ruler, `.cursor/rules` or `GEMINI.md`;
  - an opt-in safety pack for destructive and secret-reading commands.
- [ ] Let small outside PRs skip the plan step in AGENTS.md.

Exit criteria:

- 5+ agents generated and golden-tested.
- At least one merged outside contribution.
- Windows either green in CI without skips in the guard path, or explicitly out of 1.0.

### v1.0.0 "Freeze" (rc.1 ~2027-03, final two weeks later)

Goal: the public surface is something we are willing to keep for years, shaped by the users the launch brought.

- **Schema discipline**
  - [ ] One version key name across `project.yaml`, `lock.json`, `guard.json` and `me.yaml` (today: `toolkit`, `schema`, `schema`, none).
  - [ ] Check the lock's `schema` on read in `sync`, and the guard policy's `schema` in the guard, not only in `doctor`. The lock already records `dashcrew_version`; a schema number lets it replace the P3/P4 heuristic (`sync.go:52-63`).
  - [ ] Write the compatibility promise above into `docs/compatibility.md`, and apply it to `guard_min_version` bumps.
- **CLI cleanup.** Each item keeps the old spelling as a deprecated alias with a warning, removed in 2.0.
  - [ ] `--pub` on `worktree create` becomes `--deps`, matching `link`.
  - [ ] `--agents` behaves the same on `init` and `platform new`.
  - [ ] `usage --agent` takes the same agent IDs as the rest of the CLI (`claude-code`, not `claude`).
  - [ ] Settle what `-p` means (project root vs any folder) and document it per command.
  - [ ] Decide whether `verify record` and the `check --ports/--links` mode flags stay as they are.
- **Schema 2.** Rename Flutter-specific keys in the generic schema (`language.dartdoc`, `policy.dartdoc_max_lines`). Ship a `dashcrew migrate` that rewrites a v1 project and is tested against every tagged golden, including the frozen P3 fixture.
- **One marker format** for generated files, defined in one place (today there are three styles, and the prefix is retyped at `sync.go:512,716`).
- **Internal clean-up.** Behavior-neutral, done under the existing goldens.
  - [ ] Move the argument parser out of `cmd/dashcrew/cli.go` into `internal/cli`.
  - [ ] Move profile loading out of `sync` (used from 8 call sites).
  - [ ] Split `doctor` (1263 lines) by check family.
  - [ ] One git exec helper instead of about 10.
  - [ ] One home-directory resolver instead of five.
  - [ ] Raise `cmd/dashcrew` coverage from 40% to 70%+.
- **Release candidates.** At least two weeks on `-rc` tags. No open bug labeled `stability`, and no golden change between the last rc and final other than the version string.
- **Announcement.** A "dashcrew 1.0" post to the same channels, led by what launch users changed.

Exit criteria:

- The compatibility promise holds across rc.1 to 1.0.0 in CI: an old binary on a new project, and a new binary on an old project.
- README, docs and `--help` agree.

## After 1.0

These ideas stay in `docs/ideas/` and `docs/plans/` until users ask for them:

- Path-scoped Claude rules (P13d).
- The bridge (P8).
- Private package servers for multi-repo (P9).
- GitHub gates across repos.
- Knowledge tracker across repos.
- A plugin or registry for community packs, possibly through the skills.sh ecosystem.

## Measuring progress

| Signal | v0.7 | v0.8 | Launch +2 weeks | v0.9 | v1.0 |
|---|---|---|---|---|---|
| Hotfix releases in the milestone | 0 | 0 | ≤1 | ≤1 | 0 |
| Fresh `init` + `sync` + `doctor` warnings | - | 0 | 0 | 0 | 0 |
| Agents supported | 3 | 4 | 4 | 5+ | 5+ |
| OS with green guard tests | 2 | 2-3 | 2-3 | 3 | 3 |
| New people opening issues or Discussions | - | 3+ (soft launch) | 20+ | - | - |
| Outside contributors | 0 | 0 | 1+ | 3+ | 5+ |

Trending is a result of these, not a target we can schedule: a daily Trending spot depends on stars per day against that day's competition. The levers we control are the 30-second README, a GIF that shows the guard working, one-line install, support for the agent the visitor already uses, and one well-prepared launch.
