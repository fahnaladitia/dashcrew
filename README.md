# dashcrew

Agent-agnostic AI coding toolkit for Flutter projects.

dashcrew keeps one neutral source of rules, roles, workflows and guards, and
renders it for the AI coding agents your team uses: Claude Code, Codex and
Antigravity. Your stack is chosen once, in `.ai/project.yaml`.

> Status: pre-release, not on pub.dev yet. Claude Code output has been run
> end to end. Codex and Antigravity output is generated and checked against
> their documentation (and, for Codex, its source), but not yet against a live
> session.

Not affiliated with or endorsed by Google, the Flutter team, Anthropic or OpenAI.

## How it works

```
core/ + packs/ (shipped)   --+
.ai/packs/ (your project)  --+-->  dashcrew sync  -->  AGENTS.md         read by every agent
.ai/project.yaml           --+                         per-agent files   skills, roles, hooks, MCP
                                                       .ai/guard.json    one policy for every agent
                                                       .ai/bin/guard     compiled per machine
                                                       .ai/lock.json     hashes, versions, owned entries
```

- **Rules** go into one `AGENTS.md`. Your own rules live in its project block, which `sync` never rewrites.
- **Workflows** (`plan`, `tdd`, `verify`, ...) and **skills** become each agent's skill format.
- **Roles** (planner, engineer, reviewer, ...) become each agent's subagent format, with a capability: `read`, `write-plans` or `write-code`.
- **The guard** is one compiled binary that every agent's pre-tool hook calls. It reads the policy in `.ai/guard.json` and blocks the commands and writes listed in [Guard](#guard).
- `doctor` checks the result against the lock without writing anything.

## Install

Requires git and Dart 3.9 or later (Flutter 3.35 or later).

```bash
dart pub global activate --source git https://github.com/fahnaladitia/dashcrew
```

Pin a commit with `--git-ref <sha>` so everyone on a team runs the same dashcrew.

## Quick start

In the Flutter project root:

```bash
dashcrew init      # reads pubspec.yaml, lib/main_<flavor>.dart and remote branches; writes .ai/project.yaml
dashcrew sync      # writes the toolkit and compiles the guard
dashcrew doctor    # checks it; exit 1 on any failure
```

`init` prints what it detected with the evidence, then asks about every stack family, proposing the detected option (`state [bloc]: ` after finding `flutter_bloc` in `pubspec.yaml`, `design [-]: ` when nothing was found). Enter keeps the proposal, `-` drops the family, and `other <package>` names a library dashcrew has no pack for. `--yes` writes what it detected without asking.

## The profile: `.ai/project.yaml`

```yaml
toolkit: 1                                # required
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
```

