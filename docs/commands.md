# Commands

Every `dashcrew` command with its flags; `dashcrew <command> --help` prints them too. `knowledge` and `spec` are in [knowledge.md](knowledge.md), `platform new` in [profile.md](profile.md#platforms).

## `dashcrew init`

`init` prints what it detected with the evidence, then asks for the platform: the one whose `detect` files it found, `other` when none matched, or your pick when several did. With `other` it asks for the detect files and each command. It then asks about every stack family of that platform, proposing the detected option: `state [bloc]: ` after finding `flutter_bloc` in `pubspec.yaml`, `design [-]: ` when nothing was found (only `flutter` detects options so far). Enter keeps the proposal, `-` drops the family, and `other <package>` names a library dashcrew has no pack for. `--yes` writes what it detected without asking, and stops when no platform or several matched.

| Flag | Default | |
|---|---|---|
| `-p, --project` | `.` | project root |
| `-y, --yes` | off | write the detected profile without asking |
| `--agents` | `claude-code` | agents to list in the profile (repeatable) |
| `--force` | off | replace an existing `.ai/project.yaml` |

Writes only `.ai/project.yaml`, plus the starter platform you pick: a repo no platform matches is offered the starter its root files point at (`go` for a `go.mod`, written as `dashcrew platform new go` writes it), else `platform: other`. Under `other`, the root files that identify a stack (`pyproject.toml`, `package.json`, `Cargo.toml`, `Makefile` and the like) are printed first and proposed as `detect`; the commands are always asked. A required answer left empty is asked once more, then `init` stops.

## `dashcrew sync`

| Flag | Default | |
|---|---|---|
| `-p, --project` | `.` | project root holding `.ai/project.yaml` |
| `--force` | off | overwrite generated files edited by hand, or a lock written by a newer dashcrew |
| `--adopt` | off | keep an existing, hand-written AGENTS.md as the project block |
| `--[no-]compile-guard` | off | deprecated, does nothing; prints a notice. There is no per-machine guard to install any more |

Running it twice writes nothing the second time. Files it stops generating are deleted.

## `dashcrew guard init`

| Flag | Default | |
|---|---|---|
| `-p, --project` | `.` | project root |
| `--agents` | `claude-code` | agents to write the hook for (repeatable) |
| `--force` | off | replace a `.ai/guard.json` or `.ai/bin/guard` that `guard init` did not write |

The guard alone, without `.ai/project.yaml`: writes the default policy `.ai/guard.json` (git `commit`, `push`, `stash`, `reset`, `rebase`, `cherry-pick`, `revert`, `merge` and `tag` blocked, formatters blocked), the shim `.ai/bin/guard`, and each agent's hook entry into the file `sync` merges it into, keeping your own entries. Then it runs the guard checks of `doctor`, so a missing or old dashcrew on `PATH` shows at once, and exits 1 when one fails. A second run writes nothing. It refuses a project with a profile, where `sync` writes the guard. A later `dashcrew init` and `dashcrew sync` replace the policy and the shim without `--force` while they are as `guard init` wrote them; `doctor` in a guard-only project runs only the guard checks.

## `dashcrew guard [<policy>]`

The guard hook: reads one tool call as hook JSON on stdin and answers per agent (see [Guard](guard.md)). The policy is `<policy>`, else `.ai/guard.json` in the hook's working directory. Hooks call it through `.ai/bin/guard`; you do not run it yourself. `dashcrew guard install` was removed with the per-machine compiled guard.

## `dashcrew doctor`

`-p, --project` (default `.`). Writes nothing; exit 1 when any check fails.

| Group | Checks |
|---|---|
| Profile and lock | `profile`, `project-packs`, `lock`, `version`, `drift` (generated files edited by hand), `stale` (output differs from the current profile and catalog) |
| Guard | `guard-policy`, `guard-binary` (the shim; the `dashcrew` first on `PATH`, which fails when missing and warns when it is an old Dart-installed one or older than the lock's `guard_min_version`; then a `git push` event through the shim), `guard-log` (only when the guard let tool calls through unchecked since the last sync: a malformed payload, an unknown tool, or a policy it could not use) |
| Claude Code | `hook`, `claude-md`, `mcp`, `skills` (fails on a missing dashcrew skill; warns on a required project skill left out, or a skill the AGENTS.md project block names that is not installed; an outside skill the generated files use only when installed, such as `idea-refine`, is a note) |
| Codex | `codex-hook`, `codex-mcp`, `codex-skills-budget`, `codex-trust` |
| Antigravity | `antigravity-hook`, `antigravity-mcp`, `antigravity-roles` |
| Knowledge | `knowledge` (the link and the clone's last pull), `knowledge-stale`, `spec` (only when the branch records a spec; warns, never fails) |
| Project | `agents-md-size`, `always-on` (per agent, what it loads from the project at session start: CLAUDE.md and its `@` imports, `.claude/rules` without `paths:`, `AGENTS.md` or `AGENTS.override.md`, Antigravity rules with `trigger: always_on`, and the memory index; warns over 40 KiB, about 10k tokens; also names the headings of the AGENTS.md project section that repeat a generated heading), `memory-index` (only when `{{paths.memory}}/MEMORY.md` exists; warns for a line over 150 characters), `gitignore` (warns while `.ai/bin/` is ignored), `handoff-rosters` (only when found) |
| Personal | `me` (your role and level here with their source, and the personal files `dashcrew me sync` writes; warns, and fails only when git tracks a personal file) |

Agent checks run only for the agents in the profile. Each line starts with `ok`, `warn`, `fail`, `skip` or `info`; only `fail` sets exit 1. `info` reports something that needs no fix, such as `handoff-rosters`: `<plans>/*/handoff/00-roster.md` files left by the team flow of a dashcrew before 0.1.0.

## `dashcrew check`

| Flag | Default | |
|---|---|---|
| `-p, --project` | `.` | project root |
| `--pr-body <file>` | none | the PR description to check; in a pull request workflow the event supplies it |
| `--base <branch>` | none | lint the commits of `<base>..HEAD`; in a pull request workflow the event's base and head supply the range |
| `--ports` | off | only the port check of [release lines](release-lines.md); runs alone, without `--pr-body` or `--base` |
| `--from <line>`, `--to <line>` | every adjacent pair | with `--ports`: the older and the newer line, each a version, a line branch or `main` |

Read-only; exit 1 on any failure, with `::error` annotations in GitHub Actions. It runs the toolkit checks of `doctor` that need no agent or guard binary and the guard hook check of each agent in the profile (`hook`, `codex-hook`, `antigravity-hook`, read from the committed files, so a hook lost to a hand edit or a merge fails the PR), then `pr-sections` (every heading of `.github/pull_request_template.md` in the PR body) and `commits` (`dashcrew commit lint` on every commit of the PR but merges; skipped without a base). With `release_lines.check_ports: true` it adds `ports`, the check of `--ports` over every adjacent pair of lines. With `project.links` it adds `links`: each listed dependency as committed in `pubspec.yaml` and `pubspec.lock`, or `go.mod`. A git ref must be a tag, or a commit on the dependency's default branch (a branch name is refused, even the default one); a `path` must stay inside the repository; a Go `replace` to a local folder and a pseudo-version off the default branch are refused; hosted versions pass, since `pub get` and `go mod download` fail on an unpublished one; a committed link file fails. Refs are read with `git ls-remote` and a commits-only fetch, using the credentials CI already has for the dependency; an unreachable repository fails the check. `dashcrew check --links` runs only this, one line per dependency, before a push.

## `dashcrew commit`

Commit messages in the pattern of `project.commit`. Neither subcommand commits; both take `-p, --project`, any folder inside the repository.

| Subcommand | |
|---|---|
| `suggest [--subject <s>] [--body <b>] [--type <t>] [--scope <s>] [--ported-from <sha>[,<sha>...]] [--json]` | prints one line, `git add -- <files> && git commit -m ...`, with the changed tracked files listed one by one and every value quoted for bash and zsh. Type, scope and ticket come from the branch and the files; untracked files are listed apart, never added. With a verify record it says which files changed after the last verify. `--ported-from` adds one `Ported-from:` trailer per commit, after the ticket trailer. With `--subject` the message is linted; a problem exits 1. Never shows an email address from the branch or the git config |
| `lint <message> \| - \| --range <base>..<head>` | checks the pattern, type, subject length, body rule, trailers, email addresses (`name@host.tld`, also inside `<...>`; `@mention` and image suffixes such as `icon@2x.png` are not addresses) and AI attribution (generated-with footers, AI co-author trailers, robot lines). One line per problem, exit 1 |

## `dashcrew release`

Needs `project.release_lines`; both subcommands take `-p, --project`.

| Subcommand | |
|---|---|
| `lines` | the release and hotfix lines oldest first, then the main line, with their head commits, and how the current branch resolves |
| `status <version>` | regenerates `<plans>/<version>/README.md`, the release ledger, and prints it. Writes only that file and runs no git write command |

## `dashcrew verify record`

`-p, --project`. Records the tree hash of the working copy, untracked files included, in `<git dir>/dashcrew/last-verify` (per worktree). The hash is built in a temporary index (`GIT_INDEX_FILE`), so the real index and staging are never touched. The `verify` workflow runs it after a PASS.

## `dashcrew worktree`

One worktree per parallel task, at `<repo>.worktrees/<name>`. It never commits, pushes or deletes a branch.

| Subcommand | |
|---|---|
| `create <branch> <base> [--name <dir>] [--no-pub]` | new branch with no upstream; links the git-ignored toolkit files from the main checkout; records the base; runs the platform's `deps` command when one of its `detect` files is there (`flutter pub get`, through `fvm` when `.fvmrc` exists) |
| `list` | branch, base, upstream, dirty lines, missing links |
| `link <name>` | restores missing links and drops an upstream the branch should not have |
| `remove <name>` | removes a clean worktree; the branch stays |
| `watch [<name>...] [--once] [--state-dir <dir>] [--interval <s>]` | one line per new event: `commit`, `pushed`, `pr` (through `gh`), `base-moved`, `merge-tree`, `overlap`, `done` |

Every subcommand takes `-p, --project`, any folder inside the repository.

## `dashcrew link`

Points a dependency listed in `project.links` at a local checkout while a task spans repos, through the platform's link file: `pubspec_overrides.yaml` for Flutter, `go.work` for a Go project (`platform_config.link`). The file is added to the checkout's git exclude file and is never committed; `doctor` warns when git would commit it, and fails when it is committed.

| Subcommand | |
|---|---|
| `add <package> <path> [--force] [--no-deps]` | checks that `<path>` holds that package (pubspec `name` or `go.mod` module), writes the entry, excludes the file and runs the platform's `deps` command. A link file dashcrew did not write is refused without `--force`, and a committed one always |
| `ls` | each link with its path and the branch of the linked checkout |
| `remove <package> [--no-deps]` | drops the entry; the file goes when nothing is left |

Every subcommand takes `-p, --project`, any folder inside the repository. Linked repos on different branch names are fine; `doctor` reports them as info.

## `dashcrew me`

Your personal profile, never committed. The repo decides the rules; the profile changes only how agents work with you. The guard never reads it, and a role grants nothing: in a repo outside your role you follow that repo's rules like its owners.

It has two layers. `~/.dashcrew/me.yaml`, one per machine, describes you; a repo's `.ai/me.yaml` holds your `role` and `level` in that repo when they differ from home (lead in one repo, new in another). `.ai/me.yaml` is listed in the clone's `.git/info/exclude`, so it never reaches the team.

```yaml
# ~/.dashcrew/me.yaml
role: mobile          # default role: be | fe | mobile | devops | fullstack | lead
stacks: [flutter]     # the platforms you work on
level: regular        # default level: new | regular
chat_language: id     # ISO 639-1; empty keeps each project's language.chat
agents: [claude-code] # claude-code, codex, antigravity
skills: [ui-slop]     # your own skills in ~/.dashcrew/skills/<name>/, linked for Claude Code

# <repo>/.ai/me.yaml
role: lead            # only role, level and skills, only what differs from home
skills: [ui-slop]     # optional skills of this repo (.ai/skills) you turn on
```

| Subcommand | |
|---|---|
| `init [-p <repo>] [--home] [--yes]` | the first time (or with `--home`): proposes the home profile from the repo's platform and asks to confirm each value. Inside a dashcrew repo afterwards: asks your role and level there, keeps what differs from home in `.ai/me.yaml` (nothing differs: the repo is forgotten), then syncs |
| `show [-p <repo>]` | prints the profile, your role and level in this repo with where each comes from, the repos set up, and whether the personal files are in step |
| `sync [-p <repo>] [--remove-repo] [--prune] [--remove]` | writes the personal instructions for home and every repo set up, links your own skills for Claude Code, and writes each repo's optional skills you turned on (git-excluded); `--remove-repo` takes this repo's files out and forgets it, `--prune` forgets repos whose `.ai/me.yaml` is gone, `--remove` takes out everything; the `me.yaml` files stay |

What `sync` writes, and only into files it owns (a file without its marker is kept and reported, exit 1):

| Agent | Home | Per repo with `.ai/me.yaml` | Personal skills |
|---|---|---|---|
| Claude Code | `$CLAUDE_CONFIG_DIR/rules/dashcrew-me.md` (default `~/.claude`) | `<repo>/.claude/rules/dashcrew-me.md` | `dashcrew-onboarding` and `dashcrew-lead` when home or any repo selects them, written to `~/.dashcrew/skills/` and linked into `<config dir>/skills/`; your own skills listed in `skills`, linked the same way |
| Codex | a block between `<!-- dashcrew:me:begin -->` and `<!-- dashcrew:me:end -->` in `$CODEX_HOME/AGENTS.md` (default `~/.codex`); the rest of the file is untouched | an "In the repository at `<path>`" section in that block, since Codex has no personal file per repo | none |
| Antigravity | `~/.gemini/config/rules/dashcrew-me.md`, an `always_on` rule | `<repo>/.agents/rules/dashcrew-me.md` | none |

The instructions say what changes: outside your stacks (a project whose `platform` is not one of them) the agent asks every assumption at step 0, runs every review skill and explains one level deeper; `level: new` reads the knowledge index and the cross-repo flow before step 0; `chat_language` answers you in that language while code and PRs keep the project's; `role: lead` reports knowledge proposals, release lines and ports, reviews and specs when asked what needs attention. A repo's file says which role and level apply there and switches the lead and new-member parts on or off. A task worktree from `dashcrew worktree create` links these git-excluded files from the main checkout like the rest of the toolkit (`worktree link` adds them to an older worktree). Run `dashcrew me sync` after editing either file; `doctor` reports files out of step and fails when git tracks a personal file.

## `dashcrew skill adopt <name | folder>`

Copies a skill folder into `.ai/skills/<name>/`, supporting files included, and writes its `meta.yaml` from the `SKILL.md` front matter (`description` required, `when_to_use` kept). Hidden files and files over the limits are left out and named. `<name>` is a skill in `~/.dashcrew/skills/`; a path is a local folder holding a `SKILL.md`, and the skill takes the folder's name. It is optional unless `--required`. It refuses a name a dashcrew skill, workflow or existing project skill has, and a URL. It never commits; run `dashcrew sync` and commit `.ai/skills/<name>` and what sync writes.

## `dashcrew usage`

Reads the Claude Code (`$CLAUDE_CONFIG_DIR` or `~/.claude/projects/`) and Codex (`$CODEX_HOME` or `~/.codex/sessions/`) transcripts whose working directory is this checkout, one of its worktrees or a folder inside them, and prints per session: tokens (input, cache write, cache read, output) per role (`main`, `reviewer`, `explorer`, `other:<agent type>`), how many runs and responses each role had, `start` (the context of the first answer: what the agent loaded before doing anything) and the skills each role loaded (a `Skill` call, a `/<skill>` command, a `SKILL.md` read). `--since <yyyy-mm-dd | Nd>` (default `7d`), `--session <id prefix>`, `--agent claude|codex`, `--json` for a before and after comparison. Writes nothing and sends nothing anywhere. No cost in money: prices are not in the transcripts.
