# P13: dashcrew usage, a token baseline from agent transcripts

Tier: standard. New package `internal/usage`, a new command in `cmd/dashcrew`, the `usage.txt` transcript changes (one new line in the command list) and a new transcript. No generated toolkit output changes, no new dependency.

First of four token-saving tasks, each with its own plan: P13 usage (this), P13b budget gate in doctor, P13c cheaper execution (reviewer, explorer, test output), P13d path-scoped Claude rules. P13 comes first so the other three have a before and after number.

## Goal

`dashcrew usage` reads the Claude Code and Codex transcripts of the current project and reports, per session: tokens (input, cache write, cache read, output) per role (main, reviewer, explorer, other subagents), the context loaded before the first answer, and the skills loaded. Reads only; writes nothing.

## What the transcripts hold (checked on this machine, expense_tracker, Claude Code)

**Claude Code** (`$CLAUDE_CONFIG_DIR` or `~/.claude`):

- `projects/<encoded cwd>/<session>.jsonl` is the main session. The encoded name is the absolute path with every character outside `[A-Za-z0-9]` replaced by `-` (`/…/expense_tracker` -> `-…-expense-tracker`). The name is lossy, so a candidate folder is kept only when its lines' `cwd` is the project dir or inside it.
- `projects/<encoded cwd>/<session>/subagents/agent-<id>.jsonl` plus `agent-<id>.meta.json` (`agentType`, `description`, `toolUseId`) is one subagent run. Older versions wrote subagent lines into the main file with `isSidechain: true`; those are grouped by `agentId`.
- Token usage is `message.usage` on `type: assistant` lines: `input_tokens`, `cache_creation_input_tokens`, `cache_read_input_tokens`, `output_tokens`. One API response is split over several lines that share `message.id` and repeat the same usage (296 of 416 ids repeat in one session), so usage is counted once per `message.id`.
- The other ~20 line types (`attachment`, `api-request`, `mode`, …) are ignored. A line that does not parse is skipped and counted (`skipped N lines`), never fatal.

**Codex** (`$CODEX_HOME` or `~/.codex`):

- `sessions/YYYY/MM/DD/rollout-*.jsonl`. The first `session_meta` line carries `payload.cwd` and `payload.id`; a rollout is kept when its cwd is the project dir or inside it.
- Usage is the last `event_msg` with `payload.type: token_count`, `payload.info.total_token_usage` (cumulative). OpenAI counts cached tokens inside `input_tokens`, so input = `input_tokens - cached_input_tokens` and cache read = `cached_input_tokens`; Codex has no cache write.
- No Codex rollout exists on this machine. The fixtures follow the documented format; the open question below asks for a real one before the parser is final.

## Roles

A subagent's role is, in order:

1. `agentType` (Claude) or the agent name (Codex) when it is a dashcrew role (`explorer`, `reviewer`, or a project role from `.ai/`).
2. Otherwise the first user prompt naming a role file: `.claude/agents/<role>.md`, `.agents/agents/<role>.md` or `.codex/agents/<role>.toml`. expense_tracker spawns its reviewer as `general-purpose` with "You are the read-only `reviewer` in …/.agents/agents/reviewer.md", so step 1 alone would miss it.
3. Otherwise `other:<agentType>`.

The main transcript is role `main`.

## Skills loaded

Per role, each skill once with a count:

- a `Skill` tool call (`input.skill`);
- a user `<command-name>/<name>` whose name is a skill in the project (`.claude/skills/`, `.agents/skills/`) or the member's own skills; built-in commands (`/clear`, `/compact`, …) are left out;
- a read of `…/skills/<name>/SKILL.md`: the `Read` tool for Claude Code, a shell read (`cat`, `sed`, `head`, `tail`, `nl`, `less`, `bat`) for Codex. A write, edit, grep or `ls` of a `SKILL.md` is not a load: on expense_tracker counting every mention gave 65 false loads (57 Bash, 8 Write).

## Start context

Per session, `start` is input + cache write + cache read of the first assistant response in `main`: what the agent had to load before doing anything (system prompt, AGENTS.md or CLAUDE.md, rules, memory index, skill list). This is the number P13b and P13d should lower.

## Command

```
dashcrew usage [--since <yyyy-mm-dd | Nd>] [--session <id prefix>] [--agent claude|codex] [--json]
```

- Default: sessions of the last 7 days, newest first, then a total.
- Project dir: the git top level of the working directory, plus every path from `git worktree list` (dashcrew's own worktree command puts parallel tasks there).
- No session found: exit 0 with `no Claude Code or Codex sessions for <dir> since <date>`.
- `--json`: one object per session, numbers unformatted, for comparing a before and an after run.
- No USD cost: prices change and are not in the transcripts.

Text output, layout only (the numbers are illustrative; below 1000 as is, else k or M with one decimal; `skipped N lines` only when N > 0):

```
claude  fccf7522  2026-11-10 09:14  claude-opus-5-5  start 76.1k
  role      runs  responses  input  cache-write  cache-read  output
  main         1        416   1.2k       410.3k       18.4M   96.0k
  reviewer     3          9     40        88.2k      402.0k    6.1k
  skills    main: checkpoint, tdd x2
codex  0a1b2c3d  2026-11-11 14:02  gpt-5-codex  start 31.0k  skipped 2 lines
  …
total  2 sessions  input 1.4k  cache-write 498.5k  cache-read 18.8M  output 102.1k
```

## Code

- `internal/usage`: `Find(projectDirs, homes, since) []Source`, `ParseClaude(Source) Session`, `ParseCodex(Source) Session`, `Report(sessions, w)`, `JSON(sessions, w)`. Plain structs, standard library only (`encoding/json`, `bufio` with a raised line limit: one line can hold a whole request).
- `cmd/dashcrew/usage.go`: flags, project dirs from git, homes from the env map (so tests set `HOME`, `CLAUDE_CONFIG_DIR`, `CODEX_HOME`).

## Tests (test first)

1. `internal/usage`, with small hand-trimmed fixtures in `internal/usage/testdata/` (no real prompts or paths):
   - repeated `message.id` counted once;
   - a subagent from `subagents/` gets its role from `agentType`, from the prompt naming `.agents/agents/reviewer.md`, or `other:general-purpose`;
   - old-style `isSidechain` lines grouped by `agentId`;
   - an encoded folder whose `cwd` is another project is left out;
   - `start` is the first main response;
   - skills from a `Skill` call, a slash command and a `SKILL.md` read; `/clear` left out;
   - Codex: last cumulative `token_count`, cached tokens moved out of input, cwd filter;
   - a broken line is skipped and counted.
2. `cmd/dashcrew`: transcript `usage-report.txt` with a fixture `HOME` (text, `--json`, `--session`, no sessions); `usage.txt` gains the `usage` line.
3. `go vet ./...`, `GOOS=windows go vet ./...`, `go test ./...`.

## Decisions (maintainer, 2026-11-16)

1. Codex: implement from the documented format now; check the fixture against a real rollout once one exists.
2. Worktrees are included by default.
3. Default window: the last 7 days.

## Result

- First run on expense_tracker (`--since 60d`, 0.7 s): 3 Claude Code sessions, `start` 71.7k to 82.9k tokens before the first answer; the reviewer ran 2 to 3 times per session and read 6.4M to 9.0M cached tokens. This is the baseline for P13b and P13d.
- Found while verifying, not fixed here: a `.DS_Store` in `core/` or `packs/` changes the catalog hash, so `sample-release-lines` (conformance) and the three release-line transcripts fail on a Mac checkout that has one. Fails the same on `main` without this change.