| Key | Default | Effect |
|---|---|---|
| `agents` | none, required | `claude-code`, `codex`, `antigravity`; files are written for each |
| `language.dartdoc`, `.pr`, `.chat` | `en` | language of Dartdoc, PR descriptions and chat (ISO 639-1) |
| `stack.<family>` | not set | an option name, or `{use: <option>, <param>: ...}`; see [Stack options](#stack-options) |
| `project.flavors` | none | flavor name to entrypoint; rendered as `flutter run --flavor <name> -t <entrypoint>` |
| `project.local_packages` | none | path packages; the app's `lib/` rules and the vendor import check skip them |
| `project.base_branches` | `[main]` | proposed bases for new branches |
| `project.branch_prefix` | `feat/`, `fix/`, `hotfix/` | branch prefixes per change type |
| `project.build_wrapper` | none | script that injects `--dart-define`s for real builds |
| `policy.agent_commit` | `false` | `true` lets the agent commit; push stays blocked |
| `policy.formatter` | `false` | `true` allows `dart format` on changed files only |
| `policy.dartdoc_max_lines` | `3` | hard cap per documented unit |
| `policy.tier_gates` | `true` | trivial and small tasks with a GO plan continue without waiting |
| `policy.terse_prompts` | `false` | agents answer short and blunt |
| `paths.*` | `.ai/plans`, `.ai/audits`, `.ai/pr`, `.ai/memory` | where plans, audits, PR descriptions and project memory live |

## Stack options

| Family | Options | Shipped with dashcrew |
|---|---|---|
| `state` | `bloc`, `riverpod`, `provider`, `none` | rules, a patterns skill, test guidance |
| `di` | `get_it` (param `injectable`), `riverpod`, `provider`, `none` | registration rules |
| `router` | `go_router`, `auto_route`, `getx`, `none` | navigation rules |
| `network` | `dio`, `http`, `retrofit`, `none` | client placement, error mapping, test client |
| `storage` | `hive`, `shared_preferences`, `sqflite`, `none` | access through repositories, migrations |
| `test` | `manual-fakes`, `mocktail`, `mockito`, `none` | mocking conventions |
| `analytics` | `posthog`, `firebase`, `mixpanel`, `none` | ask-before-tracking, facade-only imports |
| `tracker` | `clickup`, `none` | task status flow |
| `qa` | `semantics-identifier` | QA identifier rules |
| `design` | `figma`, `none` | design-to-code role |

Every family also takes `{use: other, name: <package>}` for a library without a pack.
Pack params and their defaults are in `packs/<family>/<option>/pack.yaml`; set them in the map form (`state: {use: bloc, shared_dir: lib/shared/blocs/}`).

**Project packs.** Put your own packs in `.ai/packs/<family>/<option>/`, in the same format. One with a shipped name replaces it; a new family (`architecture/clean`) becomes a valid `stack` key for that project. See [docs/catalog-format.md](docs/catalog-format.md).

## Commands

### `dashcrew init`

| Flag | Default | |
|---|---|---|
| `-p, --project` | `.` | Flutter project root holding `pubspec.yaml` |
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
| `--[no-]compile-guard` | on | compile `.ai/bin/guard` when the guard sources changed |

Running it twice writes nothing the second time. Files it stops generating are deleted.

### `dashcrew doctor`

`-p, --project` (default `.`). Writes nothing; exit 1 when any check fails.

| Group | Checks |
|---|---|
| Profile and lock | `profile`, `project-packs`, `lock`, `version`, `drift` (generated files edited by hand), `stale` (output differs from the current profile and catalog) |
| Guard | `guard-policy`, `guard-binary` (runs it on a `git push` event and expects a block) |
| Claude Code | `hook`, `claude-md`, `mcp`, `skills` |
| Codex | `codex-hook`, `codex-mcp`, `codex-skills-budget`, `codex-trust` |
| Antigravity | `antigravity-hook`, `antigravity-mcp`, `antigravity-roles` |
| Project | `agents-md-size`, `gitignore` |

Agent checks run only for the agents in the profile.

### `dashcrew worktree`

One worktree per parallel task, at `<repo>.worktrees/<name>`. It never commits, pushes or deletes a branch.

| Subcommand | |
|---|---|
| `create <branch> <base> [--name <dir>] [--no-pub]` | new branch with no upstream; links the git-ignored toolkit files from the main checkout; records the base; runs `flutter pub get` (`fvm` when `.fvmrc` exists) |
| `list` | branch, base, upstream, dirty lines, missing links |
| `link <name>` | restores missing links and drops an upstream the branch should not have |
| `remove <name>` | removes a clean worktree; the branch stays |
| `watch [<name>...] [--once] [--state-dir <dir>] [--interval <s>]` | one line per new event: `commit`, `pushed`, `pr` (through `gh`), `base-moved`, `merge-tree`, `overlap`, `done` |

Every subcommand takes `-p, --project`, any folder inside the repository.

## What `sync` writes

| | Claude Code | Codex | Antigravity |
|---|---|---|---|
| Rules | `AGENTS.md`, `CLAUDE.md` (`@AGENTS.md`) | `AGENTS.md` | `AGENTS.md` |
| Skills and workflows | `.claude/skills/<name>/SKILL.md` | `.agents/skills/<name>/SKILL.md` | `.agents/skills/<name>/SKILL.md` (shared with Codex) |
| Roles | `.claude/agents/<role>.md` | `.codex/agents/<role>.toml` | `.agents/agents/<role>.md` |
| Guard hook | `.claude/settings.json` | `.codex/hooks.json` | `.agents/hooks.json`, key `dashcrew-guard` |
| MCP (`dart-mcp-server`) | `.mcp.json` | `.codex/config.toml`, between `# dashcrew:begin` and `# dashcrew:end` | `.agents/mcp_config.json` |

Settings, hooks and MCP files are merged: your own entries stay, and an MCP server you defined differently is left alone with a warning. Every agent also gets `.ai/guard.json`, `.ai/lock.json` and, in `.gitignore`, `.ai/bin/`.

## Guard

Each agent's pre-tool hook sends the tool call to `.ai/bin/guard`, which blocks:

- `git commit` (unless `policy.agent_commit`), `push`, `stash`, `reset`, `rebase`, `cherry-pick`, `revert`, `merge`, `tag`, including behind `sudo`, `env`, `rtk`, `fvm`, variable assignments, git global options (`-C`, `-c`), `&&` chains, `$( )`, and heredocs fed to a shell;
- `dart format`, `dart fix`, `flutter format` (unless `policy.formatter`);
- writes by a `read` role, and writes outside `paths.plans` by a `write-plans` role;
- importing the analytics vendor package in the app's `lib/` (the selected analytics pack's `vendor_import`), outside local packages.

