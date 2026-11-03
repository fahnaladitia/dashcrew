# dashcrew

Agent-agnostic AI coding toolkit. Flutter is the built-in platform; any other stack works through `platform: other`.

dashcrew keeps one neutral source of rules, skills, workflows and guards, and
renders it for the AI coding agents your team uses: Claude Code, Codex and
Antigravity. Your platform and stack are chosen once, in `.ai/project.yaml`.

> Status: pre-release, no tagged release yet. Claude Code output has been run
> end to end. Codex and Antigravity output is generated and checked against
> their documentation (and, for Codex, its source), but not yet against a live
> session.

Not affiliated with or endorsed by Google, the Flutter team, Anthropic or OpenAI.

## How it works

```
core/ + packs/ + platforms/ (shipped)  --+
.ai/packs/, .ai/platforms/ (yours)     --+-->  dashcrew sync  -->  AGENTS.md         read by every agent
.ai/project.yaml                       --+                         per-agent files   skills, subagents, hooks, MCP
                                                                   .ai/guard.json    one policy for every agent
                                                                   .ai/bin/guard     committed shim -> dashcrew guard
                                                                   .ai/lock.json     hashes, versions, owned entries
```

- **Rules** go into one `AGENTS.md`. Your own rules live in its project block, which `sync` never rewrites.
- **One agent plans and builds.** The main session runs the chain: step 0, then `plan`, `tdd`, `code-review` and `verify` as the task's size tier needs. Skills, not personas, carry the expertise.
- **Skills and workflows** (`plan`, `tdd`, `code-review`, `verify`, `learn`, `audit`, `worktree`, `build-fix`, and `port` with [release lines](#release-lines)) become each agent's skill format. AGENTS.md carries a generated skill index; see [Choosing skills](#choosing-skills).
- **Two subagents** remain, both read-only: `reviewer` (independent review of a large plan and of the diff) and `explorer` (maps many files to `file:line` for `plan` and `audit`).
- **Platforms** hold everything that belongs to one stack: commands, lifecycle rules, doc comments, pack families. See [Platforms](#platforms).
- **The guard** is `dashcrew guard`, a subcommand of the same binary, that every agent's pre-tool hook reaches through the committed shim `.ai/bin/guard`. It reads the policy in `.ai/guard.json` and blocks the commands and writes listed in [Guard](#guard).
- `doctor` checks the result against the lock without writing anything.

## Install

One static binary per OS, with the catalog built in. Requires git and a POSIX shell (the guard shim is `sh`).

```bash
brew install fahnaladitia/tap/dashcrew
# or, with Go 1.26 or later:
go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@latest
```

`dashcrew --version` prints the version. A team pins one: `brew` installs the latest release, and `go install ...@v0.1.0` (or a commit) a fixed one. The guard shim needs dashcrew 0.1.0 or newer on `PATH`.

Both routes need a tagged release. Until `v0.1.0` exists, a build from `main` reports a `v0.0.0-...` pre-release version, and the guard shim refuses it, so agents cannot run tool calls in a synced project.

Before 0.1.0, dashcrew was a Dart package installed with `dart pub global activate`. That install has no `dashcrew guard`, so the shim refuses every tool call while it is first on `PATH`, and `doctor` warns about it (see [Troubleshooting](#troubleshooting)).

## Quick start

In the project root:

```bash
dashcrew init      # detects the platform, reads remote branches (flutter: also pubspec.yaml, lib/main_<flavor>.dart)
dashcrew sync      # writes the toolkit
dashcrew doctor    # checks it; exit 1 on any failure
```

`init` prints what it detected with the evidence, then asks for the platform: the one whose `detect` files it found, `other` when none matched, or your pick when several did. With `other` it asks for the detect files and each command. It then asks about every stack family of that platform, proposing the detected option: `state [bloc]: ` after finding `flutter_bloc` in `pubspec.yaml`, `design [-]: ` when nothing was found (only `flutter` detects options so far). Enter keeps the proposal, `-` drops the family, and `other <package>` names a library dashcrew has no pack for. `--yes` writes what it detected without asking, and stops when no platform or several matched.

## The profile: `.ai/project.yaml`

```yaml
toolkit: 1                                # required
platform: flutter                         # flutter (default), a project platform, or other
agents: [claude-code, codex, antigravity] # required: every agent your team uses
language: { dartdoc: en, pr: en, chat: en }
stack:
  state: { use: riverpod, codegen: true } # a pack, with params
  di: riverpod
  router: go_router
  network: { use: other, name: graphql_flutter }   # no pack: generic rules for that library
project:
  flavors: { dev: lib/main_dev.dart, prod: lib/main_prod.dart }
  local_packages: [packages/core]
  base_branches: [main, develop]
  branch_prefix: { feat: feat/, fix: fix/, hotfix: hotfix/ }
  build_wrapper: tool/build.sh
policy:
  agent_commit: false
  formatter: false
  dartdoc_max_lines: 3
  tier_gates: true
  terse_prompts: false
paths: { plans: .ai/plans, audits: .ai/audits, pr: .ai/pr, memory: .ai/memory }
ci: { github: true, dashcrew_ref: v0.1.0 }
```

| Key | Default | Effect |
|---|---|---|
| `platform` | `flutter` | `flutter`, a project platform in `.ai/platforms/<name>/`, or `other` with `platform_config`; see [Platforms](#platforms) |
| `agents` | none, required | `claude-code`, `codex`, `antigravity`; files are written for each |
| `language.dartdoc`, `.pr`, `.chat` | `en` | language of Dartdoc, PR descriptions and chat (ISO 639-1) |
| `stack.<family>` | not set | an option name, or `{use: <option>, <param>: ...}`; see [Stack options](#stack-options) |
| `project.flavors` | none | flavor name to entrypoint; rendered as `flutter run --flavor <name> -t <entrypoint>` |
| `project.local_packages` | none | path packages; the app's `lib/` rules and the vendor import check skip them |
| `project.base_branches` | `[main]` | proposed bases for new branches |
| `project.branch_prefix` | `feat/`, `fix/`, `hotfix/` | branch prefixes per change type |
| `project.build_wrapper` | none | script that injects `--dart-define`s for real builds |
| `project.commit.pattern` | `<type>(<scope>): <subject>` | commit message pattern; placeholders `type`, `scope`, `subject` (required), `ticket`. An empty scope or ticket is dropped with its brackets |
| `project.commit.types` | `feat, fix, refactor, test, docs, chore, perf, build, ci` | allowed types |
| `project.commit.type_from_branch` | `{feat: feat, fix: fix, hotfix: fix}` | `branch_prefix` key to type |
| `project.commit.scopes` | none | path glob to scope, first match wins (`lib/features/cart/**: cart`); the scope with the most changed files is picked |
| `project.commit.ticket` | none | `{from_branch: <regex>, trailer: <name>}`: ticket id from the branch name (first group, else the match); `trailer` defaults to `Refs` unless the pattern has `<ticket>` |
| `project.commit.subject_max` | `72` | longest subject line, in characters |
| `project.commit.language` | `en` | language of commit subjects and bodies |
| `project.commit.body` | `optional` | `optional`, `required` or `none` |
| `project.commit.trailers` | none | trailers allowed on top of `Co-authored-by`, `Signed-off-by`, `Reviewed-by` and the ticket trailer |
| `project.commit.allow_email_trailers` | none | the only trailers whose value may hold an email address (DCO `Signed-off-by`); an email anywhere else fails lint |
| `project.release_lines` | not set | `{main: <branch>, release: <regex>, hotfix: <regex>, check_ports: <bool>}`: the main line and the patterns of release and hotfix lines; see [Release lines](#release-lines) |
| `project.links` | not set | `[{package: <name>, repo: <git url>}]`: dependencies a task may point at a local checkout with `dashcrew link` (the pubspec name or Go module path; `repo` optional, ssh or https). `has_links` gates the multi-repo text |
| `policy.agent_commit` | `false` | `true` lets the agent commit; push stays blocked |
| `policy.formatter` | `false` | `true` allows `dart format` on changed files only |
| `policy.dartdoc_max_lines` | `3` | hard cap per documented unit |
| `policy.tier_gates` | `true` | trivial and small tasks with a GO plan continue without waiting |
| `policy.terse_prompts` | `false` | agents answer short and blunt |
| `paths.*` | `.ai/plans`, `.ai/audits`, `.ai/pr`, `.ai/memory` | where plans, audits, PR descriptions and project memory live |
| `ci.github` | `false` | write `.github/workflows/dashcrew.yml`, which runs `dashcrew check` on every pull request; see [Merge gate on GitHub](#merge-gate-on-github) |
| `ci.dashcrew_ref` | none | dashcrew release tag that workflow installs, `v0.1.0` or newer; required with `ci.github`. A commit or branch is refused |
| `knowledge` | not set | `{repo: <git url or path>, name: <folder>}`: the team's shared knowledge repo; see [Shared knowledge](#shared-knowledge) |

## Platforms

The profile's `platform` picks one. Every stack works at the first level; the others add depth.

| Level | How | What the team gets |
|---|---|---|
| `platform: other` | declare it inline: `platform_config: { detect: [<file>], commands: { analyze, test, test_scoped, deps, format }, link }` (`format` and `link` optional; `link` is `pubspec_overrides.yaml` or `go.work`) | all of core: the chain, skills, guard, `reviewer` and `explorer`, PR template and `dashcrew check`; core renders your commands, with no stack-specific rules |
| Project platform | write `.ai/platforms/<name>/` in your repo, in the format of `platforms/` (`platform.yaml`, rules, skills, workflows, packs), and set `platform: <name>` | depth for that stack, your conventions, no dashcrew release needed |
| Built-in | `platforms/<name>/` in this repo; today `flutter` | shared with every team |

```yaml
platform: other
platform_config:
  detect: [go.mod]
  commands: { analyze: go vet, test: go test ./..., test_scoped: go test, deps: go mod download, format: gofmt -l }
```

**Starters.** `dashcrew platform new go [--http nethttp|chi|gin|echo|fiber|none] [--db pgx|sqlc|gorm|none]` writes a project platform for a Go service into `.ai/platforms/go/`: Go rules (context, error wrapping, goroutine ownership, `gofmt -l` as a check), backend architecture, security and testing rules in place of the app-oriented core ones, a `backend-security` review skill, the `data-layer-tdd` and `feature-architecture` skills for services, packs for the HTTP framework and the database layer, `link: go.work`, and the backend `guard:` rules. The packs default to what `go.mod` (and a `sqlc.yaml`) names. Without `.ai/project.yaml` it writes one; with one, it prints the keys to set. The folder is the team's from then on: dashcrew never overwrites it, so rename the layers and edit the rules to match the code. `init` points at the starter when it finds a `go.mod` and no matching platform.

A project platform with a built-in name replaces it whole. The format is in [docs/catalog-format.md](docs/catalog-format.md#platforms).

## Stack options

The `flutter` platform ships these families; `tracker` and `design` are shared by every platform.

| Family | Options | Shipped with dashcrew |
|---|---|---|
| `state` | `bloc`, `riverpod`, `provider`, `none` | rules, a patterns skill, test guidance |
| `di` | `get_it` (param `injectable`), `riverpod`, `provider`, `none` | registration rules |
| `router` | `go_router`, `auto_route`, `getx`, `none` | navigation rules |
| `network` | `dio`, `http`, `retrofit`, `none` | client placement, error mapping, test client |
| `storage` | `hive`, `shared_preferences`, `sqflite`, `none` | access through repositories, migrations |
| `test` | `manual-fakes`, `mocktail`, `mockito`, `none` | mocking conventions |
| `analytics` | `posthog`, `firebase`, `mixpanel`, `none` | ask-before-tracking, facade-only imports |
| `tracker` | `clickup`, `github` (param `mode`: `project` or `labels`), `none` | task status flow: start when the branch exists, review with the PR link when the PR exists, only for work that started from a task |
| `qa` | `semantics-identifier` | QA identifier rules |
| `design` | `figma`, `none` | design-to-code skill (`figma-design`) |

Every family also takes `{use: other, name: <package>}` for a library without a pack.
Pack params and their defaults are in `pack.yaml` of each pack (`platforms/flutter/packs/<family>/<option>/` or `packs/<family>/<option>/`); set them in the map form (`state: {use: bloc, shared_dir: lib/shared/blocs/}`).

**GitHub tracker.** `tracker: {use: github, project_owner: <owner>, project_number: <n>}` keeps status in the Project's single-select `Status` field (`In Progress`, `In Review`; params `status_field`, `status_start`, `status_review`). A new GitHub Project's built-in `Status` has Todo, In Progress and Done: add an `In Review` option to it rather than a second field, since GitHub's workflows set `Status` when an item is added and a board grouped by `Status` would not show a second field's value; `mode: labels` uses issue labels instead (`label_start`, `label_review`). The PR body links the issue with `pr_link` (`Refs` by default, so merging never closes the issue unasked; set `Closes` to close it). Projects need gh 2.31 or newer and the `project` scope (`gh auth refresh -s project`); `doctor` warns with `tracker` when the `gh` on PATH has no `gh project`. The agent never creates a Project, field, option or label; a missing one stops the flow with a question. With any tracker, the `branching` and `pr-create` skills carry the flow's two steps.

**Project packs.** Put your own packs in `.ai/packs/<family>/<option>/`, in the same format. One with a shipped name replaces it; a new family (`architecture/clean`) becomes a valid `stack` key for that project. See [docs/catalog-format.md](docs/catalog-format.md).

## Choosing skills

Skills are picked once per task, at step 0, in one visible line. The agent prints this block before any work and you confirm or edit it once:

```text
TASK:             <feat | fix | hotfix>: <polished request>
Tier:             trivial | small | standard | large   (<evidence>)
Assumptions:      <each one, or none>
Questions:        <all of them, once, or none>
Skills:           <name> (<why>), <name> (<why>)
Success criteria: <id> <criterion> -> unit | widget | integration | manual
Branch:           1) <name>  2) <name>  3) <name>   base: <ask>
```

That is the flutter rendering; other platforms say `ui` where it says `widget`.

- **The index.** AGENTS.md has a generated `### Skill index`: one line per installed skill with its `when_to_use`, sorted by name. The agent writes the `Skills:` line from it. Nobody keeps that table by hand: a skill in a pack or project platform appears once it renders, and `doctor` checks that every indexed skill is installed.
- **Review checklists.** A skill whose `meta.yaml` sets `review: true` is marked `(review)` in the index. When it is in the `Skills:` line, the `reviewer` applies its checklist to the diff. Shipped ones: `design-fidelity`, plus `mobile-security` and `qa-identifiers` (with `qa: semantics-identifier`) on flutter.
- **Your own.** Add a skill to a project pack (`skills/<name>.yaml` plus a `target: skill/<name>` fragment) or a project platform (`skills/<name>/meta.yaml`), with a `when_to_use` that says when to pick it.

## Commands

### `dashcrew init`

| Flag | Default | |
|---|---|---|
| `-p, --project` | `.` | project root |
| `-y, --yes` | off | write the detected profile without asking |
| `--agents` | `claude-code` | agents to list in the profile (repeatable) |
| `--force` | off | replace an existing `.ai/project.yaml` |

Writes only `.ai/project.yaml`.

### `dashcrew sync`

| Flag | Default | |
|---|---|---|
| `-p, --project` | `.` | project root holding `.ai/project.yaml` |
| `--force` | off | overwrite generated files edited by hand, or a lock written by a newer dashcrew |
| `--adopt` | off | keep an existing, hand-written AGENTS.md as the project block |
| `--[no-]compile-guard` | off | deprecated, does nothing; prints a notice. There is no per-machine guard to install any more |

Running it twice writes nothing the second time. Files it stops generating are deleted.

### `dashcrew guard [<policy>]`

The guard hook: reads one tool call as hook JSON on stdin and answers per agent (see [Guard](#guard)). The policy is `<policy>`, else `.ai/guard.json` in the hook's working directory. Hooks call it through `.ai/bin/guard`; you do not run it yourself. `dashcrew guard install` was removed with the per-machine compiled guard.

### `dashcrew doctor`

`-p, --project` (default `.`). Writes nothing; exit 1 when any check fails.

| Group | Checks |
|---|---|
| Profile and lock | `profile`, `project-packs`, `lock`, `version`, `drift` (generated files edited by hand), `stale` (output differs from the current profile and catalog) |
| Guard | `guard-policy`, `guard-binary` (the shim; the `dashcrew` first on `PATH`, which fails when missing and warns when it is an old Dart-installed one or older than the lock's `guard_min_version`; then a `git push` event through the shim), `guard-log` (only when the guard logged malformed payloads since the last sync) |
| Claude Code | `hook`, `claude-md`, `mcp`, `skills` |
| Codex | `codex-hook`, `codex-mcp`, `codex-skills-budget`, `codex-trust` |
| Antigravity | `antigravity-hook`, `antigravity-mcp`, `antigravity-roles` |
| Knowledge | `knowledge` (the link and the clone's last pull), `knowledge-stale`, `spec` (only when the branch records a spec; warns, never fails) |
| Project | `agents-md-size`, `gitignore` (warns while `.ai/bin/` is ignored), `handoff-rosters` (only when found) |
| Personal | `me` (your `~/.dashcrew/me.yaml` and the personal files `dashcrew me sync` writes; warns, never fails) |

Agent checks run only for the agents in the profile. Each line starts with `ok`, `warn`, `fail`, `skip` or `info`; only `fail` sets exit 1. `info` reports something that needs no fix, such as `handoff-rosters`: `<plans>/*/handoff/00-roster.md` files left by the pre-P4 team flow.

### `dashcrew check`

| Flag | Default | |
|---|---|---|
| `-p, --project` | `.` | project root |
| `--pr-body <file>` | none | the PR description to check; in a pull request workflow the event supplies it |
| `--base <branch>` | none | lint the commits of `<base>..HEAD`; in a pull request workflow the event's base and head supply the range |
| `--ports` | off | only the port check of [release lines](#release-lines); runs alone, without `--pr-body` or `--base` |
| `--from <line>`, `--to <line>` | every adjacent pair | with `--ports`: the older and the newer line, each a version, a line branch or `main` |

Read-only; exit 1 on any failure, with `::error` annotations in GitHub Actions. It runs the toolkit checks of `doctor` that need no agent or guard binary, then `pr-sections` (every heading of `.github/pull_request_template.md` in the PR body) and `commits` (`dashcrew commit lint` on every commit of the PR but merges; skipped without a base). With `release_lines.check_ports: true` it adds `ports`, the check of `--ports` over every adjacent pair of lines. With `project.links` it adds `links`: each listed dependency as committed in `pubspec.yaml` and `pubspec.lock`, or `go.mod`. A git ref must be a tag, or a commit on the dependency's default branch (a branch name is refused, even the default one); a `path` must stay inside the repository; a Go `replace` to a local folder and a pseudo-version off the default branch are refused; hosted versions pass, since `pub get` and `go mod download` fail on an unpublished one; a committed link file fails. Refs are read with `git ls-remote` and a commits-only fetch, using the credentials CI already has for the dependency; an unreachable repository fails the check. `dashcrew check --links` runs only this, one line per dependency, before a push.

### `dashcrew commit`

Commit messages in the pattern of `project.commit`. Neither subcommand commits; both take `-p, --project`, any folder inside the repository.

| Subcommand | |
|---|---|
| `suggest [--subject <s>] [--body <b>] [--type <t>] [--scope <s>] [--ported-from <sha>[,<sha>...]] [--json]` | prints one line, `git add -- <files> && git commit -m ...`, with the changed tracked files listed one by one and every value quoted for bash and zsh. Type, scope and ticket come from the branch and the files; untracked files are listed apart, never added. With a verify record it says which files changed after the last verify. `--ported-from` adds one `Ported-from:` trailer per commit, after the ticket trailer. With `--subject` the message is linted; a problem exits 1. Never shows an email address from the branch or the git config |
| `lint <message> \| - \| --range <base>..<head>` | checks the pattern, type, subject length, body rule, trailers, email addresses (`name@host.tld`, also inside `<...>`; `@mention` and image suffixes such as `icon@2x.png` are not addresses) and AI attribution (generated-with footers, AI co-author trailers, robot lines). One line per problem, exit 1 |

### `dashcrew release`

Needs `project.release_lines`; both subcommands take `-p, --project`.

| Subcommand | |
|---|---|
| `lines` | the release and hotfix lines oldest first, then the main line, with their head commits, and how the current branch resolves |
| `status <version>` | regenerates `<plans>/<version>/README.md`, the release ledger, and prints it. Writes only that file and runs no git write command |

### `dashcrew verify record`

`-p, --project`. Records the tree hash of the working copy, untracked files included, in `<git dir>/dashcrew/last-verify` (per worktree). The hash is built in a temporary index (`GIT_INDEX_FILE`), so the real index and staging are never touched. The `verify` workflow runs it after a PASS.

### `dashcrew worktree`

One worktree per parallel task, at `<repo>.worktrees/<name>`. It never commits, pushes or deletes a branch.

| Subcommand | |
|---|---|
| `create <branch> <base> [--name <dir>] [--no-pub]` | new branch with no upstream; links the git-ignored toolkit files from the main checkout; records the base; runs the platform's `deps` command when one of its `detect` files is there (`flutter pub get`, through `fvm` when `.fvmrc` exists) |
| `list` | branch, base, upstream, dirty lines, missing links |
| `link <name>` | restores missing links and drops an upstream the branch should not have |
| `remove <name>` | removes a clean worktree; the branch stays |
| `watch [<name>...] [--once] [--state-dir <dir>] [--interval <s>]` | one line per new event: `commit`, `pushed`, `pr` (through `gh`), `base-moved`, `merge-tree`, `overlap`, `done` |

Every subcommand takes `-p, --project`, any folder inside the repository.

### `dashcrew link`

Points a dependency listed in `project.links` at a local checkout while a task spans repos, through the platform's link file: `pubspec_overrides.yaml` for Flutter, `go.work` for a Go project (`platform_config.link`). The file is added to the checkout's git exclude file and is never committed; `doctor` warns when git would commit it, and fails when it is committed.

| Subcommand | |
|---|---|
| `add <package> <path> [--force] [--no-deps]` | checks that `<path>` holds that package (pubspec `name` or `go.mod` module), writes the entry, excludes the file and runs the platform's `deps` command. A link file dashcrew did not write is refused without `--force`, and a committed one always |
| `ls` | each link with its path and the branch of the linked checkout |
| `remove <package> [--no-deps]` | drops the entry; the file goes when nothing is left |

Every subcommand takes `-p, --project`, any folder inside the repository. Linked repos on different branch names are fine; `doctor` reports them as info.

### `dashcrew me`

Your personal profile, `~/.dashcrew/me.yaml`, one per machine and never committed. The repo decides the rules; the profile changes only how agents work with you. The guard never reads it, and a role grants nothing: in a repo outside your role you follow that repo's rules like its owners.

```yaml
role: mobile          # be | fe | mobile | devops | fullstack | lead
stacks: [flutter]     # the platforms you work on
level: regular        # new | regular
chat_language: id     # ISO 639-1; empty keeps each project's language.chat
agents: [claude-code] # claude-code, codex, antigravity
```

| Subcommand | |
|---|---|
| `init [-p <repo>] [--yes]` | proposes values from the repo's platform (and adds it to your stacks when a profile exists), asks to confirm each, writes the file |
| `show` | prints the profile and whether the personal files are in step |
| `sync [--remove]` | writes the personal instructions where each agent reads them, outside every repo; `--remove` takes out all of it, `me.yaml` stays |

What `sync` writes, and only into files it owns (a file without its marker is kept and reported, exit 1):

| Agent | Personal instructions | Personal skills |
|---|---|---|
| Claude Code | `$CLAUDE_CONFIG_DIR/rules/dashcrew-me.md` (default `~/.claude`) | `dashcrew-onboarding` with `level: new`, `dashcrew-lead` with `role: lead`, written to `~/.dashcrew/skills/` and linked into `<config dir>/skills/` |
| Codex | a block between `<!-- dashcrew:me:begin -->` and `<!-- dashcrew:me:end -->` in `$CODEX_HOME/AGENTS.md` (default `~/.codex`); the rest of the file is untouched | none |
| Antigravity | `~/.gemini/config/rules/dashcrew-me.md`, an `always_on` rule | none |

The instructions say what changes: outside your stacks (a project whose `platform` is not one of them) the agent asks every assumption at step 0, runs every review skill and explains one level deeper; `level: new` reads the knowledge index and the cross-repo flow before step 0; `chat_language` answers you in that language while code and PRs keep the project's; `role: lead` reports knowledge proposals, release lines and ports, reviews and specs when asked what needs attention. Run `dashcrew me sync` after editing `me.yaml`; `doctor` reports when the files are out of step.

## What `sync` writes

| | Claude Code | Codex | Antigravity |
|---|---|---|---|
| Rules | `AGENTS.md`, `CLAUDE.md` (`@AGENTS.md`) | `AGENTS.md` | `AGENTS.md` |
| Skills and workflows | `.claude/skills/<name>/SKILL.md` | `.agents/skills/<name>/SKILL.md` | `.agents/skills/<name>/SKILL.md` (shared with Codex) |
| Subagents (`reviewer`, `explorer`) | `.claude/agents/<name>.md` | `.codex/agents/<name>.toml` | `.agents/agents/<name>.md` |
| Guard hook | `.claude/settings.json` | `.codex/hooks.json` | `.agents/hooks.json`, key `dashcrew-guard` |
| MCP (the platform's `mcp_servers`; `dart-mcp-server` for flutter) | `.mcp.json` | `.codex/config.toml`, between `# dashcrew:begin` and `# dashcrew:end` | `.agents/mcp_config.json` |

Settings, hooks and MCP files are merged: your own entries stay, and an MCP server you defined differently is left alone with a warning. Every agent also gets `.ai/guard.json`, `.ai/lock.json`, the guard shim `.ai/bin/guard` and `.github/pull_request_template.md`; with `ci.github`, also `.github/workflows/dashcrew.yml`. A template or workflow your team wrote under those names is left alone with a warning.

## Guard

Each agent's pre-tool hook sends the tool call to `.ai/bin/guard`, which blocks:

- `git commit` (unless `policy.agent_commit`), `push`, `stash`, `reset`, `rebase`, `cherry-pick`, `revert`, `merge`, `tag`, including behind `sudo`, `env`, `rtk`, `fvm`, variable assignments, git global options (`-C`, `-c`), `&&` chains, `$( )`, and heredocs fed to a shell;
- `dart format`, `dart fix`, `flutter format` (unless `policy.formatter`);
- writes by a `read` subagent (`reviewer` and `explorer` are both `read`);
- importing the analytics vendor package in the app's `lib/` (the selected analytics pack's `vendor_import`), outside local packages;
- a `gh` command whose arguments contain `/approve`: approvals are the user's, on whatever approval flow a team keeps on GitHub.

Claude Code and Codex get exit code 2 with the reason on stderr; Antigravity gets `{"decision": "deny", "reason": ...}` on stdout.

**Backend rules.** A platform may add a `guard:` key, in its `platform.yaml` or in `platform_config`: `commands` (a program and the words after it, options skipped, such as `[terraform, apply]` or `[kubectl, delete]`), `sql` (`clients` such as `psql`, and `statements` such as `DROP`, matched as whole words in any case in the script that runs the client, so `-c`, heredocs, here-strings and pipes are covered) and `migrations` (`tools` such as `goose`, blocked when a connection string in an argument or a `VAR=` assignment names a host outside `local_hosts`, by default `localhost`, `127.0.0.1` and `::1`; Unix sockets are local). Not seen: SQL in a file (`psql -f`) and a connection string read from the environment or a config file. A project whose policy has these rules records `guard_min_version` 0.2.0, so the shim refuses an older dashcrew that would ignore them; other projects keep 0.1.0.

**Several repos.** A file the agent writes is checked under the `.ai/guard.json` of the nearest dashcrew project holding its real path, so a dependency reached through `dashcrew link`, a symbolic link or an absolute path keeps its own rules, and the app's rules do not leak into it. When the hook runs in a folder that is no project but holds projects one level down, a shell command is blocked when any of their policies blocks it, and a file under the project holding it; with no project below, nothing changes. A policy passed as a file outside any project keeps the single-policy check.

**Hidden commands.** The guard also looks inside nested shells (`sh -c`, `bash -c` and other shells with option clusters, `eval`, here-strings, heredocs, Codex argv lists; up to 8 levels deep), shell keywords and grouping (`{ ...; }`, `if ... then`, `do`, `!`), wrappers with options (`sudo -u`, `env -i`, `/usr/bin/env`, `nice`, `timeout`, `exec`, `xargs`, `find -exec`, `command`, `builtin`, `rtk proxy`), line continuations, and `apply_patch` headers with CRLF line ends or a leading `*** Move to:`. It does not block a command whose first word is a `$` expansion (`$x push`), `source` or `.`, `fish -c`, `stdbuf`, `doas`, or nesting deeper than 8 levels: blocking those would also block legitimate scripts.

**Malformed payloads.** A hook payload the guard cannot read (not JSON, not UTF-8, a field of the wrong type) is let through, so a changed agent hook format never stops work. Each one is recorded in `<git common dir>/dashcrew/guard-malformed.log` (time, agent, reason; at most 64 KiB), and `doctor` warns with `guard-log` when it has entries since the last `sync`. A policy file that is missing or broken also lets calls through; `doctor` reports it with `guard-policy`.

**Where it runs.** `.ai/bin/guard` is a short `sh` script that `sync` writes and you commit. It runs `dashcrew guard` from `PATH` with the hook's arguments and stdin. It fails closed: when `dashcrew` is missing, or older than the minimum version the lock records as `guard_min_version` (0.1.0, the first release with `dashcrew guard`), it refuses every tool call with a one-line reason and the install command. An old Dart-installed dashcrew reports 0.0.1 and is refused the same way. A passed version check is cached in `~/.dashcrew/guard-ok` as the dashcrew path and the minimum; while both match and that dashcrew is not newer than the cache, the shim skips `dashcrew --version` and starts one process per tool call. On the maintainer's machine a call takes about 7 ms through macOS `/bin/sh` (about 5 ms through `dash`), against 3 ms for `dashcrew guard` alone. It needs a POSIX shell, so Windows is not supported yet. Older dashcrew versions compiled a guard into `~/.dashcrew/guard/`; nothing uses that folder any more, so you can delete it.

## Merge gate on GitHub

The plan is confirmed in the session by the task owner; the team reviews in the pull request, where this gate holds. GitHub is the only infrastructure.

1. Set `ci: { github: true, dashcrew_ref: v0.1.0 }` (a release tag, `v0.1.0` or newer) and run `dashcrew sync`. Commit `.github/workflows/dashcrew.yml`. The workflow sets up Go and runs `go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@<dashcrew_ref>`, then `dashcrew check`.
2. In the repository settings, add a branch protection rule (or ruleset) for your base branches that requires the status check `check` from the `dashcrew check` workflow.
3. Every pull request then needs a current toolkit, every heading of the PR template, and commits that pass `dashcrew commit lint`. Failures show as annotations on the PR. The workflow checks out the full history (`fetch-depth: 0`) so the PR's commits can be read.

The workflow reads the PR body from the pull request event and needs no token beyond read access to the repository.

## Using dashcrew in a team

Teammates may use different agents in the same repository. They share one
AGENTS.md, one set of skills and subagents, and one guard policy; each agent gets
them in the format it reads.

### Set up once

1. List every agent the team uses: `agents: [claude-code, codex, antigravity]`.
2. Run `dashcrew sync` and commit everything it wrote:
   `AGENTS.md`, `CLAUDE.md`, `.claude/`, `.codex/`, `.agents/`, `.mcp.json`, and `.ai/` (`project.yaml`, `lock.json`, `guard.json`, `bin/guard`, `packs/`, `platforms/`).
   Teammates then get the toolkit on pull, and `dashcrew doctor` compares their checkout with the committed lock.

### Each teammate

- Install the same dashcrew as everyone else (see [Install](#install)).
- Nothing else to run after cloning: every agent tool call goes through the committed shim to `dashcrew guard`.
  Without dashcrew 0.1.0 or newer on `PATH` the shim refuses every tool call and names the install command.
- Run `dashcrew doctor` to confirm.
- Optional: `dashcrew me init` and `dashcrew me sync` for your personal profile (see [`dashcrew me`](#dashcrew-me)).
- Codex users: trust the project when Codex asks, then approve the dashcrew hook in `/hooks`. Codex ignores a project's `.codex/` until both are done.

### Day to day

- Change rules through `.ai/project.yaml`, `.ai/packs/`, `.ai/platforms/` or the project block in AGENTS.md (between `dashcrew:project:begin` and `dashcrew:project:end`), never by editing generated files. `doctor` reports generated files edited by hand.
- After a merge that touched `.ai/project.yaml`, `.ai/packs/` or `.ai/platforms/`: resolve conflicts in those files only, run `dashcrew sync`, and commit the regenerated files.
- `sync` refuses a lock written by a newer dashcrew; update dashcrew instead of forcing it.
  Release builds report their tag, and `go install` of a commit reports a version just above the previous tag. A local build without a version reports 0.0.1.

### What each agent enforces

| | Claude Code | Codex | Antigravity |
|---|---|---|---|
| Rules (AGENTS.md), skills, subagents | yes | yes | yes |
| Guard: no push, commit, history rewrite, formatter or vendor import | yes | yes, once trusted | yes, by its documented hook format |
| `reviewer` and `explorer` cannot write | guard | guard | the subagent's `tools` allowlist |
| AGENTS.md cut at | no documented cut | 32 KiB | 24,000 bytes |

dashcrew keeps AGENTS.md under the smallest cut of the selected agents, and `doctor` warns above it.

## Shared knowledge

A team can keep its product knowledge (how a flow works, a contract, a trap, a decision and its reason) in one reviewed git repo that every member's agents read. Agents propose; people review.

**Layout.** `README.md` (the generated index), `features/<slug>.md`, `repos/<repo>.md`, and `inbox/` for proposals. A page has front matter (`title`, `repos`, `tags`); each fact ends with `<!-- verified <repo>@<sha> <yyyy-mm-dd> -->`, and code pointers are written `` `<repo>:<path>:<line>` ``.

**Setup.**

1. `dashcrew knowledge init <dir>` writes that layout into a new folder. Commit it to a new repo and push it.
2. In each project, set `knowledge: { repo: <git url or path> }` in `.ai/project.yaml` (`name` defaults to the repo's last path segment).
3. Each member runs `dashcrew knowledge pull`: it clones the repo into `~/.dashcrew/knowledge/<name>`, or fetches and fast-forwards it. A diverged clone is reported, never reset.
4. `dashcrew sync` links `.ai/knowledge` to the clone (a copy on Windows), git-ignores the link, and registers the knowledge MCP server for every agent in the profile. `sync` never touches the network; with no clone it warns and carries on.

**Guard rule.** Agents read the whole clone but write only `.ai/knowledge/inbox/`. The guard checks the real path, so a write through the link, the absolute clone path, `..` or another symlink is blocked the same way, for edits, patches and shell redirections.

**Retro and review.** With `knowledge` set, `learn` turns each product lesson into one proposal in `inbox/` (a standing instruction for agents goes to `.ai/packs/` instead). `dashcrew knowledge review` walks the inbox oldest first: accept merges the proposal into its page with a verified marker (making the page when needed), edit opens `$EDITOR`, skip or delete. It then regenerates `README.md` and prints the `git` commands to open a pull request on the knowledge repo. A page it creates keeps only the sections a proposal filled, and a `target` written as the page path (`features/orders.md`) is read as its slug.

**Team flow.** Two members accepting into the same page conflict on its lines and on `README.md`. Keep page edits in one place:

1. Members open pull requests on the knowledge repo that add only `inbox/` files. Each file has its own dated name, so these never conflict.
2. One member at a time (a lead, or a rota) runs `dashcrew knowledge pull` and `dashcrew knowledge review` on the main branch and opens one pull request with the merged pages, where duplicate facts also show.

`knowledge review` fetches first and stops when the clone is behind its upstream (`--no-fetch` skips it); offline, or on a branch with no upstream, it only warns. On a conflict anyway: take either side of `README.md` and run `knowledge review` with an empty inbox to rebuild the index; merge a page by hand, keeping both sides' verified markers, then run `knowledge doctor`.

**Staleness.** `dashcrew knowledge doctor` reports code pointers whose path is gone at HEAD (for the current project and sibling folders), proposals waiting over 14 days, and pages with no verified marker in 180 days. It edits nothing, and exits 1 only on a stale pointer. `dashcrew doctor` runs it as the `knowledge-stale` check.

**MCP server.** `dashcrew knowledge serve [--clone <dir>]` is a stdio MCP server with three tools: `knowledge_search` (BM25 over title, headings and body of the pages and the spec files; page, kind, title, snippet, score, with an optional `kind` filter: `spec`, `feature`, `repo` or `other`, from the page's folder), `knowledge_read` (the page and its verified markers) and `knowledge_sources` (each code pointer as found, missing or unchecked). Its index lives in memory and is rebuilt when the clone's HEAD moves. `sync` registers it as `dashcrew-knowledge` in `.mcp.json`, `.codex/config.toml` and `.agents/mcp_config.json`, and removes it when `knowledge` is dropped. AGENTS.md, the `explorer` and the `plan` workflow tell agents to read the knowledge before planning and to check a page's code pointers before relying on it.

dashcrew never commits, pushes or resets the knowledge repo; every change to it goes through a member's own pull request.

## Specs

With `knowledge` set, a large feature gets a PRD and a TRD in the knowledge repo before its tasks start, and every task says which revision it builds.

**Format.** `specs/<release>/<feature>/prd.md` (problem, users, goals, scope, Not doing, acceptance criteria) and `trd.md` (impact per repo, contracts, data, rollout order, risks, decisions, open questions). `<release>` is a version, or `unreleased`. Both files carry front matter: `title`, `feature`, `release`, `status` (`draft`, `review`, `approved` or `superseded`), `rev` (from 1), `repos`, and `owners` as roles, never names or emails.

**IDs.** `AC-n` (criteria), `C-n` (contracts) and `D-n` (decisions) start a list item or heading. They are never renumbered; a dropped one stays as `~~AC-3~~ dropped: <why>`. Every open question ends with `owner: <role>`.

**Commands.**

| Command | |
|---|---|
| `spec new <feature> [--release <v>] [--clone <dir>]` | writes both files from the repo's templates, `status: draft`, `rev: 1`; the release defaults to the branch's recorded one, else `unreleased`. Refuses an existing folder |
| `spec lint [<path>] [--against <ref>]` | front matter, duplicate IDs, gaps in a new spec, unowned questions in an approved one; with `--against`, a vanished or renumbered ID and an approved file changed without a higher `rev`. Exit 1 on any problem. `knowledge doctor` runs it over `specs/` |
| `spec revise <path>` | run by a member: moves an approved spec back to `review` with `rev` one higher in both files, and prints the `git` commands for the pull request |
| `spec status [-p <dir>] [--clone <dir>]` | the spec the branch records against the clone's current `rev` and status, the ACs this repo's plans claim (plans under the plans path that name the spec folder), the ACs none claims, and the spec's other repos as not checked |

**Guard rule.** Under `specs/`, agents write only new files and files whose status is `draft` or `review` (read from the first 2 KB). Approved and superseded files are blocked with a pointer to `dashcrew spec revise`, which agents may not run. A patch that moves or deletes a file is checked at the old path too, so a page or spec cannot be moved into the inbox. Approval is the merged knowledge-repo pull request in which a member set `status: approved`; there is no approve command.

**Workflow.** The `spec` workflow (main session, outside the chain): one round of brainstorm questions, the PRD with `acceptance-criteria`, the TRD with `feature-architecture` and the platform's TRD sections, `spec lint`, then `status: review` and the `git` commands for the member.

**Tasks follow the spec.** Step 0 gains `Spec: specs/<release>/<feature>@rev<n> | none`; with a spec, the success criteria are its ACs by ID. The `branching` skill records it with `git config branch.<name>.dashcrew-spec <value>`. The `plan` names the spec and revision, lists the ACs it covers, and states each departure from the TRD as `departs from D-n: <what and why>`; an undeclared one makes it NO-GO, and the `reviewer` reports a change against a decision or contract the plan did not declare as HIGH. An accepted departure is followed by a `spec revise` pull request, named in the summary.

**Drift.** `doctor` warns, and never fails, when the recorded `rev` is older than the clone's ("spec moved to rev 4; re-read it") or when the spec is still `draft` while the branch has commits of its own (commits no other branch holds). After the feature ships, the knowledge-gated `learn` proposes its `features/<slug>.md` page through the inbox from the PRD's goals and the TRD's final decisions, with code pointers; a spec becomes `superseded` only when a newer spec replaces it.

Not done: an approve command, specs outside the knowledge repo, syncing specs to a tracker, generating code or tests from a spec.

## Release lines

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

**A task knows its release.** Step 0 gains `Release: <version> | none`, and the `branching` skill asks for it with the base and records it with `git config branch.<name>.dashcrew-release <version>`, local config the guard allows and nothing commits. The step 0 line, the `port` workflow and the release text in `branching`, `prompt-polish`, `plan`, `pr-description` and `learn` render only when `release_lines` is set.

**Toolkit version per line.** On a protected branch without `--upgrade`, when the lock's version or catalog digest is older or different, `sync` checks each differing file on its own: a file is written as profile-driven when rendering the lock's profile with the current catalog reproduces its locked hash; otherwise it is held as catalog-driven. Held files mean exit 1, with the two ways forward: `dashcrew sync --upgrade` on the line, or upgrade on the main line and merge it down. With `release_lines` set the lock records `inputs` (catalog digest, profile hash). `doctor`'s `release-line` check compares the line's toolkit with the main line's. Limits: the catalog digest covers only the built-in catalog, and adapter config merges (`.mcp.json`, `settings.json`) are not held.

**Merges between lines.** `doctor` and `check` report generated files with conflict markers or out of step with the lock. `dashcrew sync --after-merge` refuses a conflicted profile, rebuilds a conflicted lock from our side plus the files only their side has, renders fresh the owned files that are conflicted or drifted, and prints the `git add -- ...` line. It never stages or commits, and never touches files dashcrew does not own.

**Plans and the ledger.** A task with a release plans in `<plans>/<release>/<task>/`; one without stays in `<plans>/<task>/`. `dashcrew release status <version>` regenerates `<plans>/<version>/README.md`: each task folder with its branch (a local branch named `<task>` or ending in `/<task>`, one recording the release first) and PR description path; the commits on the line since the previous release's line or version tag (`2.4.0` or `v2.4.0`) that carry `Ported-from:`; and the pending ports into and out of the line.

**Ports.** With `release_lines` set, `commit lint` allows the trailers `Ported-from: <sha>` (7 to 40 hex characters) and `Port: yes`. `dashcrew check --ports` takes every fix of the older line since its branch point (the newest commit of its history on the main line's first-parent chain, so merging the line back does not move it): commits whose type is `fix` in the commit pattern, plus any commit with `Port: yes`, minus fixes reverted on that line. Each must reach the newer line, found in this order: merged (in its history), a `Ported-from:` trailer, a `cherry-pick -x` note, an equal `git patch-id --stable`. A port of a port counts, since the trailers and notes chain back to the original. A found fix is lost when a commit on the newer line says `This reverts commit <sha>` of it and is not reverted itself. It prints one line per fix, a summary, and exits 1 when any is missing or lost. Without `--from` and `--to` it checks every adjacent pair up to the main line, which covers older lines through the newer ones. A fast-forward merge of a line into the main line puts its commits on that chain, so an older merge-base with a newer line wins; only a line both fast-forwarded into the main line and merged into the newer line loses its cut. A CI checkout needs `fetch-depth: 0` with every line fetched.

**The `port` workflow.** The main session takes a merged range or PR on one line and the target lines (default every newer line, then main), creates one worktree per target with `dashcrew worktree create`, and prints the merge for the member to run: `git merge --no-ff --no-commit -X find-renames <source line>` by default, `git cherry-pick -x --no-commit` only with a stated reason. The guard keeps merge and cherry-pick the user's in every policy. Conflicts are listed with renamed paths, `verify` runs scoped per target, each target gets its own PR description naming the origin, and the commit command comes from `dashcrew commit suggest --ported-from <sha>`. The member commits and pushes.

**Knowledge.** Pages describe the main line. A version-specific part is tagged in place, such as `(since 2.5.0)` or `(2.4 only)`; never a folder or page per version. The knowledge-gated `learn` text says so when `release_lines` is set.

Not done: creating, tagging or deleting release branches; pushing, merging or opening PRs; version folders for code, generated files or knowledge; automatic ports on merge; releases across repos.

## Troubleshooting

| Message | Fix |
|---|---|
| `AGENTS.md exists and was not written by dashcrew` | `dashcrew sync --adopt` keeps your file as the project block |
| `Files changed outside dashcrew` | move the change to its source (profile, project pack, project block), then `dashcrew sync --force` |
| `.ai/lock.json was written by dashcrew X` | install that dashcrew (or the team's pinned ref) |
| `[dashcrew guard] dashcrew is not on PATH, so this tool call is refused` | install dashcrew (see [Install](#install)); the agent's next tool call goes through |
| `[dashcrew guard] the dashcrew on PATH is older than X` | install dashcrew X or newer; `doctor` names the `dashcrew` it found |
| `guard-binary`: `is the Dart one` | an old Dart-installed dashcrew (from before 0.1.0) comes first on `PATH`: run `dart pub global deactivate dashcrew`, then install dashcrew (see [Install](#install)) |
| `guard-log`: `the guard let N malformed payloads through` | update dashcrew if an agent changed its hook format; the log names the agent |
| `dashcrew guard install was removed` | an old shim called it: run `dashcrew sync` to write the new `.ai/bin/guard` |
| `plan_gate was removed in P4` | delete `policy.plan_gate` from `.ai/project.yaml`; see [Migrating to P4](#migrating-to-p4) |
| `check`: `pr-sections` `missing: ## ...` | add the heading to the PR description; write "None" under it when it does not apply |
| `gitignore`: `.ai/bin/ is in .gitignore` | remove that line so the shim is committed |
| `codex-trust`: `project not trusted` | trust the project when Codex asks, then approve the hook in `/hooks` |
| `skill <name> skipped` | a project pack replaced the pack that defined it; add `skills/<name>.yaml` to your pack |

## Migrating to P4

P4 makes dashcrew skill-first. A project synced by an earlier dashcrew migrates on its next `dashcrew sync`; there is no legacy option.

- **Roles removed.** Every old role subagent is deleted for every agent: the eleven each project had (planner, engineer, tech-lead, `worktree-monitor` and the others), plus `design-refiner` and `local-storage-engineer`, which the design and storage packs added. `reviewer` and `explorer` remain. What the roles checked now lives in skills (`acceptance-criteria`, `feature-architecture`, `figma-design`, `hive-storage`, `qa-identifiers`) and in the workflows.
- **`orchestrate` and `checkpoint` removed.** `orchestrate team` is now the default chain: step 0 with a `Skills:` line, then the tier's steps. `verify` checks a failing test once against the base instead of a checkpoint.
- **Plan gate removed.** No `dashcrew gate` command, no `plan-approval` check, no plan-gate guard rule; `.ai/guard.json` loses `plan_gate`. Delete `policy.plan_gate` from `.ai/project.yaml`: it is ignored with a warning for one release, then rejected as an unknown key. The merge gate (`dashcrew check`) stays.
- **What `sync` does.** It deletes every generated file it no longer writes, prints `note: Migrated to P4: ...` once, and rewrites AGENTS.md around your project block. Files the team wrote are never touched: plans and handoffs under `paths.plans`, project memory under `paths.memory`, and the AGENTS.md project block.
- **After it.** Commit the result. `doctor` lists leftover `<plans>/*/handoff/00-roster.md` files as `info`; nothing reads them, so delete them when the plans are done. Text in your project block or project packs that names a removed role or `orchestrate` is yours to edit.

## Development

Requires Go 1.26 or later (`go.mod` pins the toolchain). Direct dependencies: `go.yaml.in/yaml/v3` and `github.com/modelcontextprotocol/go-sdk`.

```bash
go vet ./...
go test ./...                              # or scoped: go test ./internal/<pkg>/...
DASHCREW_UPDATE_GOLDENS=1 go test ./...    # rewrite the goldens below, then review the diff
```

Goldens:

- `test/golden/<sample>/expected/` and `test/golden/init-app/expected-project.yaml`: the generated files, checked byte for byte by `internal/conformance`.
- `cmd/dashcrew/testdata/transcripts/`: stdout, stderr, exit code and file changes of every CLI call in the README, per case.
- `internal/guard/testdata/corpus_outcomes.txt`: the guard's answer to every case of its 13,724-case corpus that it blocks.

`test/golden/sample-migrate-p3/p3/` is a frozen pre-P4 output that the migration test syncs over; it is never rewritten. The output format (JSON layout, YAML scalars, Mustache rendering, help text) is the one the earlier Dart engine wrote, so a project sees no diff after upgrading.

CI runs `go vet` and `go test` on Linux, macOS and Windows (Windows is allowed to fail until it is supported), plus `gofmt`, `go test -race`, `govulncheck` and CodeQL, on every push to `main` and every pull request. Releases are cut by pushing a `v*` tag: `.github/workflows/release.yaml` runs goreleaser (`.goreleaser.yaml`), which stamps the version, builds darwin, linux and windows for amd64 and arm64, publishes a GitHub Release and updates the Homebrew formula in `fahnaladitia/homebrew-tap`. Contributor rules for AI agents are in [AGENTS.md](AGENTS.md).

## Security

Report a vulnerability privately; see [SECURITY.md](SECURITY.md).

## Contributing

Build, test, goldens and the commit pattern are in [CONTRIBUTING.md](CONTRIBUTING.md).

## License

BSD-3-Clause. See [LICENSE](LICENSE).
