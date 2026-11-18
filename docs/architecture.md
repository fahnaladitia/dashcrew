# Architecture

A map of the code for a contributor in their first hour. Rules for changing it are in [CONTRIBUTING.md](../CONTRIBUTING.md) and [AGENTS.md](../AGENTS.md).

## Data flow

```
core/ + packs/ + platforms/ (shipped)  --+
.ai/packs/, .ai/platforms/ (yours)     --+-->  dashcrew sync  -->  AGENTS.md         read by every agent
.ai/project.yaml                       --+                         per-agent files   skills, subagents, hooks, MCP
                                                                   .ai/guard.json    one policy for every agent
                                                                   .ai/bin/guard     committed shim -> dashcrew guard
                                                                   .ai/lock.json     hashes, versions, owned entries
```

- **Neutral sources** are Markdown or YAML with front matter, with no agent-specific fields: `core/` (rules, skills, workflows, the two subagents, the PR template), `packs/<family>/<option>/` shared by every platform, and `platforms/<name>/` (today `flutter`). `catalog_embed.go` builds them into the binary, with `starters/` (`dashcrew platform new`) and `personal/` (`dashcrew me`). Their format is in [catalog-format.md](catalog-format.md).
- **The profile** `.ai/project.yaml` picks a platform, the stack options and the agents.
- **`sync`** loads the catalog, renders it for the profile, hands the result to one adapter per agent, writes the guard policy and shim, and records every file it owns in the lock.
- **The guard** runs on every tool call, through each agent's pre-tool hook and the committed shim, and decides from the policy alone.
- **`doctor` and `check`** read the project and the lock and write nothing.

## Packages

`cmd/dashcrew` parses flags and prints; every decision lives in `internal/`.

| Stage | Packages |
|---|---|
| Profile and catalog | `profile` (strict parse of `.ai/project.yaml`), `catalog` (loads core, packs, platforms and the project's own), `render` (joins fragments per target, filtered by `when`), `mustache`, `yamlutil`, `dartjson`, `omap` (byte-exact formats) |
| Writing | `sync` (the run and the lock), `adapters` (per-agent files), `agents` (the list of agents), `projectfiles` (writes inside the project, never through a link), `filemode`, `pathstyle`, `version` |
| Guard | `guard`: `event.go` reads each agent's hook JSON into one event, `unwrap.go` finds the commands inside nested shells and wrappers, `decide.go`, `push.go`, `backend.go` and `knowledge.go` apply the policy, `HookResponse` in `decide.go` answers in the agent's format, `run.go` ties it together, `malformed.go` logs what it could not read, `shim.go` and `policy.go` are what `sync` writes |
| Checks | `doctor` (one file per agent: `agent_claude.go`, `agent_codex.go`, `agent_antigravity.go`), `check` (the PR gate), `commit` (suggest and lint), `deps` and `ports` (parts of `check`) |
| Project setup | `initproject` (`init`), `starter` (`platform new`), `projectskill` (`skill adopt`), `me` (the personal profile) |
| Team features | `knowledge`, `knowledgemcp`, `spec`, `lines`, `ledger`, `link`, `worktree`, `usage` |
| Tests only | `conformance` (the goldens of `test/golden/`) |

## Adding an agent

An agent is named once, in the table of `internal/agents/agents.go`. Completeness tests then point at every place that needs an entry for it:

1. `internal/adapters/registry.go`: its adapter (the files it writes, `adapters/<agent>.go`), its skill folder, the guard version its hook needs, and its skill text.
2. `internal/guard/event.go`: how its hook JSON names tools, if it differs from the others, and `HookResponse` in `decide.go`: how it reads a block.
3. `internal/doctor/agents.go`: its hook check, its machine checks and what it loads at session start.
4. `internal/me/sync.go`, for an agent marked personal: where its personal instructions go.

Then add a sample to `test/golden/` and a transcript case.

## Goldens

| Where | What it pins |
|---|---|
| `test/golden/<sample>/expected/` | every file `sync` writes for that sample profile, byte for byte |
| `test/golden/sample-migrate-p3/p3/` | a frozen pre-0.1.0 toolkit the migration test syncs over; never rewritten |
| `cmd/dashcrew/testdata/transcripts/` | stdout, stderr, exit code and file changes of CLI calls |
| `internal/guard/testdata/corpus_outcomes.txt` | the guard's answer to every corpus case it blocks |

`DASHCREW_UPDATE_GOLDENS=1 go test ./internal/conformance ./cmd/dashcrew ./internal/guard` rewrites them; read the diff before a review.