Claude Code and Codex get exit code 2 with the reason on stderr; Antigravity gets `{"decision": "deny", "reason": ...}` on stdout. The guard fails open: a missing or broken guard never stops work, and `doctor` reports it.

## Using dashcrew in a team

Teammates may use different agents in the same repository. They share one
AGENTS.md, one set of skills and roles, and one guard policy; each agent gets
them in the format it reads.

### Set up once

1. List every agent the team uses: `agents: [claude-code, codex, antigravity]`.
2. Run `dashcrew sync` and commit everything it wrote, except `.ai/bin/` (already in `.gitignore`):
   `AGENTS.md`, `CLAUDE.md`, `.claude/`, `.codex/`, `.agents/`, `.mcp.json`, and `.ai/` (`project.yaml`, `lock.json`, `guard.json`, `packs/`).
   Teammates then get the toolkit on pull, and `dashcrew doctor` compares their checkout with the committed lock.

### Each teammate

- Install the same dashcrew as everyone else (same `--git-ref`).
- Run `dashcrew sync` once after cloning. It compiles the guard into `.ai/bin/`, which is per machine.
  Until then no guard rule applies: a hook whose command is missing does not block.
- Run `dashcrew doctor`.
- Codex users: trust the project when Codex asks, then approve the dashcrew hook in `/hooks`. Codex ignores a project's `.codex/` until both are done.

### Day to day

- Change rules through `.ai/project.yaml`, `.ai/packs/` or the project block in AGENTS.md (between `dashcrew:project:begin` and `dashcrew:project:end`), never by editing generated files. `doctor` reports generated files edited by hand.
- After a merge that touched `.ai/project.yaml` or `.ai/packs/`: resolve conflicts in those files only, run `dashcrew sync`, and commit the regenerated files.
- `sync` refuses a lock written by a newer dashcrew; update dashcrew instead of forcing it.
  Until dashcrew has numbered releases, every commit reports the same version, so the version check cannot tell two commits apart: pin the same ref.

### What each agent enforces

| | Claude Code | Codex | Antigravity |
|---|---|---|---|
| Rules (AGENTS.md), skills, roles | yes | yes | yes |
| Guard: no push, commit, history rewrite, formatter or vendor import | yes | yes, once trusted | yes, by its documented hook format |
| Read-only roles cannot write | guard | guard | the role's `tools` allowlist |
| Plans-only roles write only under the plans folder | guard | guard | instruction only |
| AGENTS.md cut at | no documented cut | 32 KiB | 24,000 bytes |

dashcrew keeps AGENTS.md under the smallest cut of the selected agents, and `doctor` warns above it.

## Troubleshooting

| Message | Fix |
|---|---|
| `AGENTS.md exists and was not written by dashcrew` | `dashcrew sync --adopt` keeps your file as the project block |
| `Files changed outside dashcrew` | move the change to its source (profile, project pack, project block), then `dashcrew sync --force` |
| `.ai/lock.json was written by dashcrew X` | install that dashcrew (or the team's pinned ref) |
| `guard-binary`: `.ai/bin/guard missing` | `dashcrew sync` compiles it |
| `codex-trust`: `project not trusted` | trust the project when Codex asks, then approve the hook in `/hooks` |
| `skill <name> skipped` | a project pack replaced the pack that defined it; add `skills/<name>.yaml` to your pack |

## Development

```bash
dart analyze --fatal-infos
dart test
DASHCREW_UPDATE_GOLDENS=1 dart test test/golden   # rewrite golden fixtures, then review the diff
```

CI runs both on Dart 3.9.0 and stable. Contributor rules for AI agents are in [AGENTS.md](AGENTS.md).

## License

BSD-3-Clause. See [LICENSE](LICENSE).
