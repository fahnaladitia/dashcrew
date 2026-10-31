# dashcrew

Agent-agnostic AI coding toolkit for Flutter projects.

dashcrew keeps one neutral source of rules, roles, workflows and guards, and
renders it for the AI coding agents your team uses (Claude Code and Codex). Your stack is chosen once, in `.ai/project.yaml`.

> Status: early development (P1). Not usable yet.

Not affiliated with or endorsed by Google or the Flutter team.

## Planned commands

| Command | Does |
|---|---|
| `dashcrew init` | detect the stack from `pubspec.yaml`, ask what it cannot detect, write `.ai/project.yaml` |
| `dashcrew sync` | render rules, roles, skills, hooks and MCP config for the selected agents |
| `dashcrew doctor` | check generated files, hooks and skills against the lock file |
| `dashcrew worktree` | one worktree per parallel task: `create`, `list`, `link`, `remove`, `watch` |

## License

BSD-3-Clause. See [LICENSE](LICENSE).
