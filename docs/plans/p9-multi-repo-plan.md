# P9 plan: multi-repo work, the generic parts

Tier: large (a new profile key, a new `link` command, a guard change, new `check` and `doctor` checks, catalog text gated on the new key). Idea: `docs/ideas/knowledge-tracker-multirepo.md` (P9) and `docs/ideas/github-gates-multi-repo.md` (multi-repo tasks). P8 is deferred; this plan does not depend on it.

## Goal

A task often touches more than one repo: a shared package and the apps that use it, or an app and its backend. dashcrew should make that safe without deciding how the team lays out its repos, and without deciding how an app consumes a dependency. Three things are needed whatever the layout:

1. **Link:** point a repo at a local checkout of another repo while working, without committing that.
2. **Guard:** the right policy applies to every file the agent touches, also in a repo it reached through a link, and also when the agent runs from a folder above several repos.
3. **Merge order:** CI refuses a PR whose committed dependency still points at work that is not merged.

Multi-repo tracks (`task new/ls/done`, a workspace layout) are not in this plan. They come later only if the generic parts prove too thin, and can then be an adapter to another tool instead (see P8).

## Dynamic by design

- **Layout:** no fixed folder structure. Repos may be side by side, in `<repo>.worktrees/`, or in another tool's track folders. Every link names its target path; nothing is derived from a layout.
- **Platform:** the platform's `link` field already names the override file (`pubspec_overrides.yaml` for Flutter). P9 adds `go.work` for a Go platform. A platform without `link` (or `platform: other` without one in `platform_config`) gets the guard and check parts, and `dashcrew link` says linking is not available for it.
- **Dependency kind:** each linked dependency is one of `git` (committed as a git URL with a ref), `hosted` (a version from a package server, public or private) or `path`. dashcrew reads the kind from the committed manifest; the profile does not have to say it.

## What exists

- `platforms/flutter/platform.yaml` has `link: pubspec_overrides.yaml`; the catalog reads it (`internal/catalog`), nothing uses it yet.
- `dashcrew worktree create` adds worktrees at `<repo>.worktrees/<name>`, records `dashcrew-base`, writes the git exclude file.
- Guard policy is `<hook cwd>/.ai/guard.json`; paths are resolved on the real path; with no policy at cwd every call is allowed.
- `dashcrew check` runs on PRs (sections, commits, ports) and is a required check in the generated workflow.

## Stages

### M0: profile key `project.links`

```yaml
project:
  links:
    - package: shared_core            # the name in pubspec.yaml or the module path in go.mod
      repo: git@github.com:acme/shared_core.git   # used by check to read refs; optional for hosted
```

- Strict keys: `package` required, `repo` optional.
- `has_links` gates every catalog line P9 adds, so teams without links pay zero AGENTS.md bytes.
- Validation: a package listed twice is an error; `repo` must be an ssh or https git URL.
- `platform_config` (level 1 platforms) accepts `link: pubspec_overrides.yaml | go.work`, so a project without a built-in platform can link too. Go has no built-in platform until P10; its `go.work` writer lives in core and is chosen by the link file name.

### M1: `dashcrew link`

- `dashcrew link add <package> <path>`: writes the platform's override file in the current repo, pointing `<package>` at `<path>`:
  - Flutter: `dependency_overrides: { <package>: { path: <path> } }` in `pubspec_overrides.yaml`;
  - Go: a `go.work` with `use .` and `use <path>`.
- Checks before writing:
  - `<package>` is listed in `project.links`;
  - `<path>` holds that package (its `pubspec.yaml` name or `go.mod` module matches);
  - the override file does not already exist with content dashcrew did not write; then it refuses unless `--force`.
- The override file goes into the worktree's git exclude file, the same way `worktree create` excludes linked toolkit files. It is never committed.
- Runs the platform `deps` command afterwards, as `worktree create` does.
- `dashcrew link ls`: each link with its target, the target's branch, and whether that branch matches this one.
- `dashcrew link remove <package>`: removes the entry, and the file when it becomes empty.
- `doctor`:
  - warns when an override file exists that is not excluded from git;
  - warns when a link target is missing;
  - notes (not a warning) when a linked repo is on a different branch name.

### M2: guard, policy per path

- **File tools:** the policy for a path is the `.ai/guard.json` of the nearest dashcrew project that contains the real path. That covers:
  - an agent in the app editing the linked package: the package's own policy applies to its files;
  - an agent started in a folder above several repos: each file gets its repo's policy.
