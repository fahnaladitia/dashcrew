# Troubleshooting

| Message | Fix |
|---|---|
| `AGENTS.md exists and was not written by dashcrew` | `dashcrew sync --adopt` keeps your file as the project block |
| `Files changed outside dashcrew` | move the change to its source (profile, project pack, project block), then `dashcrew sync --force` |
| `.ai/lock.json was written by dashcrew X` | install that dashcrew (or the team's pinned ref) |
| `[dashcrew guard] dashcrew is not on PATH, so this tool call is refused` | install dashcrew (see [Install](../README.md#quickstart)); the agent's next tool call goes through |
| agy runs `.agents/.ai/bin/guard`, which does not exist | an older hook entry: agy runs hooks in the folder of `hooks.json`; run `dashcrew sync`, which writes a command that finds the shim from the hooks folder, the folder above it or the git toplevel (`antigravity-hook` fails until then); the project then needs dashcrew 0.5.0 or newer |
| `antigravity-hook`: `agy does not load it here` | trust the folder in agy; workspace hooks load only in a trusted folder |
| `[dashcrew guard] the dashcrew on PATH is older than X` | install dashcrew X or newer; `doctor` names the `dashcrew` it found |
| `guard-binary`: `is the Dart one` | an old Dart-installed dashcrew (from before 0.1.0) comes first on `PATH`: run `dart pub global deactivate dashcrew`, then install dashcrew (see [Install](../README.md#quickstart)) |
| `guard-log`: `the guard let N tool calls through unchecked` | read the reasons in the log: update dashcrew if an agent changed its hook format or renamed a tool (the log names the agent and the tool); run `dashcrew sync` for a policy it could not use |
| `dashcrew guard install was removed` | an old shim called it: run `dashcrew sync` to write the new `.ai/bin/guard` |
| `plan_gate was removed in P4` | delete `policy.plan_gate` from `.ai/project.yaml`; see [Migrating a toolkit synced before 0.1.0](#migrating-a-toolkit-synced-before-010) |
| `check`: `pr-sections` `missing: ## ...` | add the heading to the PR description; write "None" under it when it does not apply |
| `gitignore`: `.ai/bin/ is in .gitignore` | remove that line so the shim is committed |
| `codex-trust`: `project not trusted` | trust the project when Codex asks, then approve the hook in `/hooks` |
| `dashcrew --version` prints `0.0.1`, and the shim refuses every tool call | a local build without a version stamp; install a release, or `go install` a tag or commit, which reports a real version (see [Install](../README.md#quickstart)) |
| `skill <name> skipped` | a project pack replaced the pack that defined it; add `skills/<name>.yaml` to your pack |

## Migrating a toolkit synced before 0.1.0

Before 0.1.0, dashcrew moved from role subagents to skills (the CLI messages name that change `P4`). A project synced by an older dashcrew migrates on its next `dashcrew sync`; there is no legacy option.

- **Roles removed.** Every old role subagent is deleted for every agent: the eleven each project had (planner, engineer, tech-lead, `worktree-monitor` and the others), plus `design-refiner` and `local-storage-engineer`, which the design and storage packs added. `reviewer` and `explorer` remain. What the roles checked now lives in skills (`acceptance-criteria`, `feature-architecture`, `figma-design`, `hive-storage`, `qa-identifiers`) and in the workflows.
- **`orchestrate` and `checkpoint` removed.** `orchestrate team` is now the default chain: step 0 with a `Skills:` line, then the tier's steps. `verify` checks a failing test once against the base instead of a checkpoint.
- **Plan gate removed.** No `dashcrew gate` command, no `plan-approval` check, no plan-gate guard rule; `.ai/guard.json` loses `plan_gate`. Delete `policy.plan_gate` from `.ai/project.yaml`: it is ignored with a warning for one release, then rejected as an unknown key. The merge gate (`dashcrew check`) stays.
- **What `sync` does.** It deletes every generated file it no longer writes, prints `note: Migrated to P4: ...` once, and rewrites AGENTS.md around your project block. Files the team wrote are never touched: plans and handoffs under `paths.plans`, project memory under `paths.memory`, and the AGENTS.md project block.
- **After it.** Commit the result. `doctor` lists leftover `<plans>/*/handoff/00-roster.md` files as `info`; nothing reads them, so delete them when the plans are done. Text in your project block or project packs that names a removed role or `orchestrate` is yours to edit.

## Moving a Go project to the built-in go platform

Before 0.8.0, Go was a starter: `init` and `dashcrew platform new go` wrote it into `.ai/platforms/go/`, and a project platform with a built-in name replaces the built-in one. Such a project keeps its folder and its output after upgrading; nothing changes until you choose.

- **Never edited the folder:** delete `.ai/platforms/go/`, run `dashcrew sync` and commit the result. The project is then on the built-in `go` platform and gets its rule and guard updates with each dashcrew release.
- **Edited it:** keep it. It stays yours; compare it with a fresh copy (`dashcrew platform new go -p <empty folder>`) to take what you want from the built-in rules.
