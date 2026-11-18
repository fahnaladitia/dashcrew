# dashcrew

One source of rules, skills and guard hooks for every AI coding agent your team uses. One Go binary, any stack.

[![ci](https://github.com/fahnaladitia/dashcrew/actions/workflows/ci.yaml/badge.svg)](https://github.com/fahnaladitia/dashcrew/actions/workflows/ci.yaml)
[![release](https://img.shields.io/github/v/release/fahnaladitia/dashcrew)](https://github.com/fahnaladitia/dashcrew/releases/latest)
[![license](https://img.shields.io/github/license/fahnaladitia/dashcrew)](LICENSE)

An agent tries to push for you, hidden inside a nested shell. Every agent's pre-tool hook sends the call to the same guard, which says no:

```console
$ echo '{"tool_name":"Bash","tool_input":{"command":"bash -c \"git push origin main\""}}' | dashcrew guard
[dashcrew guard] git push is reserved for the user. Ask them to run it.
$ echo $?
2
```

Claude Code, Codex and Antigravity each get that answer in the format they read. Flutter is the built-in platform; any other stack works through `platform: other` or a platform of your own.

Not affiliated with or endorsed by Google, the Flutter team, Anthropic or OpenAI.

## Why a team picks it

- **One guard policy for every agent.** The same rules for `git push`, history rewrites, formatters and read-only subagents in Claude Code, Codex and Antigravity. The guard looks inside `sh -c`, `env`, `sudo`, `&&` chains and heredocs, and the committed hook fails closed when dashcrew is missing.
- **A lock and a check in CI.** `sync` records every file it owns in `.ai/lock.json`. `dashcrew check` fails a pull request whose toolkit was edited by hand or is out of date, whose description misses a heading of the template, or whose commits break the team's pattern.
- **Team and personal profiles.** The repo's `.ai/project.yaml` decides the rules for everyone. Each member's `~/.dashcrew/me.yaml` changes only how agents work with them (chat language, role, their own skills), never what the project allows.

## Quickstart

```bash
curl -fsSL https://raw.githubusercontent.com/fahnaladitia/dashcrew/main/install.sh | sh
# or: brew install fahnaladitia/tap/dashcrew
# or, with Go 1.26 or later: go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@latest
```

`install.sh` (macOS and Linux, amd64 and arm64) puts the latest release in `~/.local/bin` without `sudo`, and installs nothing unless the release's `checksums.txt` lists the archive with the same SHA-256. `DASHCREW_VERSION=0.6.0` pins a release; `gh attestation verify <archive> -R fahnaladitia/dashcrew` checks build provenance. dashcrew needs git and a POSIX shell.

In the project root:

```bash
dashcrew init      # detects the platform and stack, asks what it could not detect, writes .ai/project.yaml
dashcrew sync      # writes the toolkit for every agent in the profile
dashcrew doctor    # checks it against the lock; exit 1 on any failure
```

Commit what `sync` wrote. A teammate who installs the same dashcrew needs nothing else (Codex and Antigravity ask once to trust the project): every agent tool call goes through the committed shim `.ai/bin/guard`. The full setup for a team is in [docs/team.md](docs/team.md).

## What `sync` writes

| | Claude Code | Codex | Antigravity |
|---|---|---|---|
| Rules | `AGENTS.md`, `CLAUDE.md` (`@AGENTS.md`) | `AGENTS.md` | `AGENTS.md` |
| Skills and workflows | `.claude/skills/<name>/SKILL.md` | `.agents/skills/<name>/SKILL.md` | `.agents/skills/<name>/SKILL.md` (shared with Codex) |
| Subagents (`reviewer`, `explorer`) | `.claude/agents/<name>.md` | `.codex/agents/<name>.toml` | `.agents/agents/<name>.md` |
| Guard hook | `.claude/settings.json` | `.codex/hooks.json` | `.agents/hooks.json`, key `dashcrew-guard` |
| MCP (the platform's `mcp_servers`) | `.mcp.json` | `.codex/config.toml`, between `# dashcrew:begin` and `# dashcrew:end` | `.agents/mcp_config.json` |

Settings, hooks and MCP files are merged: your own entries stay, and an MCP server you defined differently is left alone with a warning. Every agent also gets `.ai/guard.json` (the policy), `.ai/lock.json`, the shim `.ai/bin/guard` and `.github/pull_request_template.md`; with `ci.github`, also `.github/workflows/dashcrew.yml`. A template or workflow your team wrote under those names is left alone with a warning.

- **Rules** go into one `AGENTS.md`. Your own rules live in its project block, which `sync` never rewrites.
- **One agent plans and builds.** The main session sizes each task, then runs `plan`, `tdd`, `code-review` and `verify` as the size needs. Skills, not personas, carry the expertise (`plan`, `tdd`, `code-review`, `verify`, `learn`, `audit`, `worktree`, `build-fix`, and `port` with release lines); AGENTS.md indexes them.
- **Two read-only subagents**: `reviewer` for an independent review, `explorer` to map many files.
- **Platforms** hold what belongs to one stack: commands, rules, skills and packs for its libraries. See [docs/profile.md](docs/profile.md).

## The guard, honestly

The guard blocks, in every agent:

- `git commit`, `push`, `reset`, `rebase`, `merge`, `stash`, `tag` and the other history commands, unless the profile lets the agent commit or push (and then never a force push, a deleted branch or a push to a base branch);
- `dart format` and `dart fix`, unless the profile allows the formatter;
- writes by a read-only subagent;
- the analytics vendor package imported in the app's `lib/`, and `gh` approvals;
- on backend platforms, the commands and SQL statements the platform lists, and migrations against a remote database.

It prevents accidents, not a determined attacker. It is not a sandbox, and its known misses (a first word that is a `$` expansion, `source`, `fish -c`, nesting deeper than 8 levels) are listed in [docs/guard.md](docs/guard.md). A hook payload it cannot read is let through and logged, and `doctor` reports it.

## Compared with

| | dashcrew | rulesync | Ruler | Hand-written hooks |
|---|---|---|---|---|
| Install | one Go binary | npm, Homebrew or a single binary | npm (Node.js 20+) | per agent |
| Agents | Claude Code, Codex, Antigravity; Cursor next | 60+ tool targets | 30+ agents | the one you write it for |
| Rules, skills, subagents and MCP from one source | yes | yes | yes (skills and subagents experimental) | no |
| A shipped policy for git, formatters and read-only subagents, nested shells unwrapped | one policy for every agent | generates the hook and permission config you write | no | what you write |
| A lock, and a CI check for hand edits and stale output | `dashcrew check` | not in its README | a CI recipe comparing `git status` | no |
| Personal profile beside the team's | `dashcrew me` | user-level files (`--global`) | a global config as fallback | no |
| Commit message lint | yes | no | no | no |

rulesync and Ruler cover far more agents; pick one of them when you need an agent dashcrew does not support yet. Checked against their READMEs on 2026-11-18; corrections are welcome as an issue.

## Status

Pre-1.0: the profile format and CLI can still change before 1.0, and each change is in the [changelog](CHANGELOG.md). Claude Code output has been run end to end. Codex and Antigravity output is generated and checked against their documentation (and, for Codex, its source), but not yet against a live session. Windows is not supported yet: the guard shim is a `sh` script.

## Docs

| | |
|---|---|
| [docs/profile.md](docs/profile.md) | `.ai/project.yaml`, platforms, stack options, choosing skills |
| [docs/commands.md](docs/commands.md) | every command and flag |
| [docs/guard.md](docs/guard.md) | what the guard blocks, how it reads hidden commands, where it runs |
| [docs/team.md](docs/team.md) | setting up a team, the merge gate on GitHub, what each agent enforces |
| [docs/knowledge.md](docs/knowledge.md) | a shared knowledge repo and feature specs |
| [docs/release-lines.md](docs/release-lines.md) | two releases at once, patch lines and ports |
| [docs/troubleshooting.md](docs/troubleshooting.md) | messages and their fixes, migrating an old toolkit |
| [docs/catalog-format.md](docs/catalog-format.md) | writing your own packs and platforms |
| [docs/architecture.md](docs/architecture.md) | the code, for contributors |

## Contributing, security, license

Build, test, goldens and the commit pattern are in [CONTRIBUTING.md](CONTRIBUTING.md). Report a vulnerability privately; see [SECURITY.md](SECURITY.md). BSD-3-Clause; see [LICENSE](LICENSE).