- **Shell commands:**
  - with a policy at cwd, unchanged;
  - with no policy at cwd: the command rules (blocked git subcommands, format, publish) of every policy one level below cwd, combined; the strictest wins. Path rules are not combined, because a path rule of one repo means nothing in another.
- A path outside every dashcrew project keeps today's behaviour.
- The policy lookup is a walk up from the path, cached per call; the guard stays a single process run.
- Differential: the current corpus gives identical outcomes when every path is inside the cwd project; new corpus cases for linked and multi-repo paths.

### M3: `check` for dependency state

- New check `links`, on for profiles with `project.links`. For each listed package, it reads the committed manifest (`pubspec.yaml` and `pubspec.lock`, or `go.mod`):

  | Committed as | Refused when |
  |---|---|
  | `git` with a ref | the ref is a branch name, or a commit not reachable from the dependency repo's default branch. A tag passes. |
  | `hosted` | never by dashcrew: `pub get` in CI already fails on an unpublished version |
  | `path` | the path leaves the repo |
  | Go `replace` to a local path, or a committed `go.work` | always |
  | any | a committed override file (`pubspec_overrides.yaml`) |

- Reading refs uses `git ls-remote` with the credentials CI already has for `pub get` or `go mod download`. When the repo cannot be reached the check fails and says which credential is missing.
- The generated workflow needs no new step.
- `check --links` runs the same check locally before a push.

### M4: catalog text

- Branching skill, gated on `has_links`: a task that spans repos uses the same branch name in each, created with `dashcrew worktree create` in each repo, and linked with `dashcrew link add`.
- PR description section `Dependencies` (gated): the linked PRs or tags this PR needs, merged first.
- The commit command step reminds to release or merge the dependency first when `check --links` fails.
- Byte cost goes to skills and the PR template. AGENTS.md gets at most one gated line. The budget test proves AGENTS.md without links is unchanged.

## How we check

- M0: profile tests, including duplicates and bad URLs.
- M1: link add/ls/remove on Flutter and Go fixtures; refusal on a wrong package and on a foreign override file; exclude written; transcript tests.
- M2: guard corpus unchanged; new cases:
  - edit in a linked repo under its policy;
  - a folder above two repos (file and shell);
  - path outside every project.
- M3: fixtures for each row of the table, with a local bare repo standing in for the remote; unreachable remote fails with the message.
- M4: goldens for a profile with links; AGENTS.md budget test.
- Live: one real cross-repo task on two sample repos (a package and an app) with a git ref, then the same with a hosted version, if a private server is available.

## Not doing

- `task new/ls/done` and a workspace layout (tracks).
- Creating or merging PRs across repos, or publishing to a package server.
- Resolving versions for `hosted` dependencies; `pub get` and `go mod download` already do that.
- Linking for platforms without a `link` field.

## Decisions (confirmed)

1. When `check --links` cannot reach the dependency repo, the check fails and names the missing credential.
2. Accepted git refs: a tag, or a commit reachable from the dependency repo's default branch. A branch name is refused.
3. Linked repos on different branch names: a doctor note, not a warning.
4. Scope: the generic parts only (link, guard per path, check); tracks later if needed.
5. Guard: policy per path; command rules combined only when no policy is at cwd.

## Progress

- [x] M0 profile key: `project.links` (`internal/profile/links.go`), `platform_config.link`, `has_links`.
- [x] M1 link command: `internal/link`, `dashcrew link add/ls/remove`, doctor `links` (machine check; only with links or a link file).
- [x] M2 guard per path: `internal/guard/projects.go`; corpus outcomes unchanged; new cases in `projects_test.go`.
- [x] M3 check links: `internal/deps`, `check` adds `links`, `check --links`.
- [x] M4 catalog text: fragments gated with `when: { has_links: true }` in branching, pr-description, verify and the PR template; AGENTS.md unchanged (test); golden `sample-links`.
- [x] Live (2 Nov, local bare remote, dev build 0.2.0-dev): link add/ls/remove, doctor info, check --links on tag, branch, commit on main, commit only on a branch and an unknown ref, guard per path through the shim (9 cases). Found: `pub get` under a link rewrites `pubspec.lock` to a path, which check passed; fixed (check refuses it, link add notes it, branching skill unlinks before the commit command).
- [ ] Hosted (private package server) and a real private GitHub dependency, when one exists.
