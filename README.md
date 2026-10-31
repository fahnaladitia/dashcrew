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

## Install

Requires Dart 3.9 or later (Flutter 3.35 or later).

```bash
dart pub global activate --source git https://github.com/fahnaladitia/dashcrew
```

Pin a commit with `--git-ref <sha>` so everyone on a team runs the same dashcrew.

## Commands

| Command | Does |
|---|---|
| `dashcrew init` | detect the stack from `pubspec.yaml`, ask what it cannot detect, write `.ai/project.yaml` |
| `dashcrew sync` | render rules, roles, skills, hooks and MCP config for the selected agents, and compile the guard |
| `dashcrew doctor` | check generated files, hooks, skills and the dashcrew version against the lock file |
| `dashcrew worktree` | one worktree per parallel task: `create`, `list`, `link`, `remove`, `watch` |

Quick start, in the Flutter project root:

```bash
dashcrew init
dashcrew sync
dashcrew doctor
```

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

## License

BSD-3-Clause. See [LICENSE](LICENSE).
