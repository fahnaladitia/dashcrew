# Using dashcrew in a team

Teammates may use different agents in the same repository. They share one
AGENTS.md, one set of skills and subagents, and one guard policy; each agent gets
them in the format it reads.

## Set up once

1. List every agent the team uses: `agents: [claude-code, codex, antigravity]`.
2. Run `dashcrew sync` and commit everything it wrote:
   `AGENTS.md`, `CLAUDE.md`, `.claude/`, `.codex/`, `.agents/`, `.mcp.json`, and `.ai/` (`project.yaml`, `lock.json`, `guard.json`, `bin/guard`, `packs/`, `platforms/`).
   Teammates then get the toolkit on pull, and `dashcrew doctor` compares their checkout with the committed lock.

## Each teammate

- Install the same dashcrew as everyone else (see [Install](../README.md#quickstart)). A team pins one: `DASHCREW_VERSION=<version>` with `install.sh`, or `go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@v<version>`; `brew` installs the latest release. `dashcrew --version` prints it.
- Nothing else to run after cloning: every agent tool call goes through the committed shim to `dashcrew guard`.
  Without dashcrew 0.1.0 or newer on `PATH` the shim refuses every tool call and names the install command.
- Run `dashcrew doctor` to confirm.
- Optional: `dashcrew me init` and `dashcrew me sync` for your personal profile (see [`dashcrew me`](commands.md#dashcrew-me)).
- Codex users: trust the project when Codex asks, then approve the dashcrew hook in `/hooks`. Codex ignores a project's `.codex/` until both are done.
- Antigravity users: trust the folder when agy asks; agy loads `.agents/hooks.json` only in a trusted folder. `dashcrew doctor` asks `agy -p /hooks` whether the guard is active when `agy` is on `PATH`.

## Day to day

- Change rules through `.ai/project.yaml`, `.ai/packs/`, `.ai/platforms/` or the project block in AGENTS.md (between `dashcrew:project:begin` and `dashcrew:project:end`), never by editing generated files. `doctor` reports generated files edited by hand.
- After a merge that touched `.ai/project.yaml`, `.ai/packs/` or `.ai/platforms/`: resolve conflicts in those files only, run `dashcrew sync`, and commit the regenerated files.
- `sync` refuses a lock written by a newer dashcrew; update dashcrew instead of forcing it.
  Release builds report their tag, and `go install` of a commit reports a version just above the previous tag. A local build without a version reports 0.0.1.

## What each agent enforces

| | Claude Code | Codex | Antigravity |
|---|---|---|---|
| Rules (AGENTS.md), skills, subagents | yes | yes | yes |
| Guard: no push, commit, history rewrite, formatter or vendor import | yes | yes, once trusted | yes, once the folder is trusted |
| `reviewer` and `explorer` cannot write | guard | guard | the subagent's `tools` allowlist |
| AGENTS.md cut at | no documented cut | 32 KiB | 24,000 bytes |

dashcrew keeps AGENTS.md under the smallest cut of the selected agents, and `doctor` warns above it.

## Subagent model and effort

The model and effort of `reviewer` and `explorer` are written into their committed files, so they are the same for every member. The team changes them in `.ai/project.yaml` (`roles`, see [the profile](profile.md#the-profile-aiprojectyaml)):

```yaml
roles:
  reviewer: { model: standard, effort: medium }
  explorer: { model: inherit, effort: inherit }
```

A value the team writes wins over every member's own setting: Claude Code and Codex apply a subagent file's `model` and effort before any user default, and a same-named subagent in a member's home folder loses to the project's. To let each member choose, write `inherit`; the line is left out of the file, and each member sets their own default:

| | Model | Effort |
|---|---|---|
| Claude Code | `CLAUDE_CODE_SUBAGENT_MODEL` in the `env` block of `~/.claude/settings.json` (Claude Code 2.1.251 or newer reads it as a default) | `CLAUDE_CODE_SUBAGENT_EFFORT` in the same block (2.1.316 or newer) |
| Codex | not written by dashcrew in any case: `[agents] default_subagent_model` in `~/.codex/config.toml` | `[agents] default_subagent_reasoning_effort` in `~/.codex/config.toml` |
| Antigravity | the session's model (`/model`, `--model`) | the session's effort (`/effort`, `--effort`) |

A member who sets nothing runs the subagent on the session's model and effort. Claude Code also has switches that beat a value the team wrote, `CLAUDE_CODE_SUBAGENT_MODEL_FORCE=1` and `CLAUDE_CODE_EFFORT_LEVEL`, but they apply to every subagent, and the effort one to the main session too; dashcrew does not use them. While a subagent runs, Claude Code's `/tasks` shows the model it runs on.

## Merge gate on GitHub

The plan is confirmed in the session by the task owner; the team reviews in the pull request, where this gate holds. GitHub is the only infrastructure.

1. Set `ci: { github: true, dashcrew_ref: v0.1.0 }` (a release tag, `v0.1.0` or newer) and run `dashcrew sync`. Commit `.github/workflows/dashcrew.yml`. The workflow sets up Go and runs `go install github.com/fahnaladitia/dashcrew/cmd/dashcrew@<dashcrew_ref>`, then `dashcrew check`.
2. In the repository settings, add a branch protection rule (or ruleset) for your base branches that requires the status check `check` from the `dashcrew check` workflow.
3. Every pull request then needs a current toolkit, every heading of the PR template, and commits that pass `dashcrew commit lint`. Failures show as annotations on the PR. The workflow checks out the full history (`fetch-depth: 0`) so the PR's commits can be read.

The workflow reads the PR body from the pull request event and needs no token beyond read access to the repository.
